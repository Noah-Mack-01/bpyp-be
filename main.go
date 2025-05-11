package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"noerkrieg.com/server/controller"
	"noerkrieg.com/server/repository"
)

func main() {
	var queue *repository.WorkQueue
	var ctrlr *controller.Controller
	var supabaseStore *repository.SupabaseStore
	var router *chi.Mux

	supabaseStore, err := repository.NewSupabaseStore(os.Getenv("BPYP_POSTGRES_CONN"), os.Getenv("BPYP_POSTGRES_DIR_CONN"))
	if err != nil {
		log.Fatalf("Could not create SupabaseStore. Encountered error: %v", err)
	}

	defer supabaseStore.Close()

	queue = repository.NewWorkQueue(5, supabaseStore)
	queue.Start()

	router = chi.NewRouter()
	router.Use(middleware.Logger)

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // Replace with your allowed origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value for preflight request cache
	}))

	ctrlr = &controller.Controller{SupabaseStore: supabaseStore, WorkQueue: queue}
	router.Route("/v1", func(r chi.Router) {
		r.Get("/job", ctrlr.GetJobStatus)
		r.Post("/job", ctrlr.CreateJob)
		r.Get("/health", ctrlr.GetHealth)
		r.Get("/exercises", ctrlr.GetExercises)
	})
	http.ListenAndServe(":3000", router)
}
