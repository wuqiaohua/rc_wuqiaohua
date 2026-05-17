package rabbitmq

const (
	DeliveryExchange = "notification.delivery"
	DeliveryQueue    = "notification.delivery"
	DeliveryKey      = "deliver"

	RetryExchange = "notification.retry"
	RetryQueue    = "notification.retry"
	RetryKey      = "retry"

	DeadExchange = "notification.dead"
	DeadQueue    = "notification.dead"
	DeadKey      = "dead"
)
