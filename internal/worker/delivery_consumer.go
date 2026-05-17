package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/wujunqi/rc_wujunqi/internal/domain"
	"github.com/wujunqi/rc_wujunqi/internal/infra/httpclient"
	"github.com/wujunqi/rc_wujunqi/internal/model"
)

type TaskStore interface {
	GetTask(ctx context.Context, id string) (*model.NotificationTask, error)
	MarkProcessing(ctx context.Context, taskID string) error
	MarkSucceeded(ctx context.Context, taskID string, statusCode int) error
	MarkRetrying(ctx context.Context, taskID string, retryCount int, nextRetryAt time.Time, statusCode int, lastErr string) error
	MarkFailed(ctx context.Context, taskID string, statusCode int, lastErr string) error
}

type RetryPublisher interface {
	PublishRetry(ctx context.Context, body []byte, delay time.Duration) (string, error)
	PublishDead(ctx context.Context, body []byte) (string, error)
}

type DeliveryNotifier interface {
	Send(ctx context.Context, task *model.NotificationTask) (httpclient.DeliveryResult, error)
}

type DeliveryConsumer struct {
	store       TaskStore
	mq          RetryPublisher
	notifier    DeliveryNotifier
	retryPolicy domain.RetryPolicy
	logger      *log.Logger
}

func NewDeliveryConsumer(store TaskStore, mq RetryPublisher, notifier DeliveryNotifier, retryPolicy domain.RetryPolicy, logger *log.Logger) *DeliveryConsumer {
	if logger == nil {
		logger = log.Default()
	}
	return &DeliveryConsumer{
		store:       store,
		mq:          mq,
		notifier:    notifier,
		retryPolicy: retryPolicy,
		logger:      logger,
	}
}

type deliverySource interface {
	ConsumeDelivery(consumerTag string, prefetch int) (<-chan amqp.Delivery, *amqp.Channel, error)
}

func (c *DeliveryConsumer) Run(ctx context.Context, source deliverySource, consumerTag string, prefetch int) {
	deliveries, ch, err := source.ConsumeDelivery(consumerTag, prefetch)
	if err != nil {
		c.logger.Printf("start consumer failed: %v", err)
		return
	}
	defer ch.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			c.handleDelivery(ctx, delivery)
		}
	}
}

func (c *DeliveryConsumer) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	if err := c.process(ctx, delivery.Body); err != nil {
		c.logger.Printf("process delivery failed: %v", err)
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}

func (c *DeliveryConsumer) process(ctx context.Context, body []byte) error {
	var msg model.DeliveryMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil
	}
	if msg.TaskID == "" {
		return nil
	}

	task, err := c.store.GetTask(ctx, msg.TaskID)
	if err != nil {
		return err
	}
	if model.IsTerminalStatus(task.Status) {
		return nil
	}
	if err := c.store.MarkProcessing(ctx, task.ID); err != nil {
		return err
	}

	result, sendErr := c.notifier.Send(ctx, task)
	if sendErr == nil && result.Success {
		return c.store.MarkSucceeded(ctx, task.ID, result.StatusCode)
	}

	decision := c.retryPolicy.Decide(result.StatusCode, sendErr, task.RetryCount)
	lastErr := deliveryErrorMessage(result.StatusCode, sendErr)
	if decision.Retryable {
		nextRetryCount := task.RetryCount + 1
		nextRetryAt := time.Now().UTC().Add(decision.Delay)
		if err := c.store.MarkRetrying(ctx, task.ID, nextRetryCount, nextRetryAt, result.StatusCode, lastErr); err != nil {
			return err
		}
		if _, err := c.mq.PublishRetry(ctx, body, decision.Delay); err != nil {
			return err
		}
		return nil
	}

	if err := c.store.MarkFailed(ctx, task.ID, result.StatusCode, lastErr); err != nil {
		return err
	}
	deadBody, _ := json.Marshal(model.DeadLetterMessage{
		TaskID:         task.ID,
		RetryCount:     task.RetryCount,
		LastError:      lastErr,
		LastStatusCode: result.StatusCode,
	})
	_, err = c.mq.PublishDead(ctx, deadBody)
	return err
}

func deliveryErrorMessage(statusCode int, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("upstream returned HTTP %d", statusCode)
}
