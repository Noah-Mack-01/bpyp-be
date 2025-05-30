package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	llm "noerkrieg.com/server/llm"
)

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

func (w *WorkQueue) worker(id int) {
	defer w.wg.Done()
	workerID := fmt.Sprintf("worker-%d", id)

	log.Printf("Worker %s started", workerID)

	// Get the notification channel
	notificationChan := w.store.GetNotificationChannel()

	// For handling retries and backoff
	backoff := 100 * time.Millisecond
	maxBackoff := 5 * time.Second

	// This provides a way to check for pending jobs on startup or after errors
	checkForPendingJobs := true

	for {
		if checkForPendingJobs {
			// Check for pending jobs actively (useful at startup and after errors)
			select {
			case <-w.shutdown:
				log.Printf("Worker %s shutting down", workerID)
				return
			default:
				// Try to claim a job
				job, err := w.store.claim()
				if err != nil {
					log.Printf("Worker %s error claiming job: %v", workerID, err)
					// Use exponential backoff for errors
					time.Sleep(backoff)
					backoff = min(backoff*2, maxBackoff)
				} else if job != nil {
					// Process the claimed job
					w.processClaimedJob(job, workerID)
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
			case <-w.shutdown:
				log.Printf("Worker %s shutting down", workerID)
				return
			case job := <-notificationChan:
				// Got a notification about a new job
				log.Printf("Worker %s received notification for job %s", workerID, job.ID)
				// Try to claim this specific job
				claimedJob, err := w.store.claim()
				if err != nil {
					log.Printf("Worker %s error claiming notified job: %v", workerID, err)
					time.Sleep(backoff)
					backoff = min(backoff*2, maxBackoff)
					// Check for other pending jobs
					checkForPendingJobs = true
				} else if claimedJob != nil {
					// Process the claimed job
					w.processClaimedJob(claimedJob, workerID)
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

func (w *WorkQueue) processClaimedJob(job *Job, workerID string) {
	log.Printf("Worker %s processing job %s", workerID, job.ID)
	result, err := w.processJob(job)

	if err != nil {
		log.Printf("Worker %s job processing error: %v", workerID, err)
		job.Status = StatusFailed
		job.Error = err.Error()

		// Handle update errors
		if updateErr := w.store.updateJob(job); updateErr != nil {
			log.Printf("Worker %s error updating failed job: %v", workerID, updateErr)
		}
	} else {
		job.Status = StatusCompleted
		job.Result = result
		job.Error = ""

		// Handle update errors
		if updateErr := w.store.updateJob(job); updateErr != nil {
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

func (w *WorkQueue) processJob(job *Job) (json.RawMessage, error) {
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
	processed, err := llm.ProcessMessage(message)
	if err != nil {
		log.Printf("Error on sending message to Wit: %v", err)
		return nil, err
	}

	response, uploadErrors, err := w.store.upload(processed, job.UserID, message)
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
		if len(response) > 0 {
			// We have at least some successful results - consider it a partial success
			log.Printf("Job completed with %d errors, but some exercises were successfully processed",
				len(uploadErrors))

			// Create a response that includes error information
			resultMap := map[string]interface{}{
				"data":            json.RawMessage(response),
				"partial_success": true,
				"error_count":     len(uploadErrors),
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
