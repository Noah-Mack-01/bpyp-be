package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"noerkrieg.com/server/wit"
)

type Controller struct{}

func (c Controller) ProcessMessage(writer http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	message := queryParams.Get("message")
	processed, err := wit.ProcessMessage(message)
	if err != nil {
		log.Fatalf("Error on processing message in wit.ai: %v", err)
	}
	response := wit.PostProcess(processed)

	if err != nil {
		writer.WriteHeader(422)
		writer.Write([]byte(fmt.Sprintf("Could not issue message to NLP with message %v: %v", message, err)))
		return
	} else {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(200)
		writer.Write(response)
	}

}

func (c Controller) GetExercises(writer http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	userID := queryParams.Get("uid")     // Access the "uid" query parameter
	exerciseId := queryParams.Get("eid") // Access the "eid" query parameter

	if userID == "" || exerciseId == "" {
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
