package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"noerkrieg.com/server/wit"
)

type SupabaseStore struct {
	// Transaction pool for database operations
	DB *pgxpool.Pool
	// Connection pool for session/listener operations
	ListenerPool *pgxpool.Pool
	// Dedicated connection for listening to notifications
	ListenerConn *pgx.Conn
	// Channels for job notifications
	jobNotificationChan chan *Job
	// Context and cancel function for listener
	listenerCtx    context.Context
	listenerCancel context.CancelFunc
}

func NewSupabaseStore(DBUrl string, SessionUrl string) (*SupabaseStore, error) {
	// Parse configurations for both connection pools
	config, err := pgxpool.ParseConfig(DBUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing DB URL: %w", err)
	}

	sessionConfig, err := pgxpool.ParseConfig(SessionUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing Session URL: %w", err)
	}

	// Configure the connection pools
	config.MaxConns = 10
	sessionConfig.MaxConns = 10
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	sessionConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	// Disable prepared statement cache to avoid collisions

	// Create the main DB pool for transactions
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("error creating main DB pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("error pinging main DB: %w", err)
	}

	// Create the listener pool for session/notification operations
	listenerPool, err := pgxpool.NewWithConfig(context.Background(), sessionConfig)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("error creating listener pool: %w", err)
	}

	// Test the listener pool connection
	if err := listenerPool.Ping(context.Background()); err != nil {
		pool.Close()
		listenerPool.Close()
		return nil, fmt.Errorf("error pinging listener pool: %w", err)
	}

	// Create a dedicated connection for the PostgreSQL LISTEN/NOTIFY mechanism
	listenerConn, err := pgx.ConnectConfig(context.Background(), sessionConfig.ConnConfig)
	if err != nil {
		pool.Close()
		listenerPool.Close()
		return nil, fmt.Errorf("error creating listener connection: %w", err)
	}

	// Create context with cancel for the listener
	listenerCtx, listenerCancel := context.WithCancel(context.Background())

	// Create and return the store with both connection pools and the listener connection
	store := &SupabaseStore{
		DB:                  pool,
		ListenerPool:        listenerPool,
		ListenerConn:        listenerConn,
		jobNotificationChan: make(chan *Job, 100), // Buffer for 100 notifications
		listenerCtx:         listenerCtx,
		listenerCancel:      listenerCancel,
	}

	return store, nil
}

