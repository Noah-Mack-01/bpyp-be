package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"noerkrieg.com/server/api"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8081", "http://localhost:3000"}, // Replace with your allowed origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value for preflight request cache
	}))

	r.Get("/v1/translate/{body}", func(writer http.ResponseWriter, r *http.Request) {
		body := []byte(chi.URLParam(r, "body"))
		result, err := api.ProcessMessage(&body)
		if err != nil {
			writer.WriteHeader(422)
			writer.Write([]byte(fmt.Sprintf("Could not issue message to NLP with message %v: %v", body, err)))
			return
		} else {
			writer.WriteHeader(200)
			writer.Write(result)
		}
	})

	r.Get("/v1/exercises", func(writer http.ResponseWriter, r *http.Request) {
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
	})
	r.Get("/v1/translator", func(writer http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		userID := queryParams.Get("uid")     // Access the "uid" query parameter
		exerciseId := queryParams.Get("eid") // Access the "eid" query parameter

		if userID == "" || exerciseId == "" {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte("Missing uid or eid query parameter"))
			return
		}

	})
	http.ListenAndServe(":3000", r)
}
