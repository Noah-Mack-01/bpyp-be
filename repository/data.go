package repository

import (
	"encoding/json"
	"sync"
	"time"
)

type Job struct {
	ID         string          `json:"id"`
	Status     string          `json:"status"`
	Data       json.RawMessage `json:"data"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	RetryCount int             `json:"-"` // Hidden from API responses
	UserID     string          `json:"user_id,omitempty"`
}

const (
	StatusPending    = "pending"
	StatusQueued     = "queued" // Added new status to mark jobs that are in the channel
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

type WorkQueue struct {
	workers  int
	store    *SupabaseStore
	wg       sync.WaitGroup
	shutdown chan struct{}
}
