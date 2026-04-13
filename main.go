package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/js-beaulieu/tasks/internal/httpserver"
	"github.com/js-beaulieu/tasks/internal/mcpserver"
)

func main() {
	r := chi.NewRouter()
	r.Mount("/", httpserver.New())
	r.Handle("/mcp", mcpserver.Handler())

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
