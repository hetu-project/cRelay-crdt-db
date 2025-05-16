package orbitdb

import (
	"context"
	"fmt"
	"log"
	"strings"

	"berty.tech/go-orbit-db/iface"
	"github.com/nbd-wtf/go-nostr"
)

// OrbitDBAdapter implements the eventstore.Store interface
type OrbitDBAdapter struct {
	db           iface.DocumentStore
	causalityMgr *CausalityManager
	userStatsMgr *UserStatsManager
}

// NewOrbitDBAdapter creates a new OrbitDB adapter
func NewOrbitDBAdapter(db iface.DocumentStore) *OrbitDBAdapter {
	return &OrbitDBAdapter{
		db:           db,
		causalityMgr: NewCausalityManager(db), // Use the same database instance
		userStatsMgr: NewUserStatsManager(db), // Use the same database instance
	}
}

// SaveEvent saves an event to OrbitDB
// Updated signature to match func(ctx context.Context, event *nostr.Event) error
func (a *OrbitDBAdapter) SaveEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Convert event to document
	doc := map[string]interface{}{
		"_id":        event.ID,
		"pubkey":     event.PubKey,
		"created_at": event.CreatedAt,
		"kind":       event.Kind,
		"content":    event.Content,
		"tags":       event.Tags,
		"sig":        event.Sig,
		"doc_type":   DocTypeNostrEvent, // Add document type identifier
	}

	// Save to database
	_, err := a.db.Put(ctx, doc)
	if err != nil {
		return err
	}

	// Update causality
	if updateErr := a.causalityMgr.UpdateFromEvent(ctx, event); updateErr != nil {
		// Try to update causality, but don't affect event storage
		log.Printf("Warning: Failed to update causality: %v", updateErr)
	}

	// Update user statistics
	if updateErr := a.userStatsMgr.UpdateUserStatsFromEvent(ctx, event); updateErr != nil {
		// Try to update user statistics, but don't affect event storage
		log.Printf("Warning: Failed to update user statistics: %v", updateErr)
	}

	return nil
}

func (a *OrbitDBAdapter) QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	// Create event channel
	eventChan := make(chan *nostr.Event)

	go func() {
		defer close(eventChan)

		// Define query function
		queryFn := func(doc interface{}) (bool, error) {
			event, ok := doc.(map[string]interface{})
			if !ok {
				return false, nil
			}

			// Only process documents of type nostr event
			docType, ok := event["doc_type"].(string)
			if !ok || docType != DocTypeNostrEvent {
				return false, nil
			}

			// Implement filtering logic
			// Note: here it's _id instead of id
			if len(filter.IDs) > 0 {
				id, ok := event["_id"].(string)
				if !ok || !contains(filter.IDs, id) {
					return false, nil
				}
			}

			if len(filter.Authors) > 0 {
				pubkey, ok := event["pubkey"].(string)
				if !ok || !contains(filter.Authors, pubkey) {
					return false, nil
				}
			}

			if len(filter.Kinds) > 0 {
				kind, ok := event["kind"].(float64)
				if !ok || !containsInt(filter.Kinds, int(kind)) {
					return false, nil
				}
			}

			// Filter #sid tag
			// Check tag filtering conditions
			if len(filter.Tags) > 0 {
				tags, ok := event["tags"].([]interface{})
				if !ok {
					return false, nil
				}

				// Check each tag filtering condition
				for tagName, tagValues := range filter.Tags {
					if len(tagValues) == 0 {
						continue
					}

					// Find matching tag in the event
					found := false
					for _, tag := range tags {
						tagArray, ok := tag.([]interface{})
						if !ok || len(tagArray) < 2 {
							continue
						}

						name, ok := tagArray[0].(string)
						if !ok || !strings.EqualFold(name, tagName) {
							continue
						}

						value, ok := tagArray[1].(string)
						if !ok {
							continue
						}

						// Check if tag value is in the filtering conditions
						if contains(tagValues, value) {
							found = true
							break
						}
					}

					// If no matching tag is found, skip this event
					if !found {
						return false, nil
					}
				}
			}
			return true, nil
		}

		// Execute query
		docs, _ := a.db.Query(ctx, queryFn)
		for _, doc := range docs {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
				// Continue processing
			}

			// Directly build event object, not via JSON serialization/deserialization
			docMap, ok := doc.(map[string]interface{})
			if !ok {
				log.Printf("无效的文档格式")
				continue
			}

			event := &nostr.Event{}

			// Set basic fields
			if id, ok := docMap["_id"].(string); ok {
				event.ID = id
			}
			if pubkey, ok := docMap["pubkey"].(string); ok {
				event.PubKey = pubkey
			}
			if createdAt, ok := docMap["created_at"].(float64); ok {
				event.CreatedAt = nostr.Timestamp(createdAt)
			}
			if kind, ok := docMap["kind"].(float64); ok {
				event.Kind = int(kind)
			}
			if content, ok := docMap["content"].(string); ok {
				event.Content = content
			}
			if sig, ok := docMap["sig"].(string); ok {
				event.Sig = sig
			}

			// Process tags
			if tagsData, ok := docMap["tags"].([]interface{}); ok {
				for _, tagData := range tagsData {
					if tagArray, ok := tagData.([]interface{}); ok {
						var tag nostr.Tag
						for _, item := range tagArray {
							if str, ok := item.(string); ok {
								tag = append(tag, str)
							}
						}
						event.Tags = append(event.Tags, tag)
					}
				}
			}

			// Send event to channel
			select {
			case <-ctx.Done():
				return
			case eventChan <- event:
				// Event has been sent
			}
		}
	}()

	return eventChan, nil
}

