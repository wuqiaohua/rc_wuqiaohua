package rabbitmq

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn *amqp.Connection
}

func Dial(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) SetupTopology() error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(DeliveryExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(RetryExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(DeadExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(DeliveryQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(DeliveryQueue, DeliveryKey, DeliveryExchange, false, nil); err != nil {
		return err
	}
	retryArgs := amqp.Table{
		"x-dead-letter-exchange":    DeliveryExchange,
		"x-dead-letter-routing-key": DeliveryKey,
	}
	if _, err := ch.QueueDeclare(RetryQueue, true, false, false, false, retryArgs); err != nil {
		return err
	}
	if err := ch.QueueBind(RetryQueue, RetryKey, RetryExchange, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(DeadQueue, true, false, false, false, nil); err != nil {
		return err
	}
	return ch.QueueBind(DeadQueue, DeadKey, DeadExchange, false, nil)
}

func (c *Client) PublishDelivery(ctx context.Context, body []byte) (string, error) {
	return c.publish(ctx, DeliveryExchange, DeliveryKey, body, 0)
}

func (c *Client) PublishRetry(ctx context.Context, body []byte, delay time.Duration) (string, error) {
	return c.publish(ctx, RetryExchange, RetryKey, body, delay)
}

func (c *Client) PublishDead(ctx context.Context, body []byte) (string, error) {
	return c.publish(ctx, DeadExchange, DeadKey, body, 0)
}

func (c *Client) ConsumeDelivery(consumerTag string, prefetch int) (<-chan amqp.Delivery, *amqp.Channel, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, nil, err
	}
	if prefetch <= 0 {
		prefetch = 10
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		_ = ch.Close()
		return nil, nil, err
	}
	deliveries, err := ch.Consume(DeliveryQueue, consumerTag, false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return nil, nil, err
	}
	return deliveries, ch, nil
}

func (c *Client) publish(ctx context.Context, exchange, routingKey string, body []byte, delay time.Duration) (string, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return "", err
	}
	defer ch.Close()

	if err := ch.Confirm(false); err != nil {
		return "", err
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	messageID := uuid.NewString()
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    messageID,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	}
	if delay > 0 {
		publishing.Expiration = stringInt64(delay.Milliseconds())
	}
	if err := ch.PublishWithContext(ctx, exchange, routingKey, false, false, publishing); err != nil {
		return "", err
	}
	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return "", errors.New("rabbitmq publish was not acknowledged")
		}
		return messageID, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(5 * time.Second):
		return "", context.DeadlineExceeded
	}
}

func stringInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}
