package controller

import (
	"encoding/json"
	"time"

	"noerkrieg.com/server/repository"
)

// JobRequest represents the incoming request
type JobRequest struct {
	Data json.RawMessage `json:"data"`
}

type JobResponse struct {
	JobID          string    `json:"job_id"`
	Status         string    `json:"status"`
	StatusEndpoint string    `json:"status_endpoint"`
	CreatedAt      time.Time `json:"created_at"`
}

type Controller struct {
	SupabaseStore *repository.SupabaseStore
	WorkQueue     *repository.WorkQueue
}
