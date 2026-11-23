package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

const ProductIndexEventType = "ProductIndexRequested"

var ErrIndexWorkerUnavailable = errors.New("product index worker is unavailable")

type ProductIndexProcessor struct {
	events      ports.OutboxStore
	source      ports.ApprovedIndexSource
	indexer     ports.SearchIndexer
	invalidator ports.CacheInvalidator
}

func NewProductIndexProcessor(events ports.OutboxStore, source ports.ApprovedIndexSource, indexer ports.SearchIndexer) *ProductIndexProcessor {
	return NewProductIndexProcessorWithCache(events, source, indexer, nil)
}

func NewProductIndexProcessorWithCache(events ports.OutboxStore, source ports.ApprovedIndexSource, indexer ports.SearchIndexer, invalidator ports.CacheInvalidator) *ProductIndexProcessor {
	return &ProductIndexProcessor{events: events, source: source, indexer: indexer, invalidator: invalidator}
}

func (p *ProductIndexProcessor) Process(ctx context.Context, limit int) (processed int, err error) {
	if p.events == nil || p.source == nil || p.indexer == nil {
		return 0, ErrIndexWorkerUnavailable
	}
	events, err := p.events.ClaimByType(ctx, ProductIndexEventType, limit)
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		if productID, parseErr := strconv.ParseInt(event.AggregateID, 10, 64); parseErr == nil && productID > 0 {
			doc, sourceErr := p.source.FindApprovedForIndex(ctx, productID)
			if sourceErr != nil {
				_ = p.events.Fail(ctx, event.ID, time.Minute, sourceErr.Error())
				if err == nil {
					err = sourceErr
				}
				continue
			}
			if doc != nil {
				if indexErr := p.indexer.IndexDocument(ctx, *doc); indexErr != nil {
					_ = p.events.Fail(ctx, event.ID, time.Minute, indexErr.Error())
					if err == nil {
						err = indexErr
					}
					continue
				}
			} else {
				var payload struct {
					Slug string `json:"slug"`
				}
				_ = json.Unmarshal(event.Payload, &payload)
				if payload.Slug != "" {
					_ = p.indexer.DeleteDocument(ctx, payload.Slug)
				}
			}
		} else {
			documents, sourceErr := p.source.ListApprovedForIndex(ctx)
			if sourceErr != nil {
				_ = p.events.Fail(ctx, event.ID, time.Minute, sourceErr.Error())
				if err == nil {
					err = sourceErr
				}
				continue
			}
			if indexErr := p.indexer.Rebuild(ctx, documents); indexErr != nil {
				_ = p.events.Fail(ctx, event.ID, time.Minute, indexErr.Error())
				if err == nil {
					err = indexErr
				}
				continue
			}
		}

		if p.invalidator != nil {
			if invalidateErr := p.invalidator.DeletePrefix(ctx, "catalog:search:"); invalidateErr != nil {
				_ = p.events.Fail(ctx, event.ID, time.Minute, invalidateErr.Error())
				if err == nil {
					err = invalidateErr
				}
				continue
			}
		}
		if ackErr := p.events.Ack(ctx, event.ID); ackErr != nil {
			if err == nil {
				err = fmt.Errorf("ack product index event %d: %w", event.ID, ackErr)
			}
			continue
		}
		processed++
	}
	return processed, err
}
