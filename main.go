package main

import (
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	repository "noerkrieg.com/server/postgres_repository"
)

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	var queue *repository.WorkQueue
	var supabaseStore *repository.SupabaseStore
	var router *chi.Mux

	supabaseStore, err := repository.NewSupabaseStore(os.Getenv("BPYP_POSTGRES_DIR_CONN"))
	if err != nil {
		log.Fatalf("Could not create SupabaseStore. Encountered error: %v", err)
	}

	defer supabaseStore.Close()

	cpuCount := runtime.NumCPU()
	multiplier := 2
	if multiplierEnv := os.Getenv("BPYP_WORKER_MULTIPLIER"); multiplierEnv != "" {
		if m, err := strconv.Atoi(multiplierEnv); err == nil && m > 0 {
			multiplier = m
		}
	}

	workerCount := max(1, cpuCount*multiplier/runtime.GOMAXPROCS(0))
	log.Printf("Starting with %d workers (CPU count: %d, multiplier: %d)",
		workerCount, cpuCount, multiplier)

	queue = repository.NewWorkQueue(workerCount, supabaseStore)
	queue.Start()

	router = chi.NewRouter()
	router.Use(middleware.Logger)

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET" /*"POST", "PUT", "DELETE",*/, "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Route("/v1", func(r chi.Router) {
		r.Get("/health", func(writer http.ResponseWriter, req *http.Request) {
			writer.WriteHeader(http.StatusOK)
			writer.Write([]byte("OK"))
		})
	})
	http.ListenAndServe(":"+os.Getenv("PORT"), router)
}
