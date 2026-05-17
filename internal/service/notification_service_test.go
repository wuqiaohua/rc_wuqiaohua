package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wujunqi/rc_wujunqi/internal/model"
)

type fakeNotificationRepo struct {
	existing *model.NotificationTask
	created  bool
	err      error
}

func (f fakeNotificationRepo) CreateTaskWithOutbox(ctx context.Context, input CreateNotificationInput) (*model.NotificationTask, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	if f.existing != nil {
		return f.existing, f.created, nil
	}
	headersJSON, _ := json.Marshal(input.Headers)
	return &model.NotificationTask{
		ID:             "ntf_1",
		AppID:          input.AppID,
		Vendor:         input.Vendor,
		TargetURL:      input.TargetURL,
		Method:         input.Method,
		HeadersJSON:    headersJSON,
		PayloadJSON:    []byte(input.Payload),
		IdempotencyKey: sql.NullString{String: input.IdempotencyKey, Valid: input.IdempotencyKey != ""},
		Status:         model.StatusPending,
		RetryCount:     0,
		MaxRetries:     input.MaxRetries,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}, true, nil
}

func (f fakeNotificationRepo) GetTaskForApp(ctx context.Context, appID, id string) (*model.NotificationTask, error) {
	if f.existing == nil {
		return nil, ErrNotFound
	}
	return f.existing, nil
}

func TestCreateNormalizesRequest(t *testing.T) {
	svc := NewNotificationService(fakeNotificationRepo{})
	task, created, err := svc.Create(context.Background(), CreateNotificationInput{
		AppID:          "biz-payment",
		Vendor:         "crm",
		Payload:        json.RawMessage(`{ "b": 2 }`),
		IdempotencyKey: "evt_1",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true")
	}
	if task.Method != "POST" {
		t.Fatalf("expected default method POST, got %s", task.Method)
	}
	if task.TargetURL != "http://mock-vendor:9000/ok" {
		t.Fatalf("expected configured target url, got %s", task.TargetURL)
	}
	if string(task.PayloadJSON) != `{"b":2}` {
		t.Fatalf("expected compacted payload, got %s", task.PayloadJSON)
	}
}

func TestCreateDetectsIdempotencyConflict(t *testing.T) {
	headersJSON, _ := json.Marshal(map[string]string{})
	existing := &model.NotificationTask{
		ID:             "ntf_1",
		AppID:          "biz-payment",
		Vendor:         "crm",
		TargetURL:      "https://example.com/old",
		Method:         "POST",
		HeadersJSON:    headersJSON,
		PayloadJSON:    []byte(`{"a":1}`),
		IdempotencyKey: sql.NullString{String: "evt_1", Valid: true},
	}
	svc := NewNotificationService(fakeNotificationRepo{existing: existing, created: false})
	_, _, err := svc.Create(context.Background(), CreateNotificationInput{
		AppID:          "biz-payment",
		Vendor:         "crm",
		Payload:        json.RawMessage(`{"a":1}`),
		IdempotencyKey: "evt_1",
	})
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}

func TestCreateReturnsExistingForSameIdempotentRequest(t *testing.T) {
	headersJSON, _ := json.Marshal(map[string]string{"Content-Type": "application/json"})
	existing := &model.NotificationTask{
		ID:             "ntf_1",
		AppID:          "biz-payment",
		Vendor:         "crm",
		TargetURL:      "http://mock-vendor:9000/ok",
		Method:         "POST",
		HeadersJSON:    headersJSON,
		PayloadJSON:    []byte(`{"a":1}`),
		IdempotencyKey: sql.NullString{String: "evt_1", Valid: true},
	}
	svc := NewNotificationService(fakeNotificationRepo{existing: existing, created: false})
	task, created, err := svc.Create(context.Background(), CreateNotificationInput{
		AppID:          "biz-payment",
		Vendor:         "crm",
		Payload:        json.RawMessage(`{"a":1}`),
		IdempotencyKey: "evt_1",
	})
	if err != nil {
		t.Fatalf("expected existing request to be accepted, got %v", err)
	}
	if created {
		t.Fatalf("expected created=false")
	}
	if task.ID != existing.ID {
		t.Fatalf("expected existing task %s, got %s", existing.ID, task.ID)
	}
}

func TestCreateRejectsInvalidPayload(t *testing.T) {
	svc := NewNotificationService(fakeNotificationRepo{})
	_, _, err := svc.Create(context.Background(), CreateNotificationInput{
		AppID:   "biz-payment",
		Vendor:  "crm",
		Payload: json.RawMessage(`{bad-json`),
	})
	if !errors.Is(err, ErrInvalidNotification) {
		t.Fatalf("expected invalid notification, got %v", err)
	}
}

func TestCreateRejectsUnauthorizedVendor(t *testing.T) {
	svc := NewNotificationService(fakeNotificationRepo{})
	_, _, err := svc.Create(context.Background(), CreateNotificationInput{
		AppID:   "biz-payment",
		Vendor:  "unknown",
		Payload: json.RawMessage(`{"a":1}`),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