// Create adds a new job
func (j *SupabaseStore) Create(job *Job) error {
	log.Printf("job %v", &job)
	query := `
	INSERT INTO jobs (id, status, data, result, error, created_at, updated_at, retry_count, user_id)
	VALUES ($1::uuid, $2::text, $3::jsonb, $4::jsonb, $5::text, $6::timestamptz, $7::timestamptz, $8::integer, $9::text)
	`

	_, err := j.DB.Exec(
		context.Background(),
		query,
		job.ID,
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

func (j *SupabaseStore) Get(id string) (*Job, error) {
	query := `SELECT id, status, data, result, error, created_at, updated_At, retry_count, user_id FROM jobs where id = $1::uuid`
	var job Job
	err := j.DB.QueryRow(context.Background(), query, id).Scan(
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

func (s *SupabaseStore) Update(job *Job) error {
	ctx := context.Background()
	tx, err := s.DB.Begin(ctx)
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

// ClaimJob claims a job for processing with SKIP LOCKED to prevent race conditions
func (s *SupabaseStore) ClaimJob(workerID string) (*Job, error) {
	ctx := context.Background()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		log.Printf("Error on DB.Begin")
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

	err := s.DB.QueryRow(context.Background(), query, StatusPending, StatusFailed).Scan(&count)
	return count, err
}

// StartListener starts the PostgreSQL notification listener
func (s *SupabaseStore) StartListener() error {
	log.Println("Starting PostgreSQL notification listener")

	// Ensure we have a valid listener connection
	if s.ListenerConn == nil {
		// Create a new connection if it doesn't exist
		var err error
		s.ListenerConn, err = pgx.ConnectConfig(s.listenerCtx, s.ListenerPool.Config().ConnConfig)
		if err != nil {
			return fmt.Errorf("error creating listener connection: %w", err)
		}
	}

	// Start listening for notifications on the 'job_updates' channel
	_, err := s.ListenerConn.Exec(s.listenerCtx, "LISTEN job_updates")
	if err != nil {
		s.ListenerConn.Close(context.Background())
		s.ListenerConn = nil
		return fmt.Errorf("error listening to job_updates channel: %w", err)
	}

	log.Println("Successfully subscribed to 'job_updates' notifications")

	// Start goroutine to handle notifications
	go func() {
		conn := s.ListenerConn // Local reference to the connection
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
				conn, err = pgx.ConnectConfig(s.listenerCtx, s.ListenerPool.Config().ConnConfig)
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
				s.ListenerConn = conn
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
				job, err := s.Get(payload.ID)
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
	s.StopListener()

	// Close both connection pools
	if s.DB != nil {
		s.DB.Close()
	}

	if s.ListenerPool != nil {
		s.ListenerPool.Close()
	}

	// Ensure the listener connection is closed
	if s.ListenerConn != nil {
		s.ListenerConn.Close(context.Background())
		s.ListenerConn = nil
	}
}

func NewWorkQueue(workers int, store *SupabaseStore) *WorkQueue {
	return &WorkQueue{
		workers:  workers,
		store:    store,
		shutdown: make(chan struct{}),
	}
}

// Start initializes the worker pool and notification listener
func (p *WorkQueue) Start() {
	// Start the PostgreSQL notification listener
	if err := p.store.StartListener(); err != nil {
		log.Printf("Warning: Failed to start notification listener: %v", err)
		log.Printf("Workers will rely on periodic polling for jobs")
	} else {
		log.Printf("PostgreSQL notification listener started")
	}

	// Start worker goroutines
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	log.Printf("Started %d workers", p.workers)
}

// Shutdown gracefully stops the processor
func (p *WorkQueue) Shutdown(ctx context.Context) {
	close(p.shutdown)

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All workers gracefully stopped")
	case <-ctx.Done():
		log.Println("Shutdown timeout: some workers may still be running")
	}
}

func (p *WorkQueue) worker(id int) {
	defer p.wg.Done()
	workerID := fmt.Sprintf("worker-%d", id)

	log.Printf("Worker %s started", workerID)

	// Get the notification channel
	notificationChan := p.store.GetNotificationChannel()

	// For handling retries and backoff
	backoff := 100 * time.Millisecond
	maxBackoff := 5 * time.Second

	// This provides a way to check for pending jobs on startup or after errors
	checkForPendingJobs := true

	for {
		if checkForPendingJobs {
			// Check for pending jobs actively (useful at startup and after errors)
			select {
			case <-p.shutdown:
				log.Printf("Worker %s shutting down", workerID)
				return
			default:
				// Try to claim a job
				job, err := p.store.ClaimJob(workerID)
				if err != nil {
					log.Printf("Worker %s error claiming job: %v", workerID, err)
					// Use exponential backoff for errors
					time.Sleep(backoff)
					backoff = min(backoff*2, maxBackoff)
				} else if job != nil {
					// Process the claimed job
					processClaimedJob(job, workerID, p.store)
					// Reset backoff on successful operation
					backoff = 100 * time.Millisecond
				} else {
					// No jobs to claim, switch to listening for notifications
					checkForPendingJobs = false
				}
			}
		} else {
			// Wait for notifications or shutdown signal
			select {
			case <-p.shutdown:
				log.Printf("Worker %s shutting down", workerID)
				return
			case job := <-notificationChan:
				// Got a notification about a new job
				log.Printf("Worker %s received notification for job %s", workerID, job.ID)
				// Try to claim this specific job
				claimedJob, err := p.store.ClaimJob(workerID)
				if err != nil {
					log.Printf("Worker %s error claiming notified job: %v", workerID, err)
					time.Sleep(backoff)
					backoff = min(backoff*2, maxBackoff)
					// Check for other pending jobs
					checkForPendingJobs = true
				} else if claimedJob != nil {
					// Process the claimed job
					processClaimedJob(claimedJob, workerID, p.store)
					// Reset backoff on successful operation
					backoff = 100 * time.Millisecond
					// Check for more pending jobs
					checkForPendingJobs = true
				}
			case <-time.After(30 * time.Second):
				// Periodically check for pending jobs even without notifications
				// This provides resilience in case we miss a notification
				log.Printf("Worker %s periodic check for pending jobs", workerID)
				checkForPendingJobs = true
			}
		}
	}
}

// Helper function to process a claimed job
func processClaimedJob(job *Job, workerID string, store *SupabaseStore) {
	log.Printf("Worker %s processing job %s", workerID, job.ID)
	result, err := processJob(job)

	if err != nil {
		log.Printf("Worker %s job processing error: %v", workerID, err)
		job.Status = StatusFailed
		job.Error = err.Error()

		// Handle update errors
		if updateErr := store.Update(job); updateErr != nil {
			log.Printf("Worker %s error updating failed job: %v", workerID, updateErr)
		}
	} else {
		job.Status = StatusCompleted
		job.Result = result
		job.Error = ""

		// Handle update errors
		if updateErr := store.Update(job); updateErr != nil {
			log.Printf("Worker %s error updating completed job: %v", workerID, updateErr)
		} else {
			log.Printf("Worker %s successfully updated job %s", workerID, job.ID)
		}

		// we need to now update exercises with the parsed JSON from job.results.

	}
}

// min returns the minimum of two durations
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func processJob(job *Job) (json.RawMessage, error) {
	var js map[string]interface{}
	if err := json.Unmarshal(job.Data, &js); err != nil {
		log.Printf("Error deserializing job data: %v", err)
		return nil, err
	}
	message, ok := js["message"].(string)
	if !ok {
		log.Printf("Request did not contain message.")
		return nil, fmt.Errorf("request %v did not contain key 'message'", js)
	}
	processed, err := wit.ProcessMessage(message)
	if err != nil {
		log.Printf("Error on sending message to Wit: %v", err)
		return nil, err
	}
	exercises, err := wit.PostProcess(processed)
	if err != nil {
		log.Printf("Error post processing message: %v", err)
		return nil, err
	}

	response, uploadErrors, err := UploadExercises(exercises, job.UserID, message)
	if err != nil {
		// Critical error that prevented any processing
		return nil, fmt.Errorf("critical error in exercise upload: %w", err)
	}

	// Handle partial success case
	if len(uploadErrors) > 0 {
		// Log individual errors
		for i, e := range uploadErrors {
			log.Printf("Upload error %d: %v", i+1, e)
		}

		// Decide whether to treat this as successful with warnings or as a failure
		if response != nil && len(response) > 0 {
			// We have at least some successful results - consider it a partial success
			log.Printf("Job completed with %d errors, but some exercises were successfully processed",
				len(uploadErrors))

			// Create a response that includes error information
			resultMap := map[string]interface{}{
				"data": json.RawMessage(response),
				"partial_success": true,
				"error_count": len(uploadErrors),
			}

			// Convert to JSON
			enhancedResponse, err := json.Marshal(resultMap)
			if err != nil {
				log.Printf("Error creating enhanced response: %v", err)
				// Fall back to returning just the original response
				return response, nil
			}

			return enhancedResponse, nil
		} else {
			// No successful exercises - treat as a failure
			return nil, fmt.Errorf("failed to upload any exercises: %v", uploadErrors[0])
		}
	}

	// Complete success
	return response, nil
}
