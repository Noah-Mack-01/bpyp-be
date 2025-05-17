package main

import (
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"noerkrieg.com/server/repository"
)

func main() {
	var queue *repository.WorkQueue
	var supabaseStore *repository.SupabaseStore
	var router *chi.Mux

	supabaseStore, err := repository.NewSupabaseStore(os.Getenv("BPYP_POSTGRES_CONN"), os.Getenv("BPYP_POSTGRES_DIR_CONN"))
	if err != nil {
		log.Fatalf("Could not create SupabaseStore. Encountered error: %v", err)
	}

	defer supabaseStore.Close()
	workerCount := runtime.NumCPU() - 1
	queue = repository.NewWorkQueue(workerCount, supabaseStore)
	queue.Start()

	router = chi.NewRouter()
	router.Use(middleware.Logger)

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // Replace with your allowed origins
		AllowedMethods:   []string{"GET" /*"POST", "PUT", "DELETE",*/, "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value for preflight request cache
	}))

	router.Route("/v1", func(r chi.Router) {
		r.Get("/health", func(writer http.ResponseWriter, req *http.Request) {
			writer.WriteHeader(http.StatusOK)
			writer.Write([]byte("OK"))
		})
	})
	http.ListenAndServe(":3000", router)
}
