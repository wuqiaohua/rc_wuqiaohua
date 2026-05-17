package worker

import (
	"context"
	"log"
	"time"

	"github.com/wujunqi/rc_wujunqi/internal/model"
)

type OutboxStore interface {
	FetchPendingOutbox(ctx context.Context, limit int) ([]model.OutboxMessage, error)
	MarkOutboxPublished(ctx context.Context, outboxID, taskID, messageID string) error
	MarkOutboxFailed(ctx context.Context, outboxID string, publishErr error) error
}

type DeliveryPublisher interface {
	PublishDelivery(ctx context.Context, body []byte) (string, error)
}

type OutboxPublisher struct {
	store    OutboxStore
	mq       DeliveryPublisher
	interval time.Duration
	limit    int
	logger   *log.Logger
}

func NewOutboxPublisher(store OutboxStore, mq DeliveryPublisher, interval time.Duration, limit int, logger *log.Logger) *OutboxPublisher {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if limit <= 0 {
		limit = 20
	}
	if logger == nil {
		logger = log.Default()
	}
	return &OutboxPublisher{store: store, mq: mq, interval: interval, limit: limit, logger: logger}
}

func (p *OutboxPublisher) Run(ctx context.Context) {
	p.publishOnce(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.publishOnce(ctx)
		}
	}
}

func (p *OutboxPublisher) publishOnce(ctx context.Context) {
	messages, err := p.store.FetchPendingOutbox(ctx, p.limit)
	if err != nil {
		p.logger.Printf("fetch outbox failed: %v", err)
		return
	}
	for _, msg := range messages {
		messageID, err := p.mq.PublishDelivery(ctx, msg.PayloadJSON)
		if err != nil {
			p.logger.Printf("publish outbox %s failed: %v", msg.ID, err)
			if markErr := p.store.MarkOutboxFailed(ctx, msg.ID, err); markErr != nil {
				p.logger.Printf("mark outbox %s failed: %v", msg.ID, markErr)
			}
			continue
		}
		if err := p.store.MarkOutboxPublished(ctx, msg.ID, msg.TaskID, messageID); err != nil {
			p.logger.Printf("mark outbox %s published failed: %v", msg.ID, err)
		}
	}
}
