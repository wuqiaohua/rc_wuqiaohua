package model

import (
	"database/sql"
	"time"
)

const (
	OutboxStatusPending   = "pending"
	OutboxStatusPublished = "published"
	OutboxStatusFailed    = "failed"

	EventNotificationCreated = "notification.created"
)

type OutboxMessage struct {
	ID              string
	TaskID          string
	EventType       string
	PayloadJSON     []byte
	Status          string
	PublishAttempts int
	LastError       sql.NullString
	CreatedAt       time.Time
	PublishedAt     sql.NullTime
}

type DeliveryMessage struct {
	TaskID string `json:"task_id"`
}

type DeadLetterMessage struct {
	TaskID         string `json:"task_id"`
	RetryCount     int    `json:"retry_count"`
	LastError      string `json:"last_error"`
	LastStatusCode int    `json:"last_status_code,omitempty"`
}
