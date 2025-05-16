package orbitdb

import (
	"context"
	"testing"
	"time"

	ipfslog "berty.tech/go-ipfs-log"
	"berty.tech/go-ipfs-log/identityprovider"
	ipfsiface "berty.tech/go-ipfs-log/iface"
	"berty.tech/go-orbit-db/accesscontroller"
	"berty.tech/go-orbit-db/address"
	"berty.tech/go-orbit-db/events"
	"berty.tech/go-orbit-db/iface"
	"berty.tech/go-orbit-db/stores/operation"
	"berty.tech/go-orbit-db/stores/replicator"
	"github.com/ipfs/go-datastore"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// MockDocumentStore 是一个模拟的 DocumentStore 接口实现
type MockDocumentStore struct {
	mock.Mock
}

func (m *MockDocumentStore) Put(ctx context.Context, doc interface{}) (operation.Operation, error) {
	args := m.Called(ctx, doc)
	return args.Get(0).(operation.Operation), args.Error(1)
}

func (m *MockDocumentStore) Get(ctx context.Context, key string, opts *iface.DocumentStoreGetOptions) ([]interface{}, error) {
	args := m.Called(ctx, key, opts)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockDocumentStore) Delete(ctx context.Context, key string) (operation.Operation, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(operation.Operation), args.Error(1)
}

func (m *MockDocumentStore) Query(ctx context.Context, queryFn func(doc interface{}) (bool, error)) ([]interface{}, error) {
	args := m.Called(ctx, queryFn)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockDocumentStore) AccessController() accesscontroller.Interface {
	args := m.Called()
	return args.Get(0).(accesscontroller.Interface)
}

func (m *MockDocumentStore) AddOperation(ctx context.Context, op operation.Operation, c chan<- ipfslog.Entry) (ipfslog.Entry, error) {
	args := m.Called(ctx, op, c)
	return args.Get(0).(ipfslog.Entry), args.Error(1)
}

func (m *MockDocumentStore) Address() address.Address {
	args := m.Called()
	return args.Get(0).(address.Address)
}

func (m *MockDocumentStore) Cache() datastore.Datastore {
	args := m.Called()
	return args.Get(0).(datastore.Datastore)
}

func (m *MockDocumentStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDocumentStore) DBName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockDocumentStore) Drop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDocumentStore) Emit(ctx context.Context, event events.Event) {
	m.Called(ctx, event)
}

func (m *MockDocumentStore) EventBus() event.Bus {
	args := m.Called()
	return args.Get(0).(event.Bus)
}

func (m *MockDocumentStore) GlobalChannel(ctx context.Context) <-chan events.Event {
	args := m.Called(ctx)
	return args.Get(0).(<-chan events.Event)
}

func (m *MockDocumentStore) IO() ipfsiface.IO {
	args := m.Called()
	return args.Get(0).(ipfsiface.IO)
}

func (m *MockDocumentStore) IPFS() coreiface.CoreAPI {
	args := m.Called()
	return args.Get(0).(coreiface.CoreAPI)
}

func (m *MockDocumentStore) Identity() *identityprovider.Identity {
	args := m.Called()
	return args.Get(0).(*identityprovider.Identity)
}
func (m *MockDocumentStore) Index() iface.StoreIndex {
	args := m.Called()
	return args.Get(0).(iface.StoreIndex)
}

func (m *MockDocumentStore) Load(ctx context.Context, amount int) error {
	args := m.Called(ctx, amount)
	return args.Error(0)
}

func (m *MockDocumentStore) LoadFromSnapshot(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDocumentStore) LoadMoreFrom(ctx context.Context, amount uint, entries []ipfslog.Entry) {
	m.Called(ctx, amount, entries)
}

func (m *MockDocumentStore) Logger() *zap.Logger {
	args := m.Called()
	return args.Get(0).(*zap.Logger)
}

func (m *MockDocumentStore) OpLog() ipfslog.Log {
	args := m.Called()
	return args.Get(0).(ipfslog.Log)
}

func (m *MockDocumentStore) PutAll(ctx context.Context, docs []interface{}) (operation.Operation, error) {
	args := m.Called(ctx, docs)
	return args.Get(0).(operation.Operation), args.Error(1)
}

func (m *MockDocumentStore) PutBatch(ctx context.Context, docs []interface{}) (operation.Operation, error) {
	args := m.Called(ctx, docs)
	return args.Get(0).(operation.Operation), args.Error(1)
}

