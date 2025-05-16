package orbitdb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"berty.tech/go-orbit-db/iface"
	"github.com/nbd-wtf/go-nostr"
)

// DocumentType is used to distinguish between different types of documents
const (
	DocTypeNostrEvent = "nostr_event"
	DocTypeCausality  = "causality"
)

// CausalityKey represents a causality key
type CausalityKey struct {
	Key     uint32 `json:"key"`     // Causality key identifier
	Counter uint64 `json:"counter"` // Lamport clock counter
}

// SubspaceCausality represents causality data for a subspace
type SubspaceCausality struct {
	ID         string            `json:"id"`          // Subspace ID, format: 0x-prefixed 64-bit hex string
	DocType    string            `json:"doc_type"`    // Document type, here it's "causality"
	SubspaceID string            `json:"subspace_id"` // Alternative representation of subspace ID (if needed)
	Keys       map[uint32]uint64 `json:"keys"`        // Keys are causality key IDs, values are counters
	Events     []string          `json:"events"`      // List of associated event IDs
	Created    int64             `json:"created"`     // Creation timestamp
	Updated    int64             `json:"updated"`     // Update timestamp
}

// CausalityManager manages causality relationships
type CausalityManager struct {
	db iface.DocumentStore
}

// NewCausalityManager creates a new causality manager
func NewCausalityManager(db iface.DocumentStore) *CausalityManager {
	return &CausalityManager{
		db: db,
	}
}

// GetSubspaceCausality retrieves causality data for a subspace
func (cm *CausalityManager) GetSubspaceCausality(ctx context.Context, subspaceID string) (*SubspaceCausality, error) {
	if !IsValidSubspaceID(subspaceID) {
		return nil, fmt.Errorf("invalid subspace ID format: %s", subspaceID)
	}

	// Query subspace data
	docs, err := cm.db.Get(ctx, subspaceID, nil)
	if err != nil {
		return nil, err
	}

	// If it doesn't exist, return nil
	if len(docs) == 0 {
		return nil, nil
	}

	// Iterate through results to find causality type documents
	var causalityDoc map[string]interface{}
	for _, doc := range docs {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			continue
		}

		docType, ok := docMap["doc_type"].(string)
		if !ok || docType != DocTypeCausality {
			continue
		}

		causalityDoc = docMap
		break
	}

	if causalityDoc == nil {
		return nil, nil
	}

	// Convert document to JSON and parse it into struct
	jsonData, err := json.Marshal(causalityDoc)
	if err != nil {
		return nil, err
	}

	var causality SubspaceCausality
	if err := json.Unmarshal(jsonData, &causality); err != nil {
		return nil, err
	}

	return &causality, nil
}

// parseOpsTag parses ops tag, extracts operation and corresponding causality key
func parseOpsTag(opsValue string) map[string]uint32 {
	result := make(map[string]uint32)

	pairs := strings.Split(opsValue, ",")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			continue
		}

		key := kv[0]
		valueStr := kv[1]

		value, err := strconv.ParseUint(valueStr, 10, 32)
		if err != nil {
			log.Printf("Cannot parse causality key value: %s", valueStr)
			continue
		}

		result[key] = uint32(value)
	}

	return result
}

