package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"noerkrieg.com/server/repository"
	"noerkrieg.com/server/wit"
)

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

func (c *Controller) GetExercise(writer http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	eid := query.Get("eid")
	if eid == "" {
		http.Error(writer, "Request eid cannot be empty", http.StatusBadRequest)
		return
	}
	jwt := strings.Split(r.Header.Get("Authorization"), " ")[1]
	uid, err := repository.VerifyAndGetUserID(jwt)
	if err != nil {
		http.Error(writer, "Could not validate jwt", http.StatusBadRequest)
		return
	}
	ex, err := repository.GetExercise(eid, uid)
	if err != nil {
		http.Error(writer, fmt.Sprintf("Could not query eid %v", eid), http.StatusNotFound)
		return
	}
	writer.WriteHeader(http.StatusOK)
	writer.Header().Set("Content-Type", "application/json")
	if _, err := writer.Write(ex); err != nil {
		http.Error(writer, "Error on writing JSON to response", http.StatusInternalServerError)
		return
	}
}

func (c *Controller) CreateExercise(writer http.ResponseWriter, r *http.Request) {
	var req wit.Exercise
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(writer, "Could not decode request", http.StatusBadRequest)
		return
	}

	jwt := strings.Split(r.Header.Get("Authorization"), " ")[1]
	userID, err := repository.VerifyAndGetUserID(jwt)
	if jwt == "" || err != nil {
		http.Error(writer, "Could not verify jwt", http.StatusBadRequest)
		return
	}

	response, errors, err := repository.UploadExercises([]wit.Exercise{req}, userID, "")

	// Set the content type header
	writer.Header().Set("Content-Type", "application/json")

	if err != nil {
		// Critical error occurred
		log.Printf("Critical error during exercise upload: %v", err)
		http.Error(writer, "Error on upload: "+err.Error(), http.StatusInternalServerError)
		return
	} else if len(errors) != 0 {
		// Some errors occurred, but we still have partial results
		errMsgs := make([]string, len(errors))
		for i, e := range errors {
			errMsgs[i] = e.Error()
		}

		// Include error details in response header
		writer.Header().Set("X-Error-Details", strings.Join(errMsgs, "; "))
		writer.WriteHeader(http.StatusPartialContent)
	} else {
		// Complete success
		writer.WriteHeader(http.StatusOK)
	}

	// Write the successful portion of the response
	if response != nil {
		writer.Write(response)
	} else {
		// No successful exercises were processed
		writer.Write([]byte("[]"))
	}
}
