package handlers

import (
	// "context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/nbd-wtf/go-nostr"

	"github.com/hetu-project/cRelay-crdt-db/internal/storage"
	"github.com/hetu-project/cRelay-crdt-db/orbitdb"
)

// CausalityHandlers handles causality-related API requests
type CausalityHandlers struct {
	store storage.Store
}

// NewCausalityHandlers creates a new CausalityHandlers
func NewCausalityHandlers(store storage.Store) *CausalityHandlers {
	return &CausalityHandlers{
		store: store,
	}
}

// GetSubspaceCausality handles getting subspace causality requests
func (h *CausalityHandlers) GetSubspaceCausality(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subspaceID := vars["id"]

	// Get subspace causality
	causality, err := h.store.GetSubspaceCausality(r.Context(), subspaceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get subspace causality: %v", err), http.StatusInternalServerError)
		return
	}

	if causality == nil {
		http.Error(w, "Subspace does not exist", http.StatusNotFound)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(causality)
}

// GetCausalityKey handles getting specific causality key requests
func (h *CausalityHandlers) GetCausalityKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subspaceID := vars["id"]
	keyIDStr := vars["key"]

	// Convert key ID to uint32
	keyID, err := strconv.ParseUint(keyIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	// Get causality key counter value
	counter, err := h.store.GetCausalityKey(r.Context(), subspaceID, uint32(keyID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get causality key: %v", err), http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"subspace_id": subspaceID,
		"key":         keyID,
		"counter":     counter,
	})
}

// GetSubspaceEvents handles getting subspace events requests
func (h *CausalityHandlers) GetSubspaceEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subspaceID := vars["id"]

	// Get subspace event ID list
	eventIDs, err := h.store.GetCausalityEvents(r.Context(), subspaceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get subspace events: %v", err), http.StatusInternalServerError)
		return
	}

	if len(eventIDs) == 0 {
		// Return empty array
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	// Create filter to query these events
	filter := nostr.Filter{
		IDs: eventIDs,
	}

	// Limit returned events
	limit := 100 // Default limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Query events
	events := make([]*nostr.Event, 0)
	eventChan, err := h.store.QueryEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query events: %v", err), http.StatusInternalServerError)
		return
	}

	// Collect events
	count := 0
	for event := range eventChan {
		if count >= limit {
			break
		}
		events = append(events, event)
		count++
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// ListSubspaces handles listing all subspaces requests
func (h *CausalityHandlers) ListSubspaces(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	query := r.URL.Query()
	sinceStr := query.Get("since")
	untilStr := query.Get("until")

	// Parse time range
	var since, until *int64
	if sinceStr != "" {
		sinceVal, err := strconv.ParseInt(sinceStr, 10, 64)
		if err == nil {
			since = &sinceVal
		}
	}

	if untilStr != "" {
		untilVal, err := strconv.ParseInt(untilStr, 10, 64)
		if err == nil {
			until = &untilVal
		}
	}

	// Create filter function
	filter := func(c *orbitdb.SubspaceCausality) bool {
		if since != nil && c.Updated < *since {
			return false
		}
		if until != nil && c.Updated > *until {
			return false
		}
		return true
	}

	// Query subspaces
	subspaces, err := h.store.QuerySubspaces(r.Context(), filter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query subspaces: %v", err), http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subspaces)
}

// CreateSubspaceEvent handles creating a subspace event
// func (h *CausalityHandlers) CreateSubspaceEvent(w http.ResponseWriter, r *http.Request) {
// 	// Parse request body
// 	var requestData struct {
// 		SubspaceID string `json:"subspace_id"`
// 		PubKey     string `json:"pubkey"`
// 		KeyID      uint32 `json:"key_id"`
// 		Content    string `json:"content,omitempty"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
// 		http.Error(w, "Invalid request body", http.StatusBadRequest)
// 		return
// 	}

// 	// Validate subspace ID
// 	if !orbitdb.IsValidSubspaceID(requestData.SubspaceID) {
// 		http.Error(w, "Invalid subspace ID format", http.StatusBadRequest)
// 		return
// 	}

// 	// Get current counter value
// 	counter, err := h.store.GetCausalityKey(r.Context(), requestData.SubspaceID, requestData.KeyID)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to get causality key: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// Increment counter
// 	newCounter := counter + 1

// 	// Create event
// 	event := &nostr.Event{
// 		PubKey:    requestData.PubKey,
// 		CreatedAt: nostr.Now(),
// 		Kind:      int(requestData.KeyID), // Use KeyID as event Kind
// 		Tags: []nostr.Tag{
// 			{"d", "subspace_op"},
// 			{"sid", requestData.SubspaceID},
// 			{"causal", fmt.Sprintf("%d", requestData.KeyID), fmt.Sprintf("%d", newCounter)},
// 		},
// 		Content: requestData.Content,
// 	}

// 	// Calculate event ID
// 	err = event.Sign() // Note: In actual use, this should be signed by the client
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to sign event: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// Save event
// 	err = h.store.SaveEvent(r.Context(), event)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to save event: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// Return JSON response
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusCreated)
// 	json.NewEncoder(w).Encode(event)
// }
