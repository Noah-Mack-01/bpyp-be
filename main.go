package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"noerkrieg.com/server/wit_connection"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	uri := os.Getenv("MONGODB_URI")
	docs := "www.mongodb.com/docs/drivers/go/current/"
	if uri == "" {
		log.Fatal("Set your 'MONGODB_URI' environment variable. " +
			"See: " + docs +
			"usage-examples/#environment-variable")
	}
	client, err := mongo.Connect(options.Client().
		ApplyURI(uri))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/{body}", func(writer http.ResponseWriter, r *http.Request) {
		body := []byte(chi.URLParam(r, "body"))
		result, err := wit_connection.ProcessMessage(&body)
		if err != nil {
			writer.WriteHeader(422)
			writer.Write([]byte(fmt.Sprintf("Could not issue message to NLP with message %v: %v", body, err)))
			return
		} else {
			writer.WriteHeader(200)
			writer.Write(result)
		}
	})
	http.ListenAndServe(":3000", r)
}
