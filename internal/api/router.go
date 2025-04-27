package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	//"github.com/hetu-project/hetu-orbitdb/internal/api/handlers"
	"github.com/hetu-project/cRelay-crdt-db/internal/api/handlers"
	"github.com/hetu-project/cRelay-crdt-db/internal/storage"
)

// Router handles HTTP routing
type Router struct {
	store storage.Store
}

// NewRouter creates a new router
func NewRouter(store storage.Store) *Router {
	return &Router{
		store: store,
	}
}

// Handler returns the configured HTTP handler
func (r *Router) Handler() http.Handler {
	router := mux.NewRouter()

	// Create event handlers
	eventHandlers := handlers.NewEventHandlers(r.store)

	// Event API endpoints
	router.HandleFunc("/events", eventHandlers.SaveEvent).Methods(http.MethodPost)
	router.HandleFunc("/events/{id}", eventHandlers.GetEvent).Methods(http.MethodGet)
	router.HandleFunc("/events/query", eventHandlers.QueryEvents).Methods(http.MethodPost)
	router.HandleFunc("/events/{id}", eventHandlers.DeleteEvent).Methods(http.MethodDelete)

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods(http.MethodGet)

	// CORS configuration
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
	})

	return c.Handler(router)
}
