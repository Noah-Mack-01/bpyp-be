package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"noerkrieg.com/server/controller"
)

func main() {
	controller := controller.Controller{}
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

	r.Get("/v1/processor", controller.ProcessMessage)
	r.Get("/v1/exercises", controller.GetExercises)

	http.ListenAndServe(":3000", r)
}
