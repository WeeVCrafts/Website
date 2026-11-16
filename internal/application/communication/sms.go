package communication

import (
	"context"
	"errors"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

type SMSProcessor struct {
	Store  ports.SMSStore
	Sender ports.SMSSender
}

func (p *SMSProcessor) Process(ctx context.Context, limit int) (int, error) {
	if p.Sender == nil {
		return 0, nil
	}
	if p.Store == nil {
		return 0, errors.New("SMS delivery store unavailable")
	}
	if limit < 1 || limit > 20 {
		limit = 5
	}
	completed := 0
	var firstErr error
	for i := 0; i < limit; i++ {
		if ctx.Err() != nil {
			return completed, ctx.Err()
		}
		job, claimed, err := p.Store.ClaimSMS(ctx)
		if err != nil {
			return completed, err
		}
		if !claimed {
			break
		}
		sendCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		result, sendErr := p.Sender.SendTransactionalSMS(sendCtx, job)
		cancel()
		if result.Outcome != "accepted" && result.Outcome != "rejected" && result.Outcome != "skipped" {
			result.Outcome = "uncertain"
		}
		finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err = p.Store.FinishSMS(finishCtx, job.ID, result)
		finishCancel()
		if err != nil {
			return completed, errors.New("SMS result persistence requires reconciliation")
		}
		if sendErr != nil && firstErr == nil {
			firstErr = errors.New("SMS delivery requires operator review")
		}
		completed++
	}
	return completed, firstErr
}
