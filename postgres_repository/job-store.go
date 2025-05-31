package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	llm "noerkrieg.com/server/llm"
)

type SupabaseStore struct {
	// Connection pool for session/listener operations
	Pool *pgxpool.Pool
	// Dedicated connection for listening to notifications
	Connection *pgx.Conn
	// Channels for job notifications
	jobNotificationChan chan *Job
	// Context and cancel function for listener
	listenerCtx    context.Context
	listenerCancel context.CancelFunc
}

func NewSupabaseStore(SessionUrl string) (*SupabaseStore, error) {
	sessionConfig, err := pgxpool.ParseConfig(SessionUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing Session URL: %w", err)
	}

	// Configure the connection pools
	sessionConfig.MaxConns = 8
	sessionConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	log.Printf("Allowed %d connections", sessionConfig.MaxConns)
	// Disable prepared statement cache to avoid collisions

	// Create the listener pool for session/notification operations
	listenerPool, err := pgxpool.NewWithConfig(context.Background(), sessionConfig)
	if err != nil {
		listenerPool.Close()
		return nil, fmt.Errorf("error creating listener pool: %w", err)
	}

	// Test the listener pool connection
	if err := listenerPool.Ping(context.Background()); err != nil {
		listenerPool.Close()
		return nil, fmt.Errorf("error pinging listener pool: %w", err)
	}

	// Create a dedicated connection for the PostgreSQL LISTEN/NOTIFY mechanism
	listenerConn, err := pgx.ConnectConfig(context.Background(), sessionConfig.ConnConfig)
	if err != nil {
		listenerPool.Close()
		return nil, fmt.Errorf("error creating listener connection: %w", err)
	}

	// Create context with cancel for the listener
	listenerCtx, listenerCancel := context.WithCancel(context.Background())

	// Create and return the store with both connection pools and the listener connection
	store := &SupabaseStore{
		Pool:                listenerPool,
		Connection:          listenerConn,
		jobNotificationChan: make(chan *Job, 100), // Buffer for 100 notifications
		listenerCtx:         listenerCtx,
		listenerCancel:      listenerCancel,
	}

	return store, nil
}

func (j *SupabaseStore) get(id string) (*Job, error) {
	query := `SELECT id, status, data, result, error, created_at, updated_At, retry_count, user_id FROM jobs where id = $1::uuid`
	var job Job
	err := j.Pool.QueryRow(context.Background(), query, id).Scan(
		&job.ID,
		&job.Status,
		&job.Data,
		&job.Result,
		&job.Error,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.RetryCount,
		&job.UserID,
	)
	if err == pgx.ErrNoRows {
		log.Printf("No job found with id %v", id)
		return nil, nil
	} else if err != nil {
		log.Printf("Error on query for row %v: %v", id, err)
		return nil, err
	} else {
		return &job, nil
	}
}

func (s *SupabaseStore) updateJob(job *Job) error {
	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}

	// Track if the transaction has been closed
	txClosed := false

	// Make sure we don't try to roll back a committed transaction
	defer func() {
		if !txClosed && tx != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
				log.Printf("Error rolling back transaction: %v", rbErr)
			}
		}
	}()

	// Get current retry count to prevent race conditions
	var currentRetryCount int
	err = tx.QueryRow(ctx,
		"SELECT retry_count FROM jobs WHERE id = $1::uuid FOR UPDATE",
		job.ID).Scan(&currentRetryCount)

	if err != nil {
		return err
	}

	// Only increment if this is a failure update
	if job.Status == StatusFailed {
		job.RetryCount = currentRetryCount + 1
	}

	// Update timestamp
	job.UpdatedAt = time.Now()

	// Update the record
	_, err = tx.Exec(ctx,
		`UPDATE jobs SET 
			status = $1::text, 
			result = $2::jsonb, 
			error = $3::text, 
			updated_at = $4::timestamptz, 
			retry_count = $5::integer
		WHERE id = $6::uuid`,
		job.Status, job.Result, job.Error, job.UpdatedAt, job.RetryCount, job.ID)

	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	txClosed = true
	return err
}