// UpdateFromEvent updates causality relationships from an event
func (cm *CausalityManager) UpdateFromEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Find subspace ID tag in the event
	var subspaceID string
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "sid" {
			subspaceID = tag[1]
			break
		}
	}

	if subspaceID == "" {
		// No subspace tag, no causality relationship to handle
		return nil
	}

	// Verify subspace ID format
	if !IsValidSubspaceID(subspaceID) {
		log.Printf("Warning: Event %s contains invalid subspace ID format: %s", event.ID, subspaceID)
		return nil
	}

	// Get existing subspace causality
	causality, err := cm.GetSubspaceCausality(ctx, subspaceID)
	if err != nil {
		return err
	}

	now := nostr.Now()

	// If it doesn't exist, create new
	if causality == nil {
		causality = &SubspaceCausality{
			ID:         subspaceID,
			DocType:    DocTypeCausality,
			SubspaceID: subspaceID,
			Keys:       make(map[uint32]uint64),
			Events:     []string{event.ID},
			Created:    int64(now),
			Updated:    int64(now),
		}
	} else {
		// Update event list to avoid duplicates
		eventExists := false
		for _, id := range causality.Events {
			if id == event.ID {
				eventExists = true
				break
			}
		}

		if !eventExists {
			causality.Events = append(causality.Events, event.ID)
		}

		causality.Updated = int64(now)
	}

	// Handle special event types
	if event.Kind == 30100 {
		// This is subspace creation event, need to initialize all causality key counters
		var opsValue string
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == "ops" {
				opsValue = tag[1]
				break
			}
		}

		if opsValue != "" {
			// Parse ops tag
			ops := parseOpsTag(opsValue)
			for _, keyID := range ops {
				// Initialize each causality key counter to 0
				causality.Keys[keyID] = 0
			}

			log.Printf("Initialized causality keys for subspace %s: %v", subspaceID, causality.Keys)
		}
	} else {
		// For other types of events, find corresponding causality key and update counter
		var opName string
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == "op" {
				opName = tag[1]
				break
			}
		}

		// Find operation corresponding causality key and update counter
		if opName != "" {
			// For specific kind values, directly use its value as causality key
			// For example: kind 30302 corresponds to vote operation
			var keyID uint32
			foundKey := false

			// Try to use kind value as causality key first
			keyID = uint32(event.Kind)
			if _, exists := causality.Keys[keyID]; exists {
				foundKey = true
				causality.Keys[keyID]++
				log.Printf("Updated causality key %d counter for subspace %s to %d", keyID, subspaceID, causality.Keys[keyID])
			} else {
				// If no direct match, try to match through tag
				for key, counter := range causality.Keys {
					// Here we need a mapping table to map operation names to corresponding causality keys
					// But since we don't have this mapping, this is just example code
					keyIDStr := fmt.Sprintf("%d", key)
					if strings.HasSuffix(keyIDStr, opName) {
						causality.Keys[key] = counter + 1
						log.Printf("Updated causality key %d counter for subspace %s to %d", key, subspaceID, causality.Keys[key])
						foundKey = true
						break
					}
				}
			}

			if !foundKey {
				log.Printf("Warning: Cannot find corresponding causality key for operation %s", opName)
			}
		}
	}

	// Save updated causality
	doc := map[string]interface{}{
		"_id":         causality.ID,
		"id":          causality.ID,
		"doc_type":    DocTypeCausality,
		"subspace_id": causality.SubspaceID,
		"keys":        causality.Keys,
		"events":      causality.Events,
		"created":     causality.Created,
		"updated":     causality.Updated,
	}

	_, err = cm.db.Put(ctx, doc)
	return err
}

// IsValidSubspaceID checks if subspace ID is valid
func IsValidSubspaceID(sid string) bool {
	if len(sid) != 66 { // 0x + 64 hex chars
		return false
	}

	if !strings.HasPrefix(sid, "0x") {
		return false
	}

	for _, c := range sid[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}

	return true
}

// GetCausalityEvents retrieves all events related to a specific subspace
func (cm *CausalityManager) GetCausalityEvents(ctx context.Context, subspaceID string) ([]string, error) {
	causality, err := cm.GetSubspaceCausality(ctx, subspaceID)
	if err != nil {
		return nil, err
	}

	if causality == nil {
		return []string{}, nil
	}

	return causality.Events, nil
}

// GetCausalityKey retrieves a specific causality key for a specific subspace
func (cm *CausalityManager) GetCausalityKey(ctx context.Context, subspaceID string, keyID uint32) (uint64, error) {
	causality, err := cm.GetSubspaceCausality(ctx, subspaceID)
	if err != nil {
		return 0, err
	}

	if causality == nil {
		return 0, fmt.Errorf("subspace %s does not exist", subspaceID)
	}

	counter, exists := causality.Keys[keyID]
	if !exists {
		return 0, nil // Return 0 indicates key does not exist
	}

	return counter, nil
}

// GetAllCausalityKeys retrieves all causality keys for a specific subspace
func (cm *CausalityManager) GetAllCausalityKeys(ctx context.Context, subspaceID string) (map[uint32]uint64, error) {
	causality, err := cm.GetSubspaceCausality(ctx, subspaceID)
	if err != nil {
		return nil, err
	}

	if causality == nil {
		return make(map[uint32]uint64), nil
	}

	return causality.Keys, nil
}

// QuerySubspaces queries subspaces based on conditions
func (cm *CausalityManager) QuerySubspaces(ctx context.Context, filter func(*SubspaceCausality) bool) ([]*SubspaceCausality, error) {
	var results []*SubspaceCausality

	queryFn := func(doc interface{}) (bool, error) {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		// Check if it's causality type
		docType, ok := docMap["doc_type"].(string)
		if !ok || docType != DocTypeCausality {
			return false, nil
		}

		// Convert document to JSON
		jsonData, err := json.Marshal(docMap)
		if err != nil {
			return false, nil
		}

		var causality SubspaceCausality
		if err := json.Unmarshal(jsonData, &causality); err != nil {
			return false, nil
		}

		// Apply filter
		if filter == nil || filter(&causality) {
			results = append(results, &causality)
		}

		return true, nil
	}

	// Execute query
	cm.db.Query(ctx, queryFn)

	return results, nil
}
