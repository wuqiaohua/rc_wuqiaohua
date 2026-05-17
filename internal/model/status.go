package model

const (
	StatusPending    = "pending"
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusRetrying   = "retrying"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
)

func IsTerminalStatus(status string) bool {
	return status == StatusSucceeded || status == StatusFailed
}
