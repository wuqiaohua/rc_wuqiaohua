package model

import (
	"database/sql"
	"encoding/json"
	"time"
)

type NotificationTask struct {
	ID             string
	AppID          string
	Vendor         string
	TargetURL      string
	Method         string
	HeadersJSON    []byte
	PayloadJSON    []byte
	IdempotencyKey sql.NullString
	Status         string
	RetryCount     int
	MaxRetries     int
	NextRetryAt    sql.NullTime
	LastStatusCode sql.NullInt64
	LastError      sql.NullString
	MQMessageID    sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (t NotificationTask) Headers() map[string]string {
	if len(t.HeadersJSON) == 0 {
		return map[string]string{}
	}
	var headers map[string]string
	if err := json.Unmarshal(t.HeadersJSON, &headers); err != nil || headers == nil {
		return map[string]string{}
	}
	return headers
}

func (t NotificationTask) IdempotencyKeyValue() string {
	if !t.IdempotencyKey.Valid {
		return ""
	}
	return t.IdempotencyKey.String
}
