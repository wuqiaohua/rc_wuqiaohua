package domain

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

type RetryPolicy struct {
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	MaxRetries int
}

type RetryDecision struct {
	Retryable bool
	Delay     time.Duration
	Reason    string
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		BaseDelay:  30 * time.Second,
		MaxDelay:   30 * time.Minute,
		MaxRetries: 5,
	}
}

func (p RetryPolicy) Decide(statusCode int, err error, retryCount int) RetryDecision {
	if retryCount >= p.MaxRetries {
		return RetryDecision{Retryable: false, Reason: "max retries reached"}
	}
	if err != nil {
		if isRetryableError(err) {
			return RetryDecision{Retryable: true, Delay: p.delay(retryCount), Reason: err.Error()}
		}
		return RetryDecision{Retryable: false, Reason: err.Error()}
	}
	if isRetryableStatus(statusCode) {
		return RetryDecision{Retryable: true, Delay: p.delay(retryCount), Reason: http.StatusText(statusCode)}
	}
	return RetryDecision{Retryable: false, Reason: http.StatusText(statusCode)}
}

func (p RetryPolicy) delay(retryCount int) time.Duration {
	delay := p.BaseDelay
	for i := 0; i < retryCount; i++ {
		delay *= 2
		if delay >= p.MaxDelay {
			return p.MaxDelay
		}
	}
	return delay
}

func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooManyRequests ||
		(statusCode >= 500 && statusCode <= 599)
}

func isRetryableError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}