// claim claims a job for processing with SKIP LOCKED to prevent race conditions
func (s *SupabaseStore) claim() (*Job, error) {
	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	// Track transaction state to prevent double-close issues
	txClosed := false

	// Ensure transaction gets cleaned up properly
	defer func() {
		if !txClosed && tx != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
				log.Printf("Error rolling back transaction: %v", rbErr)
			}
		}
	}()

	var job Job
	err = tx.QueryRow(ctx, `
		UPDATE jobs
		SET status = $1::text, updated_at = $2::timestamptz
		WHERE id = (
			SELECT id FROM jobs
			WHERE (status = $3::text OR (status = $4::text AND retry_count < 3))
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, status, data, result, error, created_at, updated_at, retry_count, user_id
	`, StatusProcessing, time.Now(), StatusPending, StatusFailed).Scan(
		&job.ID,
		&job.Status,
		&job.Data,
		&job.Result,
		&job.Error,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.RetryCount,
		&job.UserID,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			// No jobs available - not an error
			txClosed = true // Mark transaction as handled
			if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
				log.Printf("Error rolling back transaction: %v", rbErr)
			}
			return nil, nil
		}
		return nil, fmt.Errorf("error claiming job: %w", err)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	txClosed = true // Mark transaction as handled
	if err != nil {
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	return &job, nil
}

// GetPendingJobCount returns the number of pending jobs
func (s *SupabaseStore) GetPendingJobCount() (int, error) {
	var count int
	query := `
	SELECT COUNT(*) FROM jobs 
	WHERE status = $1::text OR (status = $2::text AND retry_count < 3)
	`

	err := s.Pool.QueryRow(context.Background(), query, StatusPending, StatusFailed).Scan(&count)
	return count, err
}

