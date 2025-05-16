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

// MockStore 是一个模拟的存储接口实现
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

// 测试 QueryEvents 的时间戳过滤功能
func TestQueryEventsWithTimestampFilter(t *testing.T) {
	// 创建模拟存储
	mockStore := new(MockStore)
	handler := NewEventHandlers(mockStore)

	// 创建测试事件
	now := time.Now().Unix()
	event1 := &nostr.Event{
		ID:        "event1",
		CreatedAt: nostr.Timestamp(now - 3600), // 1小时前
		Content:   "test event 1",
	}
	event2 := &nostr.Event{
		ID:        "event2",
		CreatedAt: nostr.Timestamp(now), // 现在
		Content:   "test event 2",
	}

	// 设置模拟行为
	eventChan := make(chan *nostr.Event, 2)
	eventChan <- event1
	eventChan <- event2
	close(eventChan)

	mockStore.On("QueryEvents", mock.Anything, mock.Anything).Return(eventChan, nil)

	// 测试用例
	tests := []struct {
		name           string
		queryParams    map[string]interface{}
		expectedCount  int
		expectedEvents []string
	}{
		{
			name: "查询所有事件",
			queryParams: map[string]interface{}{
				"limit": 10,
			},
			expectedCount:  2,
			expectedEvents: []string{"event1", "event2"},
		},
		{
			name: "使用 since 过滤",
			queryParams: map[string]interface{}{
				"since": float64(now - 1800), // 30分钟前
				"limit": 10,
			},
			expectedCount:  1,
			expectedEvents: []string{"event2"},
		},
		{
			name: "使用 until 过滤",
			queryParams: map[string]interface{}{
				"until": float64(now - 1800), // 30分钟前
				"limit": 10,
			},
			expectedCount:  1,
			expectedEvents: []string{"event1"},
		},
		{
			name: "使用 since 和 until 过滤",
			queryParams: map[string]interface{}{
				"since": float64(now - 7200), // 2小时前
				"until": float64(now - 1800), // 30分钟前
				"limit": 10,
			},
			expectedCount:  1,
			expectedEvents: []string{"event1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建请求
			body, _ := json.Marshal(tt.queryParams)
			req := httptest.NewRequest("POST", "/events/query", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			// 执行请求
			handler.QueryEvents(w, req)

			// 验证响应
			assert.Equal(t, http.StatusOK, w.Code)

			var events []*nostr.Event
			err := json.NewDecoder(w.Body).Decode(&events)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(events))

			// 验证事件ID
			eventIDs := make([]string, len(events))
			for i, event := range events {
				eventIDs[i] = event.ID
			}
			assert.ElementsMatch(t, tt.expectedEvents, eventIDs)
		})
	}
}

// 测试保存事件
func TestSaveEvent(t *testing.T) {
	mockStore := new(MockStore)
	handler := NewEventHandlers(mockStore)

	// 创建测试事件
	event := &nostr.Event{
		ID:        "test-event",
		CreatedAt: nostr.Now(),
		Content:   "test content",
	}

	// 设置模拟行为
	mockStore.On("SaveEvent", mock.Anything, mock.Anything).Return(nil)

	// 创建请求
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/events", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	// 执行请求
	handler.SaveEvent(w, req)

	// 验证响应
	assert.Equal(t, http.StatusCreated, w.Code)
	mockStore.AssertExpectations(t)
}

// 测试获取单个事件
func TestGetEvent(t *testing.T) {
	mockStore := new(MockStore)
	handler := NewEventHandlers(mockStore)

	// 创建测试事件
	event := &nostr.Event{
		ID:        "test-event",
		CreatedAt: nostr.Now(),
		Content:   "test content",
	}

	// 设置模拟行为
	eventChan := make(chan *nostr.Event, 1)
	eventChan <- event
	close(eventChan)
	mockStore.On("QueryEvents", mock.Anything, mock.MatchedBy(func(filter nostr.Filter) bool {
		return len(filter.IDs) == 1 && filter.IDs[0] == "test-event"
	})).Return(eventChan, nil)

	// 创建请求
	req := httptest.NewRequest("GET", "/events/test-event", nil)
	w := httptest.NewRecorder()

	// 设置路由参数
	router := mux.NewRouter()
	router.HandleFunc("/events/{id}", handler.GetEvent).Methods("GET")
	router.ServeHTTP(w, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)
	var responseEvent nostr.Event
	err := json.NewDecoder(w.Body).Decode(&responseEvent)
	assert.NoError(t, err)
	assert.Equal(t, event.ID, responseEvent.ID)
	assert.Equal(t, event.Content, responseEvent.Content)
}
