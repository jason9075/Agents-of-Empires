package api

import (
	"net/http"

	"github.com/jason9075/agents_of_dynasties/internal/ticker"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

// Server is the HTTP API server for the game.
type Server struct {
	mux *http.ServeMux
}

// NewServer wires up all routes and returns a ready-to-serve Server.
// webDir is the path to the directory of static files (e.g. "./web").
func NewServer(w *world.World, q *ticker.Queue, webDir string) *Server {
	mux := http.NewServeMux()

	mux.Handle("/map", &mapHandler{w: w})
	mux.Handle("/state", &stateHandler{w: w})
	mux.Handle("/state/full", &fullStateHandler{w: w})
	mux.Handle("/command", &commandHandler{w: w, q: q})

	// Serve static frontend files. Registered last so API routes take priority.
	mux.Handle("/", http.FileServer(http.Dir(webDir)))

	return &Server{mux: mux}
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
