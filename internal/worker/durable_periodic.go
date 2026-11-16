package worker

import (
	"context"
	"fmt"
	"time"
)

type ScheduledJobStore interface {
	EnqueueScheduled(context.Context, string, string) error
}

// Register before starting consumers. The ticker only persists work; consumers
// execute it under the same durable claims as fulfillment and finance jobs.
func RegisterDurablePeriodic(ctx context.Context, interval time.Duration, name string, store ScheduledJobStore, handlers map[string]func(context.Context) error, job Job) {
	if interval <= 0 || job == nil {
		return
	}
	handlers[name] = job
	go RunPeriodic(ctx, interval, func(ctx context.Context) error {
		due, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return store.EnqueueScheduled(due, name, fmt.Sprintf("scheduled:%s:%d", name, time.Now().UnixNano()/int64(interval)))
	})
}
