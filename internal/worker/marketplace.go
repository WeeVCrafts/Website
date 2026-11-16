package worker

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wecratfs/commerce/internal/ports"
)

// PostgreSQL retains jobs until completion. Streams are a transport, so queue
// loss or a process crash cannot discard a committed marketplace operation.
func RunMarketplaceQueue(ctx context.Context, client *redis.Client, store ports.MarketplaceStore, process func(context.Context, int64) error, consumer string) {
	const stream = "weevcrafts:marketplace:jobs"
	const group = "marketplace-workers"
	dispatch := func() {
		qctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		_ = store.QueueMaintenance(qctx)
		jobs, err := store.DueJobs(qctx, 25)
		if err != nil {
			return
		}
		for _, j := range jobs {
			if client.XAdd(qctx, &redis.XAddArgs{Stream: stream, MaxLen: 10000, Approx: true, Values: map[string]any{"job_id": j.ID}}).Err() != nil {
				return
			}
			if store.JobDispatched(qctx, j.ID) != nil {
				return
			}
		}
	}
	dispatch()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		// Creating an existing group returns BUSYGROUP; all other failures are
		// handled by the next iteration. This also recovers a lost Redis instance.
		_ = client.XGroupCreateMkStream(ctx, stream, group, "0").Err()
		messages, _, err := client.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: stream, Group: group, Consumer: consumer, MinIdle: 3 * time.Minute, Start: "0-0", Count: 5}).Result()
		if err == nil {
			for _, m := range messages {
				consumeMarketplaceMessage(ctx, client, stream, group, m, process)
			}
		}
		batches, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: group, Consumer: consumer, Streams: []string{stream, ">"}, Count: 5, Block: time.Second}).Result()
		if err == nil {
			for _, batch := range batches {
				for _, m := range batch.Messages {
					consumeMarketplaceMessage(ctx, client, stream, group, m, process)
				}
			}
		} else if !errors.Is(err, redis.Nil) && ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dispatch()
		default:
		}
	}
}
func consumeMarketplaceMessage(ctx context.Context, client *redis.Client, stream, group string, m redis.XMessage, process func(context.Context, int64) error) {
	id, err := strconv.ParseInt(stringValue(m.Values["job_id"]), 10, 64)
	if err == nil && id > 0 {
		_ = process(ctx, id)
	}
	// A failed job remains retryable in PostgreSQL, including when saving its
	// result failed; its claim expires before the dispatcher republishes it.
	if ctx.Err() == nil {
		_ = client.XAck(ctx, stream, group, m.ID).Err()
	}
}
func stringValue(v any) string { s, _ := v.(string); return s }
