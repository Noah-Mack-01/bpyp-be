package controller

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"noerkrieg.com/server/repository"
)

func (c *Controller) GetExercises(writer http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	userSession := r.Header.Get("Authorization")
	exerciseId := queryParams.Get("eid") // Access the "eid" query parameter

	if userSession == "" || exerciseId == "" {
		writer.WriteHeader(http.StatusBadRequest)
		writer.Write([]byte("Missing uid or eid query parameter"))
		return
	}

	// Mock exercise JSON
	mockExercise := map[string]any{
		"key":             exerciseId,
		"exercise":        "Bench Press",
		"summary":         "5 sets of 5 reps with 185lbs",
		"type":            "anaerobic",
		"sets":            5,
		"work":            5,
		"workUnit":        "repetitions",
		"resistance":      185,
		"resistanceUnits": "pounds",
		"timeStamp":       "2025-05-03T12:00:00Z",
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	json.NewEncoder(writer).Encode(mockExercise)
}

func (c *Controller) GetHealth(writer http.ResponseWriter, r *http.Request) {
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("OK"))
}

func (c *Controller) GetJobStatus(writer http.ResponseWriter, r *http.Request) {
	queries := r.URL.Query()
	id := queries.Get("id")
	job, err := getJobStatusHandler(c.SupabaseStore, id)
	if err != nil {
	}
	if job == nil {
	}
	writer.Header().Set("Content-Type", "application/json")

	// Set status code based on job status
	if job.Status == repository.StatusPending || job.Status == repository.StatusProcessing {
		writer.WriteHeader(http.StatusAccepted) // 202
	} else {
		writer.WriteHeader(http.StatusOK) // 200
	}

	if err := json.NewEncoder(writer).Encode(job); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func (c *Controller) CreateJob(writer http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(writer, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request data
	if len(req.Data) == 0 {
		http.Error(writer, "Request data cannot be empty", http.StatusBadRequest)
		return
	}

	// Create new job
	jobID := uuid.New().String()
	now := time.Now()
	jwt := strings.Split(r.Header.Get("Authorization"), " ")[1]
	userID, err := repository.VerifyAndGetUserID(jwt)
	if err != nil {
		log.Printf("Could not find user for session, encountered %v", err)
		http.Error(writer, "Failed to create job", http.StatusUnauthorized)
		return
	}
	log.Print(userID)
	job := &repository.Job{
		ID:         jobID,
		Status:     repository.StatusPending,
		Data:       req.Data,
		CreatedAt:  now,
		UpdatedAt:  now,
		RetryCount: 0,
		UserID:     userID,
	}
	if err := submitJobHandler(job, c.SupabaseStore); err != nil {
		log.Printf("Error creating job: %v", err)
		http.Error(writer, "Failed to create job", http.StatusInternalServerError)
		return
	}

	// Return 202 Accepted with job info
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)

	resp := JobResponse{
		JobID:          jobID,
		Status:         repository.StatusPending,
		StatusEndpoint: "/v1/job?id=" + jobID,
		CreatedAt:      job.CreatedAt,
	}

	if err := json.NewEncoder(writer).Encode(resp); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
