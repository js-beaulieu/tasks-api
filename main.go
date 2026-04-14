package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/js-beaulieu/tasks/internal/httpserver"
	"github.com/js-beaulieu/tasks/internal/mcpserver"
	"github.com/js-beaulieu/tasks/internal/store/sqlite"
)

func main() {
	db, err := sqlite.Open("tasks.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	s := sqlite.New(db)

	r := chi.NewRouter()
	r.Mount("/", httpserver.New(s))
	r.Handle("/mcp", mcpserver.Handler(s))

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
