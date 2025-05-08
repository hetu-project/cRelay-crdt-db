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
	causalityHandlers := handlers.NewCausalityHandlers(r.store)
	userHandlers := handlers.NewUserHandlers(r.store)

	// Event API endpoints
	router.HandleFunc("/events", eventHandlers.SaveEvent).Methods(http.MethodPost)
	router.HandleFunc("/events/{id}", eventHandlers.GetEvent).Methods(http.MethodGet)
	router.HandleFunc("/events/query", eventHandlers.QueryEvents).Methods(http.MethodPost)
	router.HandleFunc("/events/{id}", eventHandlers.DeleteEvent).Methods(http.MethodDelete)

	// 子空间信息端点
	// router.HandleFunc("/subspace/{id}", eventHandlers.GetSubspace).Methods("GET")

	// 子空间事件端点
	// router.HandleFunc("/subspace/{id}/events", eventHandlers.GetSubspaceEvents).Methods("GET")

	// 列出所有子空间
	// router.HandleFunc("/subspaces", eventHandlers.ListSubspaces).Methods("GET")

	// Causality API endpoints
	router.HandleFunc("/subspaces", causalityHandlers.ListSubspaces).Methods(http.MethodGet)
	router.HandleFunc("/subspaces/{id}", causalityHandlers.GetSubspaceCausality).Methods(http.MethodGet)
	router.HandleFunc("/subspaces/{id}/events", causalityHandlers.GetSubspaceEvents).Methods(http.MethodGet)
	router.HandleFunc("/subspaces/{id}/keys/{key}", causalityHandlers.GetCausalityKey).Methods(http.MethodGet)
	//router.HandleFunc("/subspaces/events", causalityHandlers.CreateSubspaceEvent).Methods(http.MethodPost)

	// User Stats API endpoints
	router.HandleFunc("/users/{id}/stats", userHandlers.GetUserStats).Methods(http.MethodGet)
	router.HandleFunc("/users/{id}/subspaces", userHandlers.GetUserSubspaces).Methods(http.MethodGet)
	router.HandleFunc("/users/{id}/invites", userHandlers.GetUserInvites).Methods(http.MethodGet)
	router.HandleFunc("/users/top", userHandlers.ListTopUsers).Methods(http.MethodGet)
	router.HandleFunc("/subspaces/{id}/users", userHandlers.GetSubspaceUsers).Methods(http.MethodGet)

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
