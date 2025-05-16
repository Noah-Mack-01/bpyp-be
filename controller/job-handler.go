package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"noerkrieg.com/server/repository"
)

func submitJobHandler(job *repository.Job, s *repository.SupabaseStore) error {
	ctx, cancel := withTimeout(context.Background())
	defer cancel()

	// Use explicit parameter types to avoid statement cache issues
	query := `
	INSERT INTO jobs (id, status, data, result, error, created_at, updated_at, retry_count, user_id)
	VALUES ($1::uuid, $2::text, $3::jsonb, $4::jsonb, $5::text, $6::timestamptz, $7::timestamptz, $8::integer, $9::uuid)`

	_, err := s.Pool.Exec(
		ctx, query, //mandatory
		job.ID, // optionals
		job.Status,
		job.Data,
		job.Result,
		job.Error,
		job.CreatedAt,
		job.UpdatedAt,
		job.RetryCount,
		job.UserID,
	)

	return err
}

func getJobStatusHandler(store *repository.SupabaseStore, jobId string) (*repository.Job, error) {
	ctx, cancel := withTimeout(context.Background())
	defer cancel()

	// Use explicit type casting for parameters
	query := `SELECT id, status, data, result, error, created_at, updated_at, retry_count 
		FROM jobs WHERE id = $1::uuid`

	var job repository.Job
	err := store.Pool.QueryRow(ctx, query, jobId).Scan(
		&job.ID,
		&job.Status,
		&job.Data,
		&job.Result,
		&job.Error,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.RetryCount,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("database error in GetJob: %w", err) // Wrap error with context
	}

	return &job, nil
}

func withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 10*time.Second)
}
