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

// DocumentType 用于区分不同类型的文档
const (
	DocTypeNostrEvent = "nostr_event"
	DocTypeCausality  = "causality"
)

// CausalityKey 表示一个因果关系键
type CausalityKey struct {
	Key     uint32 `json:"key"`     // 因果关系键标识符
	Counter uint64 `json:"counter"` // Lamport时钟计数器
}

// SubspaceCausality 表示子空间的因果关系数据
type SubspaceCausality struct {
	ID         string            `json:"id"`          // 子空间ID，格式为0x开头的64位十六进制字符串
	DocType    string            `json:"doc_type"`    // 文档类型，这里为"causality"
	SubspaceID string            `json:"subspace_id"` // 子空间ID的另一种表示方式（如果需要）
	Keys       map[uint32]uint64 `json:"keys"`        // 键为causality key的ID，值为计数器
	Events     []string          `json:"events"`      // 关联的事件ID列表
	Created    int64             `json:"created"`     // 创建时间戳
	Updated    int64             `json:"updated"`     // 更新时间戳
}

// CausalityManager 管理因果关系的结构体
type CausalityManager struct {
	db iface.DocumentStore
}

// NewCausalityManager 创建一个新的因果关系管理器
func NewCausalityManager(db iface.DocumentStore) *CausalityManager {
	return &CausalityManager{
		db: db,
	}
}

// GetSubspaceCausality 获取子空间的因果关系数据
func (cm *CausalityManager) GetSubspaceCausality(ctx context.Context, subspaceID string) (*SubspaceCausality, error) {
	if !IsValidSubspaceID(subspaceID) {
		return nil, fmt.Errorf("无效的子空间ID格式: %s", subspaceID)
	}

	// 查询子空间数据
	docs, err := cm.db.Get(ctx, subspaceID, nil)
	if err != nil {
		return nil, err
	}

	// 如果不存在，返回空值
	if len(docs) == 0 {
		return nil, nil
	}

	// 遍历结果查找causality类型的文档
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

	// 将文档转换为JSON再解析为结构体
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

// parseOpsTag 解析ops标签，提取操作和对应的因果关系键
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
			log.Printf("无法解析因果关系键值: %s", valueStr)
			continue
		}

		result[key] = uint32(value)
	}

	return result
}

// UpdateFromEvent 从事件更新因果关系
func (cm *CausalityManager) UpdateFromEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("事件不能为空")
	}

	// 查找事件中的子空间ID标签
	var subspaceID string
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "sid" {
			subspaceID = tag[1]
			break
		}
	}

	if subspaceID == "" {
		// 没有子空间标签，不需要处理因果关系
		return nil
	}

	// 验证子空间ID格式
	if !IsValidSubspaceID(subspaceID) {
		log.Printf("警告: 事件 %s 包含无效的子空间ID格式: %s", event.ID, subspaceID)
		return nil
	}

	// 获取现有的子空间因果关系
	causality, err := cm.GetSubspaceCausality(ctx, subspaceID)
	if err != nil {
		return err
	}

	now := nostr.Now()

	// 如果不存在，创建新的
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
		// 更新事件列表，避免重复
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

	// 处理特殊事件类型
	if event.Kind == 30100 {
		// 这是子空间创建事件，需要初始化所有的因果关系键
		var opsValue string
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == "ops" {
				opsValue = tag[1]
				break
			}
		}

		if opsValue != "" {
			// 解析ops标签
			ops := parseOpsTag(opsValue)
			for _, keyID := range ops {
				// 初始化每个因果关系键的计数器为0
				causality.Keys[keyID] = 0
			}

			log.Printf("已初始化子空间 %s 的因果关系键: %v", subspaceID, causality.Keys)
		}
	} else {
		// 对于其他类型的事件，寻找对应的因果关系键并更新计数器
		var opName string
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == "ops" {
				opName = tag[1]
				break
			}
		}

		// 找到操作对应的因果关系键并增加计数器
		if opName != "" {
			// 对于特定的kind值，直接使用它的值作为因果关系键
			// 例如：kind 30302 对应 vote 操作
			var keyID uint32
			foundKey := false

			// 优先尝试使用kind值作为因果关系键
			keyID = uint32(event.Kind)
			if _, exists := causality.Keys[keyID]; exists {
				foundKey = true
				causality.Keys[keyID]++
				log.Printf("已更新子空间 %s 的因果关系键 %d 的计数器为 %d", subspaceID, keyID, causality.Keys[keyID])
			} else {
				// 如果没有直接匹配，尝试通过标签匹配操作
				for key, counter := range causality.Keys {
					// 这里需要一个映射表，将操作名称映射到对应的因果关系键
					// 但由于我们没有这个映射，所以这里只是示例代码
					keyIDStr := fmt.Sprintf("%d", key)
					if strings.HasSuffix(keyIDStr, opName) {
						causality.Keys[key] = counter + 1
						log.Printf("已更新子空间 %s 的因果关系键 %d 的计数器为 %d", subspaceID, key, causality.Keys[key])
						foundKey = true
						break
					}
				}
			}

			if !foundKey {
				log.Printf("警告: 无法找到操作 %s 对应的因果关系键", opName)
			}
		}
	}

	// 保存更新后的因果关系
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

// IsValidSubspaceID 判断子空间ID是否有效
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

// GetCausalityEvents 获取与特定子空间相关的所有事件
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

// GetCausalityKey 获取特定子空间的特定因果关系键
func (cm *CausalityManager) GetCausalityKey(ctx context.Context, subspaceID string, keyID uint32) (uint64, error) {
	causality, err := cm.GetSubspaceCausality(ctx, subspaceID)
	if err != nil {
		return 0, err
	}

	if causality == nil {
		return 0, fmt.Errorf("子空间 %s 不存在", subspaceID)
	}

	counter, exists := causality.Keys[keyID]
	if !exists {
		return 0, nil // 返回0表示键不存在
	}

	return counter, nil
}

// GetAllCausalityKeys 获取特定子空间的所有因果关系键
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

// QuerySubspaces 根据条件查询子空间
func (cm *CausalityManager) QuerySubspaces(ctx context.Context, filter func(*SubspaceCausality) bool) ([]*SubspaceCausality, error) {
	var results []*SubspaceCausality

	queryFn := func(doc interface{}) (bool, error) {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		// 检查是否是因果关系类型
		docType, ok := docMap["doc_type"].(string)
		if !ok || docType != DocTypeCausality {
			return false, nil
		}

		// 将文档转换为JSON
		jsonData, err := json.Marshal(docMap)
		if err != nil {
			return false, nil
		}

		var causality SubspaceCausality
		if err := json.Unmarshal(jsonData, &causality); err != nil {
			return false, nil
		}

		// 应用过滤器
		if filter == nil || filter(&causality) {
			results = append(results, &causality)
		}

		return true, nil
	}

	// 执行查询
	cm.db.Query(ctx, queryFn)

	return results, nil
}
