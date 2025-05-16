package orbitdb

import (
	"context"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// 测试子空间ID验证
func TestIsValidSubspaceID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{
			name:     "有效的子空间ID",
			id:       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: true,
		},
		{
			name:     "无效的长度",
			id:       "0x123",
			expected: false,
		},
		{
			name:     "无效的前缀",
			id:       "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: false,
		},
		{
			name:     "无效的字符",
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

// 测试获取子空间因果关系
func TestGetSubspaceCausality(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// 创建测试数据
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

	// 设置模拟行为
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{causalityDoc}, nil)

	// 执行测试
	causality, err := manager.GetSubspaceCausality(context.Background(), subspaceID)
	assert.NoError(t, err)
	assert.NotNil(t, causality)
	assert.Equal(t, subspaceID, causality.ID)
	assert.Equal(t, DocTypeCausality, causality.DocType)
	assert.Equal(t, uint64(5), causality.Keys[1])
	assert.Equal(t, uint64(3), causality.Keys[2])
	assert.Equal(t, []string{"event1", "event2"}, causality.Events)
}

// 测试从事件更新因果关系
func TestUpdateFromEvent(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// 创建测试事件
	subspaceID := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	event := &nostr.Event{
		ID:        "test-event",
		PubKey:    "test-pubkey",
		CreatedAt: nostr.Now(),
		Kind:      30302, // 假设这是一个投票事件
		Tags: nostr.Tags{
			{"sid", subspaceID},
			{"op", "vote"},
		},
	}

	// 设置模拟行为
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{}, nil)
	mockDB.On("Put", mock.Anything, mock.Anything).Return(subspaceID, nil)

	// 执行测试
	err := manager.UpdateFromEvent(context.Background(), event)
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

// 测试获取因果关系事件
func TestGetCausalityEvents(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// 创建测试数据
	subspaceID := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	events := []string{"event1", "event2", "event3"}
	causalityDoc := map[string]interface{}{
		"_id":         subspaceID,
		"id":          subspaceID,
		"doc_type":    DocTypeCausality,
		"subspace_id": subspaceID,
		"events":      events,
	}

	// 设置模拟行为
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{causalityDoc}, nil)

	// 执行测试
	result, err := manager.GetCausalityEvents(context.Background(), subspaceID)
	assert.NoError(t, err)
	assert.Equal(t, events, result)
}

// 测试获取因果关系键
func TestGetCausalityKey(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// 创建测试数据
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

	// 设置模拟行为
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{causalityDoc}, nil)

	// 执行测试
	result, err := manager.GetCausalityKey(context.Background(), subspaceID, keyID)
	assert.NoError(t, err)
	assert.Equal(t, counter, result)
}

// 测试获取所有因果关系键
func TestGetAllCausalityKeys(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// 创建测试数据
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

	// 设置模拟行为
	mockDB.On("Get", mock.Anything, subspaceID, nil).Return([]interface{}{causalityDoc}, nil)

	// 执行测试
	result, err := manager.GetAllCausalityKeys(context.Background(), subspaceID)
	assert.NoError(t, err)
	assert.Equal(t, keys, result)
}

// 测试查询子空间
func TestQuerySubspaces(t *testing.T) {
	mockDB := new(MockDocumentStore)
	manager := NewCausalityManager(mockDB)

	// 创建测试数据
	subspaceID := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	now := time.Now().Unix()
	causalityDoc := map[string]interface{}{
		"_id":         subspaceID,
		"id":          subspaceID,
		"doc_type":    DocTypeCausality,
		"subspace_id": subspaceID,
		"updated":     float64(now),
	}

	// 设置模拟行为
	mockDB.On("Query", mock.Anything, mock.Anything).Return([]interface{}{causalityDoc}, nil)

	// 创建过滤器
	filter := func(c *SubspaceCausality) bool {
		return c.Updated >= now-3600 // 只返回最近一小时更新的子空间
	}

	// 执行测试
	results, err := manager.QuerySubspaces(context.Background(), filter)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, subspaceID, results[0].ID)
}