// StartListener starts the PostgreSQL notification listener
func (s *SupabaseStore) StartListener() error {
	log.Println("Starting PostgreSQL notification listener")

	// Ensure we have a valid listener connection
	if s.Connection == nil {
		// Create a new connection if it doesn't exist
		var err error
		s.Connection, err = pgx.ConnectConfig(s.listenerCtx, s.Pool.Config().ConnConfig)
		if err != nil {
			return fmt.Errorf("error creating listener connection: %w", err)
		}
	}

	// Start listening for notifications on the 'job_updates' channel
	_, err := s.Connection.Exec(s.listenerCtx, "LISTEN job_updates")
	if err != nil {
		s.Connection.Close(context.Background())
		s.Connection = nil
		return fmt.Errorf("error listening to job_updates channel: %w", err)
	}

	log.Println("Successfully subscribed to 'job_updates' notifications")

	// Start goroutine to handle notifications
	go func() {
		conn := s.Connection // Local reference to the connection
		defer func() {
			if conn != nil {
				conn.Close(context.Background())
			}
			log.Println("Notification listener stopped")
		}()

		for {
			notification, err := conn.WaitForNotification(s.listenerCtx)
			if err != nil {
				// Check if context was canceled (normal shutdown)
				if s.listenerCtx.Err() != nil {
					return
				}

				log.Printf("Error waiting for notification: %v. Attempting to reconnect...", err)

				// Close the current connection
				if conn != nil {
					conn.Close(context.Background())
				}

				// Try to reconnect
				conn, err = pgx.ConnectConfig(s.listenerCtx, s.Pool.Config().ConnConfig)
				if err != nil {
					log.Printf("Failed to reconnect: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}

				// Re-establish the listener
				_, err = conn.Exec(s.listenerCtx, "LISTEN job_updates")
				if err != nil {
					log.Printf("Failed to re-establish listener: %v", err)
					conn.Close(context.Background())
					conn = nil
					time.Sleep(5 * time.Second)
					continue
				}

				// Update the store's connection reference
				s.Connection = conn
				log.Println("Successfully reconnected and subscribed to 'job_updates'")
				continue
			}

			log.Printf("Received notification on channel: %s", notification.Channel)

			// Parse the notification payload
			var payload struct {
				ID        string    `json:"id"`
				Status    string    `json:"status"`
				UpdatedAt time.Time `json:"updated_at"`
				UserID    string    `json:"user_id"`
				Operation string    `json:"operation"`
			}

			if err := json.Unmarshal([]byte(notification.Payload), &payload); err != nil {
				log.Printf("Error parsing notification payload: %v", err)
				continue
			}

			log.Printf("Notification payload: job %s status %s user_id %s operation %s",
				payload.ID, payload.Status, payload.UserID, payload.Operation)

			// If this is a new or updated job with a pending status, fetch it and send to the channel
			if payload.Status == StatusPending || (payload.Status == StatusFailed && payload.Operation == "UPDATE") {
				job, err := s.get(payload.ID)
				if err != nil {
					log.Printf("Error fetching job from notification: %v", err)
					continue
				}

				if job != nil {
					select {
					case s.jobNotificationChan <- job:
						log.Printf("Notification for job %s sent to channel", job.ID)
					default:
						log.Printf("Warning: Notification channel full, dropped update for job %s", job.ID)
					}
				}
			}
		}
	}()

	log.Println("PostgreSQL notification listener started successfully")
	return nil
}

// GetNotificationChannel returns the channel for job notifications
func (s *SupabaseStore) GetNotificationChannel() <-chan *Job {
	return s.jobNotificationChan
}

// StopListener stops the notification listener
func (s *SupabaseStore) StopListener() {
	log.Println("Stopping PostgreSQL notification listener")
	if s.listenerCancel != nil {
		s.listenerCancel()
	}
}

// Close closes the database connection pools and stops the listener
func (s *SupabaseStore) Close() {
	// Ensure the listener connection is closed
	if s.Connection != nil {
		s.Connection.Close(context.Background())
		s.Connection = nil
	}

	s.StopListener()
	if s.Pool != nil {
		s.Pool.Close()
	}
}

// upload uploads exercises to the database using the direct PostgreSQL connection
func (s *SupabaseStore) upload(exercises []llm.Exercise, userID string, message string) ([]byte, []error, error) {
	log.Print(exercises)
	errors := make([]error, 0)
	compiled := make([]map[string]interface{}, 0)
	stats := struct {
		Total     int
		Succeeded int
		Failed    int
	}{
		Total: len(exercises),
	}

	// Set common fields on all exercises
	now := time.Now()
	for i := range exercises {
		exercises[i].UserId = userID
		exercises[i].Summary = fmt.Sprintf(`"%v"`, message)
		exercises[i].Timestamp = now
	}

	ctx := context.Background()

	for _, ex := range exercises {
		// Attempt to upsert the exercise using direct PostgreSQL connection
		query := `
			INSERT INTO exercises (
				exercise_name, summary, type, sets, work, work_type,
				resistance, resistance_type, duration, attributes, user_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) 
			ON CONFLICT (id) DO UPDATE SET
				exercise_name = $1,
				summary = $2,
				type = $3,
				sets = $4,
				work = $5,
				work_type = $6,
				resistance = $7,
				resistance_type = $8,
				duration = $9,
				attributes = $10,
				user_id = $11
			RETURNING *;
		`

		// Convert string array to proper PostgreSQL array
		var attributes []string
		if ex.Attributes != nil {
			attributes = ex.Attributes
		} else {
			attributes = []string{}
		}

		var result pgx.Rows
		result, err := s.Pool.Query(ctx, query,
			ex.Exercise,
			ex.Summary,
			ex.Type,
			ex.Sets,
			ex.Quantity,
			ex.QuantityType,
			ex.Resistance,
			ex.ResistanceType,
			ex.Duration,
			attributes,
			userID,
		)

		if err != nil {
			log.Printf("Failed to insert exercise %s: %v", ex.Exercise, err)
			errors = append(errors, fmt.Errorf("failed to insert exercise %s: %w", ex.Exercise, err))
			stats.Failed++
			continue
		}

		// Process the result
		var inserted map[string]interface{}
		rows, err := pgx.CollectRows(result, pgx.RowToMap)
		if err != nil {
			nErr := fmt.Errorf("error collecting rows for exercise %s: %w", ex.Exercise, err)
			errors = append(errors, nErr)
			stats.Failed++
			continue
		}

		if len(rows) == 0 {
			nErr := fmt.Errorf("empty response for exercise %s", ex.Exercise)
			errors = append(errors, nErr)
			stats.Failed++
			continue
		}

		// Record success
		inserted = rows[0]
		compiled = append(compiled, inserted)
		stats.Succeeded++
	}

	// Marshal results
	result, err := json.Marshal(compiled)
	if err != nil {
		return nil, errors, fmt.Errorf("failed to marshal compiled results: %w", err)
	}

	// Log operation summary
	log.Printf("Exercise upload summary: total=%d, succeeded=%d, failed=%d",
		stats.Total, stats.Succeeded, stats.Failed)
	if len(errors) > 0 {
		log.Printf("Upload errors: %v", errors)
	}

	return result, errors, nil
}