func (m *MockDocumentStore) ReplicationStatus() replicator.ReplicationInfo {
	args := m.Called()
	return args.Get(0).(replicator.ReplicationInfo)
}

func (m *MockDocumentStore) Replicator() replicator.Replicator {
	args := m.Called()
	return args.Get(0).(replicator.Replicator)
}

func (m *MockDocumentStore) Subscribe(ctx context.Context) <-chan events.Event {
	args := m.Called(ctx)
	return args.Get(0).(<-chan events.Event)
}

func (m *MockDocumentStore) Sync(ctx context.Context, entries []ipfslog.Entry) error {
	args := m.Called(ctx, entries)
	return args.Error(0)
}

func (m *MockDocumentStore) Tracer() trace.Tracer {
	args := m.Called()
	return args.Get(0).(trace.Tracer)
}

func (m *MockDocumentStore) Type() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockDocumentStore) UnsubscribeAll() {
	m.Called()
}

// 测试时间戳过滤功能
func TestQueryEventsWithTimestampFilter(t *testing.T) {
	// 创建模拟存储
	mockDB := new(MockDocumentStore)
	adapter := NewOrbitDBAdapter(mockDB)

	// 创建测试事件
	now := time.Now().Unix()
	event1 := map[string]interface{}{
		"_id":        "event1",
		"created_at": float64(now - 3600), // 1小时前
		"content":    "test event 1",
		"doc_type":   DocTypeNostrEvent,
	}
	event2 := map[string]interface{}{
		"_id":        "event2",
		"created_at": float64(now), // 现在
		"content":    "test event 2",
		"doc_type":   DocTypeNostrEvent,
	}

	// 设置模拟行为
	mockDB.On("Query", mock.Anything, mock.Anything).Return([]interface{}{event1, event2}, nil)

	// 测试用例
	tests := []struct {
		name           string
		filter         nostr.Filter
		expectedCount  int
		expectedEvents []string
	}{
		{
			name: "查询所有事件",
			filter: nostr.Filter{
				Limit: 10,
			},
			expectedCount:  2,
			expectedEvents: []string{"event1", "event2"},
		},
		{
			name: "使用 since 过滤",
			filter: nostr.Filter{
				Since: func() *nostr.Timestamp {
					t := nostr.Timestamp(now - 1800) // 30分钟前
					return &t
				}(),
				Limit: 10,
			},
			expectedCount:  1,
			expectedEvents: []string{"event2"},
		},
		{
			name: "使用 until 过滤",
			filter: nostr.Filter{
				Until: func() *nostr.Timestamp {
					t := nostr.Timestamp(now - 1800) // 30分钟前
					return &t
				}(),
				Limit: 10,
			},
			expectedCount:  1,
			expectedEvents: []string{"event1"},
		},
		{
			name: "使用 since 和 until 过滤",
			filter: nostr.Filter{
				Since: func() *nostr.Timestamp {
					t := nostr.Timestamp(now - 7200) // 2小时前
					return &t
				}(),
				Until: func() *nostr.Timestamp {
					t := nostr.Timestamp(now - 1800) // 30分钟前
					return &t
				}(),
				Limit: 10,
			},
			expectedCount:  1,
			expectedEvents: []string{"event1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行查询
			eventChan, err := adapter.QueryEvents(context.Background(), tt.filter)
			assert.NoError(t, err)

			// 收集事件
			var events []*nostr.Event
			for event := range eventChan {
				events = append(events, event)
			}

			// 验证结果
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
	mockDB := new(MockDocumentStore)
	adapter := NewOrbitDBAdapter(mockDB)

	// 创建测试事件
	event := &nostr.Event{
		ID:        "test-event",
		CreatedAt: nostr.Now(),
		Content:   "test content",
	}

	// 设置模拟行为
	mockDB.On("Put", mock.Anything, mock.Anything).Return("test-event", nil)

	// 执行保存
	err := adapter.SaveEvent(context.Background(), event)
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

// 测试删除事件
func TestDeleteEvent(t *testing.T) {
	mockDB := new(MockDocumentStore)
	adapter := NewOrbitDBAdapter(mockDB)

	// 创建测试事件
	event := &nostr.Event{
		ID:        "test-event",
		CreatedAt: nostr.Now(),
		Content:   "test content",
	}

	// 设置模拟行为
	mockDB.On("Delete", mock.Anything, "test-event").Return("test-event", nil)

	// 执行删除
	err := adapter.DeleteEvent(context.Background(), event)
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}