// DeleteEvent deletes an event from the database
// Updated signature to match func(ctx context.Context, event *nostr.Event) error
func (a *OrbitDBAdapter) DeleteEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	_, err := a.db.Delete(ctx, event.ID)
	return err
}

// CountEvents implements counting method to match Counter interface
func (a *OrbitDBAdapter) CountEvents(ctx context.Context, filter nostr.Filter) (int, error) {
	count := 0

	queryFn := func(doc interface{}) (bool, error) {
		event, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		// Implement the same filtering logic as QueryEvents
		if len(filter.IDs) > 0 {
			id, ok := event["_id"].(string)
			if !ok || !contains(filter.IDs, id) {
				return false, nil
			}
		}

		if len(filter.Authors) > 0 {
			pubkey, ok := event["pubkey"].(string)
			if !ok || !contains(filter.Authors, pubkey) {
				return false, nil
			}
		}

		if len(filter.Kinds) > 0 {
			kind, ok := event["kind"].(float64)
			if !ok || !containsInt(filter.Kinds, int(kind)) {
				return false, nil
			}
		}

		count++
		return true, nil
	}

	// Execute query count
	a.db.Query(ctx, queryFn)

	return count, nil
}

// ReplaceEvent replaces an event in the database
func (a *OrbitDBAdapter) ReplaceEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	doc := map[string]interface{}{
		"_id":        event.ID,
		"pubkey":     event.PubKey,
		"created_at": event.CreatedAt,
		"kind":       event.Kind,
		"content":    event.Content,
		"sig":        event.Sig,
		"tags":       event.Tags,
		"doc_type":   DocTypeNostrEvent, // Add document type identifier
	}

	_, err := a.db.Put(ctx, doc)

	if err != nil {
		return err
	}

	// Update causality
	if a.causalityMgr != nil {
		// Try to update causality, but don't affect event storage
		if updateErr := a.causalityMgr.UpdateFromEvent(ctx, event); updateErr != nil {
			log.Printf("Warning: Failed to update causality: %v", updateErr)
		}
	}

	// Update user statistics
	if a.userStatsMgr != nil {
		// Try to update user statistics, but don't affect event storage
		if updateErr := a.userStatsMgr.UpdateUserStatsFromEvent(ctx, event); updateErr != nil {
			log.Printf("Warning: Failed to update user statistics: %v", updateErr)
		}
	}

	return nil
}

// Helper function: check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetSubspaceCausality retrieves causality data for a subspace
func (a *OrbitDBAdapter) GetSubspaceCausality(ctx context.Context, subspaceID string) (*SubspaceCausality, error) {
	return a.causalityMgr.GetSubspaceCausality(ctx, subspaceID)
}

// QuerySubspaces queries subspaces based on conditions
func (a *OrbitDBAdapter) QuerySubspaces(ctx context.Context, filter func(*SubspaceCausality) bool) ([]*SubspaceCausality, error) {
	return a.causalityMgr.QuerySubspaces(ctx, filter)
}

// UpdateFromEvent updates causality relationships from an event
func (a *OrbitDBAdapter) UpdateFromEvent(ctx context.Context, event *nostr.Event) error {
	return a.causalityMgr.UpdateFromEvent(ctx, event)
}

// GetCausalityEvents retrieves all events related to a specific subspace
func (a *OrbitDBAdapter) GetCausalityEvents(ctx context.Context, subspaceID string) ([]string, error) {
	return a.causalityMgr.GetCausalityEvents(ctx, subspaceID)
}

// GetCausalityKey retrieves a specific causality key for a specific subspace
func (a *OrbitDBAdapter) GetCausalityKey(ctx context.Context, subspaceID string, keyID uint32) (uint64, error) {
	return a.causalityMgr.GetCausalityKey(ctx, subspaceID, keyID)
}

// GetAllCausalityKeys retrieves all causality keys for a specific subspace
func (a *OrbitDBAdapter) GetAllCausalityKeys(ctx context.Context, subspaceID string) (map[uint32]uint64, error) {
	return a.causalityMgr.GetAllCausalityKeys(ctx, subspaceID)
}

// GetUserStats retrieves user statistics
func (a *OrbitDBAdapter) GetUserStats(ctx context.Context, userID string) (*UserStats, error) {
	return a.userStatsMgr.GetUserStats(ctx, userID)
}

// QueryUsersBySubspace queries all users in a specific subspace
func (a *OrbitDBAdapter) QueryUsersBySubspace(ctx context.Context, subspaceID string) ([]*UserStats, error) {
	return a.userStatsMgr.QueryUsersBySubspace(ctx, subspaceID)
}

// QueryUserStats queries user statistics based on conditions
func (a *OrbitDBAdapter) QueryUserStats(ctx context.Context, filter func(*UserStats) bool) ([]*UserStats, error) {
	return a.userStatsMgr.QueryUserStats(ctx, filter)
}

// Helper function: check if a slice contains an integer
func containsInt(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
