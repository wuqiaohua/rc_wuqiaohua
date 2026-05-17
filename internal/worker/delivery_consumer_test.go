package worker

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/wujunqi/rc_wujunqi/internal/domain"
	"github.com/wujunqi/rc_wujunqi/internal/infra/httpclient"
	"github.com/wujunqi/rc_wujunqi/internal/model"
)

type fakeTaskStore struct {
	task           *model.NotificationTask
	processing     bool
	succeeded      bool
	retrying       bool
	failed         bool
	nextRetryAt    time.Time
	lastRetryCount int
}

func (f *fakeTaskStore) GetTask(ctx context.Context, id string) (*model.NotificationTask, error) {
	if f.task == nil {
		return nil, errors.New("not found")
	}
	return f.task, nil
}

func (f *fakeTaskStore) MarkProcessing(ctx context.Context, taskID string) error {
	f.processing = true
	return nil
}

func (f *fakeTaskStore) MarkSucceeded(ctx context.Context, taskID string, statusCode int) error {
	f.succeeded = true
	return nil
}

func (f *fakeTaskStore) MarkRetrying(ctx context.Context, taskID string, retryCount int, nextRetryAt time.Time, statusCode int, lastErr string) error {
	f.retrying = true
	f.lastRetryCount = retryCount
	f.nextRetryAt = nextRetryAt
	return nil
}

func (f *fakeTaskStore) MarkFailed(ctx context.Context, taskID string, statusCode int, lastErr string) error {
	f.failed = true
	return nil
}

type fakeRetryPublisher struct {
	retried bool
	dead    bool
}

func (f *fakeRetryPublisher) PublishRetry(ctx context.Context, body []byte, delay time.Duration) (string, error) {
	f.retried = true
	return "retry-message", nil
}

func (f *fakeRetryPublisher) PublishDead(ctx context.Context, body []byte) (string, error) {
	f.dead = true
	return "dead-message", nil
}

type fakeNotifier struct {
	result httpclient.DeliveryResult
	err    error
}

func (f fakeNotifier) Send(ctx context.Context, task *model.NotificationTask) (httpclient.DeliveryResult, error) {
	return f.result, f.err
}

func TestDeliveryConsumerSuccess(t *testing.T) {
	store := &fakeTaskStore{task: queuedTask()}
	mq := &fakeRetryPublisher{}
	consumer := NewDeliveryConsumer(store, mq, fakeNotifier{
		result: httpclient.DeliveryResult{StatusCode: 200, Success: true},
	}, domain.DefaultRetryPolicy(), nil)

	err := consumer.process(context.Background(), []byte(`{"task_id":"ntf_1"}`))
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if !store.processing || !store.succeeded {
		t.Fatalf("expected processing and succeeded to be marked")
	}
	if mq.retried || mq.dead {
		t.Fatalf("success should not publish retry/dead messages")
	}
}

func TestDeliveryConsumerRetry(t *testing.T) {
	store := &fakeTaskStore{task: queuedTask()}
	mq := &fakeRetryPublisher{}
	policy := domain.RetryPolicy{BaseDelay: time.Millisecond, MaxDelay: time.Second, MaxRetries: 5}
	consumer := NewDeliveryConsumer(store, mq, fakeNotifier{
		result: httpclient.DeliveryResult{StatusCode: 503, Success: false},
	}, policy, nil)

	err := consumer.process(context.Background(), []byte(`{"task_id":"ntf_1"}`))
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if !store.retrying || !mq.retried {
		t.Fatalf("expected retrying state and retry publish")
	}
	if store.lastRetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", store.lastRetryCount)
	}
}

func TestDeliveryConsumerFailedOnBadRequest(t *testing.T) {
	store := &fakeTaskStore{task: queuedTask()}
	mq := &fakeRetryPublisher{}
	consumer := NewDeliveryConsumer(store, mq, fakeNotifier{
		result: httpclient.DeliveryResult{StatusCode: 400, Success: false},
	}, domain.DefaultRetryPolicy(), nil)

	err := consumer.process(context.Background(), []byte(`{"task_id":"ntf_1"}`))
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if !store.failed || !mq.dead {
		t.Fatalf("expected failed state and dead-letter publish")
	}
}

func queuedTask() *model.NotificationTask {
	return &model.NotificationTask{
		ID:             "ntf_1",
		Vendor:         "crm",
		TargetURL:      "https://example.com/hook",
		Method:         "POST",
		HeadersJSON:    []byte(`{}`),
		PayloadJSON:    []byte(`{"a":1}`),
		IdempotencyKey: sql.NullString{String: "evt_1", Valid: true},
		Status:         model.StatusQueued,
		RetryCount:     0,
		MaxRetries:     5,
	}
}
