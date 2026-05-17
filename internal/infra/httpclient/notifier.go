package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/wujunqi/rc_wujunqi/internal/model"
)

type DeliveryResult struct {
	StatusCode int
	Success    bool
}

type Notifier struct {
	client *http.Client
}

func NewNotifier(timeout time.Duration) *Notifier {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Notifier{
		client: &http.Client{Timeout: timeout},
	}
}

func (n *Notifier) Send(ctx context.Context, task *model.NotificationTask) (DeliveryResult, error) {
	req, err := http.NewRequestWithContext(ctx, task.Method, task.TargetURL, bytes.NewReader(task.PayloadJSON))
	if err != nil {
		return DeliveryResult{}, err
	}
	for key, value := range task.Headers() {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := task.IdempotencyKeyValue(); key != "" && req.Header.Get("Idempotency-Key") == "" {
		req.Header.Set("Idempotency-Key", key)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return DeliveryResult{}, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return DeliveryResult{
		StatusCode: resp.StatusCode,
		Success:    resp.StatusCode >= 200 && resp.StatusCode <= 299,
	}, nil
}
