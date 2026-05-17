package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/wujunqi/rc_wujunqi/internal/model"
	"github.com/wujunqi/rc_wujunqi/internal/service"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) CreateTaskWithOutbox(ctx context.Context, input service.CreateNotificationInput) (*model.NotificationTask, bool, error) {
	headersJSON, err := json.Marshal(input.Headers)
	if err != nil {
		return nil, false, err
	}
	payloadJSON := []byte(input.Payload)
	taskID := "ntf_" + uuid.NewString()
	outboxID := "obx_" + uuid.NewString()
	now := time.Now().UTC()

	var idempotencyValue any
	if input.IdempotencyKey != "" {
		idempotencyValue = input.IdempotencyKey
	}

	outboxPayload, err := json.Marshal(model.DeliveryMessage{TaskID: taskID})
	if err != nil {
		return nil, false, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO notification_tasks
  (id, app_id, vendor, target_url, method, headers, payload, idempotency_key, status, retry_count, max_retries, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?)`,
		taskID, input.AppID, input.Vendor, input.TargetURL, input.Method, headersJSON, payloadJSON, idempotencyValue,
		model.StatusPending, input.MaxRetries, now, now,
	)
	if err != nil {
		_ = tx.Rollback()
		if isDuplicateKey(err) && input.IdempotencyKey != "" {
			existing, getErr := r.GetTaskByIdempotencyKey(ctx, input.AppID, input.Vendor, input.IdempotencyKey)
			return existing, false, getErr
		}
		return nil, false, err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO outbox_messages
  (id, task_id, event_type, payload, status, publish_attempts, created_at)
VALUES
  (?, ?, ?, ?, ?, 0, ?)`,
		outboxID, taskID, model.EventNotificationCreated, outboxPayload, model.OutboxStatusPending, now,
	)
	if err != nil {
		_ = tx.Rollback()
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	task, err := r.GetTask(ctx, taskID)
	return task, true, err
}

func (r *TaskRepository) GetTask(ctx context.Context, id string) (*model.NotificationTask, error) {
	row := r.db.QueryRowContext(ctx, selectTaskSQL()+` WHERE id = ?`, id)
	return scanTask(row)
}

func (r *TaskRepository) GetTaskForApp(ctx context.Context, appID, id string) (*model.NotificationTask, error) {
	row := r.db.QueryRowContext(ctx, selectTaskSQL()+` WHERE app_id = ? AND id = ?`, appID, id)
	return scanTask(row)
}

func (r *TaskRepository) GetTaskByIdempotencyKey(ctx context.Context, appID, vendor, idempotencyKey string) (*model.NotificationTask, error) {
	row := r.db.QueryRowContext(ctx, selectTaskSQL()+` WHERE app_id = ? AND vendor = ? AND idempotency_key = ?`, appID, vendor, idempotencyKey)
	return scanTask(row)
}

func (r *TaskRepository) FetchPendingOutbox(ctx context.Context, limit int) ([]model.OutboxMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, task_id, event_type, payload, status, publish_attempts, last_error, created_at, published_at
FROM outbox_messages
WHERE status IN (?, ?)
ORDER BY created_at ASC
LIMIT ?`, model.OutboxStatusPending, model.OutboxStatusFailed, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []model.OutboxMessage
	for rows.Next() {
		var msg model.OutboxMessage
		if err := rows.Scan(
			&msg.ID,
			&msg.TaskID,
			&msg.EventType,
			&msg.PayloadJSON,
			&msg.Status,
			&msg.PublishAttempts,
			&msg.LastError,
			&msg.CreatedAt,
			&msg.PublishedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

func (r *TaskRepository) MarkOutboxPublished(ctx context.Context, outboxID, taskID, messageID string) error {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE outbox_messages
SET status = ?, published_at = ?, last_error = NULL
WHERE id = ?`, model.OutboxStatusPublished, now, outboxID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE notification_tasks
SET status = ?, mq_message_id = ?, updated_at = ?
WHERE id = ? AND status = ?`, model.StatusQueued, messageID, now, taskID, model.StatusPending); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *TaskRepository) MarkOutboxFailed(ctx context.Context, outboxID string, publishErr error) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE outbox_messages
SET status = ?, publish_attempts = publish_attempts + 1, last_error = ?
WHERE id = ?`, model.OutboxStatusFailed, publishErr.Error(), outboxID)
	return err
}

func (r *TaskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE notification_tasks
SET status = ?, updated_at = ?
WHERE id = ? AND status IN (?, ?, ?)`,
		model.StatusProcessing, time.Now().UTC(), taskID, model.StatusQueued, model.StatusRetrying, model.StatusProcessing,
	)
	return err
}

func (r *TaskRepository) MarkSucceeded(ctx context.Context, taskID string, statusCode int) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE notification_tasks
SET status = ?, last_status_code = ?, last_error = NULL, updated_at = ?
WHERE id = ?`, model.StatusSucceeded, statusCode, time.Now().UTC(), taskID)
	return err
}

func (r *TaskRepository) MarkRetrying(ctx context.Context, taskID string, retryCount int, nextRetryAt time.Time, statusCode int, lastErr string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE notification_tasks
SET status = ?, retry_count = ?, next_retry_at = ?, last_status_code = ?, last_error = ?, updated_at = ?
WHERE id = ?`,
		model.StatusRetrying, retryCount, nextRetryAt.UTC(), nullableStatus(statusCode), lastErr, time.Now().UTC(), taskID,
	)
	return err
}

func (r *TaskRepository) MarkFailed(ctx context.Context, taskID string, statusCode int, lastErr string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE notification_tasks
SET status = ?, last_status_code = ?, last_error = ?, updated_at = ?
WHERE id = ?`, model.StatusFailed, nullableStatus(statusCode), lastErr, time.Now().UTC(), taskID)
	return err
}

func selectTaskSQL() string {
	return `
SELECT id, app_id, vendor, target_url, method, headers, payload, idempotency_key, status,
       retry_count, max_retries, next_retry_at, last_status_code, last_error,
       mq_message_id, created_at, updated_at
FROM notification_tasks`
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner rowScanner) (*model.NotificationTask, error) {
	var task model.NotificationTask
	if err := scanner.Scan(
		&task.ID,
		&task.AppID,
		&task.Vendor,
		&task.TargetURL,
		&task.Method,
		&task.HeadersJSON,
		&task.PayloadJSON,
		&task.IdempotencyKey,
		&task.Status,
		&task.RetryCount,
		&task.MaxRetries,
		&task.NextRetryAt,
		&task.LastStatusCode,
		&task.LastError,
		&task.MQMessageID,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, err
	}
	return &task, nil
}

func nullableStatus(statusCode int) any {
	if statusCode <= 0 {
		return nil
	}
	return statusCode
}

func isDuplicateKey(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
