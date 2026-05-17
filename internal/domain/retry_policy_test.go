package domain

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRetryPolicyRetryableStatus(t *testing.T) {
	policy := RetryPolicy{BaseDelay: time.Second, MaxDelay: time.Minute, MaxRetries: 5}
	decision := policy.Decide(http.StatusServiceUnavailable, nil, 1)
	if !decision.Retryable {
		t.Fatalf("expected 503 to be retryable")
	}
	if decision.Delay != 2*time.Second {
		t.Fatalf("expected exponential delay, got %s", decision.Delay)
	}
}

func TestRetryPolicyNonRetryableStatus(t *testing.T) {
	policy := DefaultRetryPolicy()
	decision := policy.Decide(http.StatusBadRequest, nil, 0)
	if decision.Retryable {
		t.Fatalf("expected 400 to be non-retryable")
	}
}

func TestRetryPolicyMaxRetries(t *testing.T) {
	policy := RetryPolicy{BaseDelay: time.Second, MaxDelay: time.Minute, MaxRetries: 2}
	decision := policy.Decide(http.StatusInternalServerError, nil, 2)
	if decision.Retryable {
		t.Fatalf("expected retry to stop when max retries reached")
	}
}

func TestRetryPolicyUnknownError(t *testing.T) {
	policy := DefaultRetryPolicy()
	decision := policy.Decide(0, errors.New("bad request shape"), 0)
	if decision.Retryable {
		t.Fatalf("plain errors should not be retried by default")
	}
}
