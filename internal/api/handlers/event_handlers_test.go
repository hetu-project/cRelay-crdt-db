package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/hetu-project/cRelay-crdt-db/orbitdb"
	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStore is a mock implementation of the storage interface
type MockStore struct {
	mock.Mock
}

func (m *MockStore) SaveEvent(ctx context.Context, event *nostr.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockStore) QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(chan *nostr.Event), args.Error(1)
}

func (m *MockStore) DeleteEvent(ctx context.Context, event *nostr.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockStore) CountEvents(ctx context.Context, filter nostr.Filter) (int, error) {
	args := m.Called(ctx, filter)
	return args.Int(0), args.Error(1)
}

func (m *MockStore) ReplaceEvent(ctx context.Context, event *nostr.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockStore) GetAllCausalityKeys(ctx context.Context, key string) (map[uint32]uint64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(map[uint32]uint64), args.Error(1)
}

func (m *MockStore) GetCausalityEvents(ctx context.Context, key string) ([]string, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockStore) GetCausalityKey(ctx context.Context, key string, userID uint32) (uint64, error) {
	args := m.Called(ctx, key, userID)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockStore) GetSubspaceCausality(ctx context.Context, key string) (*orbitdb.SubspaceCausality, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(*orbitdb.SubspaceCausality), args.Error(1)
}

func (m *MockStore) GetUserStats(ctx context.Context, userID string) (*orbitdb.UserStats, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*orbitdb.UserStats), args.Error(1)
}

func (m *MockStore) QuerySubspaces(ctx context.Context, filter func(*orbitdb.SubspaceCausality) bool) ([]*orbitdb.SubspaceCausality, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*orbitdb.SubspaceCausality), args.Error(1)
}

func (m *MockStore) QueryUserStats(ctx context.Context, filter func(*orbitdb.UserStats) bool) ([]*orbitdb.UserStats, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*orbitdb.UserStats), args.Error(1)
}

func (m *MockStore) QueryUsersBySubspace(ctx context.Context, subspace string) ([]*orbitdb.UserStats, error) {
	args := m.Called(ctx, subspace)
	return args.Get(0).([]*orbitdb.UserStats), args.Error(1)
}

func (m *MockStore) UpdateFromEvent(ctx context.Context, event *nostr.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// Test timestamp filtering functionality of QueryEvents
func TestQueryEventsWithTimestampFilter(t *testing.T) {
	// Create mock store
	mockStore := new(MockStore)
	handler := NewEventHandlers(mockStore)

	// Create test events
	now := time.Now().Unix()
	event1 := &nostr.Event{
		ID:        "event1",
		CreatedAt: nostr.Timestamp(now - 3600), // 1 hour ago
		Content:   "test event 1",
	}
	event2 := &nostr.Event{
		ID:        "event2",
		CreatedAt: nostr.Timestamp(now), // now
		Content:   "test event 2",
	}

	// Set up mock behavior
	eventChan := make(chan *nostr.Event, 2)
	eventChan <- event1
	eventChan <- event2
	close(eventChan)

	mockStore.On("QueryEvents", mock.Anything, mock.Anything).Return(eventChan, nil)

	// Test cases
	tests := []struct {
		name           string
		queryParams    map[string]interface{}
		expectedCount  int
		expectedEvents []string
	}{
		{
			name: "Query all events",
			queryParams: map[string]interface{}{
				"limit": 10,
			},
			expectedCount:  2,
			expectedEvents: []string{"event1", "event2"},
		},
		{
			name: "Filter with since",
			queryParams: map[string]interface{}{
				"since": float64(now - 1800), // 30 minutes ago
				"limit": 10,
			},
			expectedCount:  1,
			expectedEvents: []string{"event2"},
		},
		{
			name: "Filter with until",
			queryParams: map[string]interface{}{
				"until": float64(now - 1800), // 30 minutes ago
				"limit": 10,
			},
			expectedCount:  1,
			expectedEvents: []string{"event1"},
		},
		{
			name: "Filter with both since and until",
			queryParams: map[string]interface{}{
				"since": float64(now - 7200), // 2 hours ago
				"until": float64(now - 1800), // 30 minutes ago
				"limit": 10,
			},
			expectedCount:  1,
			expectedEvents: []string{"event1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			body, _ := json.Marshal(tt.queryParams)
			req := httptest.NewRequest("POST", "/events/query", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			// Execute request
			handler.QueryEvents(w, req)

			// Verify response
			assert.Equal(t, http.StatusOK, w.Code)

			var events []*nostr.Event
			err := json.NewDecoder(w.Body).Decode(&events)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(events))

			// Verify event IDs
			eventIDs := make([]string, len(events))
			for i, event := range events {
				eventIDs[i] = event.ID
			}
			assert.ElementsMatch(t, tt.expectedEvents, eventIDs)
		})
	}
}

// Test saving events
func TestSaveEvent(t *testing.T) {
	mockStore := new(MockStore)
	handler := NewEventHandlers(mockStore)

	// Create test event
	event := &nostr.Event{
		ID:        "test-event",
		CreatedAt: nostr.Now(),
		Content:   "test content",
	}

	// Set up mock behavior
	mockStore.On("SaveEvent", mock.Anything, mock.Anything).Return(nil)

	// Create request
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	// Execute request
	handler.SaveEvent(w, req)

	// Verify response
	assert.Equal(t, http.StatusCreated, w.Code)
	mockStore.AssertExpectations(t)
}

// Test getting a single event
func TestGetEvent(t *testing.T) {
	mockStore := new(MockStore)
	handler := NewEventHandlers(mockStore)

	// Create test event
	event := &nostr.Event{
		ID:        "test-event",
		CreatedAt: nostr.Now(),
		Content:   "test content",
	}

	// Set up mock behavior
	eventChan := make(chan *nostr.Event, 1)
	eventChan <- event
	close(eventChan)
	mockStore.On("QueryEvents", mock.Anything, mock.MatchedBy(func(filter nostr.Filter) bool {
		return len(filter.IDs) == 1 && filter.IDs[0] == "test-event"
	})).Return(eventChan, nil)

	// Create request
	req := httptest.NewRequest("GET", "/events/test-event", nil)
	w := httptest.NewRecorder()

	// Set up route parameters
	router := mux.NewRouter()
	router.HandleFunc("/events/{id}", handler.GetEvent).Methods("GET")
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	var responseEvent nostr.Event
	err := json.NewDecoder(w.Body).Decode(&responseEvent)
	assert.NoError(t, err)
	assert.Equal(t, event.ID, responseEvent.ID)
	assert.Equal(t, event.Content, responseEvent.Content)
}
