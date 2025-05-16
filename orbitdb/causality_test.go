package orbitdb

import (
	"context"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test subspace ID validation
func TestIsValidSubspaceID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{
			name:     "Valid subspace ID",
			id:       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: true,
		},
		{
			name:     "Invalid length",
			id:       "0x123",
			expected: false,
		},
		{
			name:     "Invalid prefix",
			id:       "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: false,
		},
		{
			name:     "Invalid characters",
			id:       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdeg",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidSubspaceID(tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test getting subspace causality
func TestGetSubspaceCausality(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// Create test data
	subspaceID := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	now := time.Now().Unix()
	causalityDoc := map[string]interface{}{
		"_id":         subspaceID,
		"id":          subspaceID,
		"doc_type":    DocTypeCausality,
		"subspace_id": subspaceID,
		"keys": map[string]interface{}{
			"1": float64(5),
			"2": float64(3),
		},
		"events":  []string{"event1", "event2"},
		"created": float64(now - 3600),
		"updated": float64(now),
	}

	// Set mock behavior
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{causalityDoc}, nil)

	// Execute test
	causality, err := manager.GetSubspaceCausality(context.Background(), subspaceID)
	assert.NoError(t, err)
	assert.NotNil(t, causality)
	assert.Equal(t, subspaceID, causality.ID)
	assert.Equal(t, DocTypeCausality, causality.DocType)
	assert.Equal(t, uint64(5), causality.Keys[1])
	assert.Equal(t, uint64(3), causality.Keys[2])
	assert.Equal(t, []string{"event1", "event2"}, causality.Events)
}

// Test updating causality from event
func TestUpdateFromEvent(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// Create test event
	subspaceID := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	event := &nostr.Event{
		ID:        "test-event",
		PubKey:    "test-pubkey",
		CreatedAt: nostr.Now(),
		Kind:      30302, // Assume this is a vote event
		Tags: nostr.Tags{
			{"sid", subspaceID},
			{"op", "vote"},
		},
	}

	// Set mock behavior
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{}, nil)
	mockDB.On("Put", mock.Anything, mock.Anything).Return(subspaceID, nil)

	// Execute test
	err := manager.UpdateFromEvent(context.Background(), event)
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

// Test getting causality events
func TestGetCausalityEvents(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// Create test data
	subspaceID := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	events := []string{"event1", "event2", "event3"}
	causalityDoc := map[string]interface{}{
		"_id":         subspaceID,
		"id":          subspaceID,
		"doc_type":    DocTypeCausality,
		"subspace_id": subspaceID,
		"events":      events,
	}

	// Set mock behavior
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{causalityDoc}, nil)

	// Execute test
	result, err := manager.GetCausalityEvents(context.Background(), subspaceID)
	assert.NoError(t, err)
	assert.Equal(t, events, result)
}

// Test getting causality key
func TestGetCausalityKey(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// Create test data
	subspaceID := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	keyID := uint32(1)
	counter := uint64(5)
	causalityDoc := map[string]interface{}{
		"_id":         subspaceID,
		"id":          subspaceID,
		"doc_type":    DocTypeCausality,
		"subspace_id": subspaceID,
		"keys": map[string]interface{}{
			"1": float64(counter),
		},
	}

	// Set mock behavior
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{causalityDoc}, nil)

	// Execute test
	result, err := manager.GetCausalityKey(context.Background(), subspaceID, keyID)
	assert.NoError(t, err)
	assert.Equal(t, counter, result)
}

// Test getting all causality keys
func TestGetAllCausalityKeys(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// Create test data
	subspaceID := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	keys := map[uint32]uint64{
		1: 5,
		2: 3,
	}
	causalityDoc := map[string]interface{}{
		"_id":         subspaceID,
		"id":          subspaceID,
		"doc_type":    DocTypeCausality,
		"subspace_id": subspaceID,
		"keys": map[string]interface{}{
			"1": float64(5),
			"2": float64(3),
		},
	}

	// Set mock behavior
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{causalityDoc}, nil)

	// Execute test
	result, err := manager.GetAllCausalityKeys(context.Background(), subspaceID)
	assert.NoError(t, err)
	assert.Equal(t, keys, result)
}

// Test querying subspaces
func TestQuerySubspaces(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// Create test data
	subspaceID := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	now := time.Now().Unix()
	causalityDoc := map[string]interface{}{
		"_id":         subspaceID,
		"id":          subspaceID,
		"doc_type":    DocTypeCausality,
		"subspace_id": subspaceID,
		"updated":     float64(now),
	}

	// Set mock behavior
	mockDB.On("Query", mock.Anything, mock.Anything).Return([]interface{}{causalityDoc}, nil)

	// Create filter
	filter := func(c *SubspaceCausality) bool {
		return c.Updated >= now-3600 // Only return subspaces updated in the last hour
	}

	// Execute test
	results, err := manager.QuerySubspaces(context.Background(), filter)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, subspaceID, results[0].ID)
}
