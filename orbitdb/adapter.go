package orbitdb

import (
	"context"
	"fmt"
	"log"
	"strings"

	"berty.tech/go-orbit-db/iface"
	"github.com/nbd-wtf/go-nostr"
)

// OrbitDBAdapter 实现 eventstore.Store 接口
type OrbitDBAdapter struct {
	db           iface.DocumentStore
	causalityMgr *CausalityManager
	userStatsMgr *UserStatsManager
}

// NewOrbitDBAdapter 创建一个新的 OrbitDB 适配器
func NewOrbitDBAdapter(db iface.DocumentStore) *OrbitDBAdapter {
	return &OrbitDBAdapter{
		db:           db,
		causalityMgr: NewCausalityManager(db), // 使用同一个数据库实例
		userStatsMgr: NewUserStatsManager(db), // 使用同一个数据库实例
	}
}

// SaveEvent 保存事件到 OrbitDB
// 更新签名以匹配 func(ctx context.Context, event *nostr.Event) error
func (a *OrbitDBAdapter) SaveEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("事件不能为空")
	}

	doc := map[string]interface{}{
		"_id":        event.ID,
		"pubkey":     event.PubKey,
		"created_at": event.CreatedAt,
		"kind":       event.Kind,
		"content":    event.Content,
		"sig":        event.Sig,
		"tags":       event.Tags,
		"doc_type":   DocTypeNostrEvent, // 添加文档类型标识
	}

	_, err := a.db.Put(ctx, doc)

	if err != nil {
		return err
	}

	// 更新因果关系
	if a.causalityMgr != nil {
		// 尝试更新因果关系，但不影响事件存储
		if updateErr := a.causalityMgr.UpdateFromEvent(ctx, event); updateErr != nil {
			log.Printf("警告: 更新因果关系失败: %v", updateErr)
		}
	}

	// 更新用户统计
	if a.userStatsMgr != nil {
		// 尝试更新用户统计，但不影响事件存储
		if updateErr := a.userStatsMgr.UpdateUserStatsFromEvent(ctx, event); updateErr != nil {
			log.Printf("警告: 更新用户统计失败: %v", updateErr)
		}
	}

	return nil
}

func (a *OrbitDBAdapter) QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	// 创建事件通道
	eventChan := make(chan *nostr.Event)

	go func() {
		defer close(eventChan)

		// 定义查询函数
		queryFn := func(doc interface{}) (bool, error) {
			event, ok := doc.(map[string]interface{})
			if !ok {
				return false, nil
			}

			// 只处理nostr事件类型的文档
			docType, ok := event["doc_type"].(string)
			if !ok || docType != DocTypeNostrEvent {
				return false, nil
			}

			// 实现过滤逻辑
			if len(filter.IDs) > 0 {
				id, ok := event["_id"].(string) // 注意这里是 _id 而不是 id
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

			// if len(filter.IDs) == 0 && len(filter.Authors) == 0 && len(filter.Kinds) == 0 {
			// 	return true, nil
			// }

			// 过滤 #sid 标签
			// 检查标签过滤条件
			if len(filter.Tags) > 0 {
				tags, ok := event["tags"].([]interface{})
				if !ok {
					return false, nil
				}

				// 对每个标签过滤条件进行检查
				for tagName, tagValues := range filter.Tags {
					if len(tagValues) == 0 {
						continue
					}

					// 查找事件中是否有匹配的标签
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

						// 检查标签值是否在过滤条件中
						if contains(tagValues, value) {
							found = true
							break
						}
					}

					// 如果没有找到匹配的标签，则跳过此事件
					if !found {
						return false, nil
					}
				}
			}
			return true, nil
		}

		// 执行查询
		docs, _ := a.db.Query(ctx, queryFn)
		for _, doc := range docs {
			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				return
			default:
				// 继续处理
			}

			// 直接构建事件对象，而不是通过JSON序列化和反序列化
			docMap, ok := doc.(map[string]interface{})
			if !ok {
				log.Printf("无效的文档格式")
				continue
			}

			event := &nostr.Event{}

			// 设置基本字段
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

			// 处理标签
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

			// 发送事件到通道
			select {
			case <-ctx.Done():
				return
			case eventChan <- event:
				// 事件已发送
			}
		}
	}()

	return eventChan, nil
}

// DeleteEvent 从数据库中删除事件
// 更新签名以匹配 func(ctx context.Context, event *nostr.Event) error
func (a *OrbitDBAdapter) DeleteEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("事件不能为空")
	}

	_, err := a.db.Delete(ctx, event.ID)
	return err
}

// CountEvents 实现计数方法以匹配 Counter 接口
func (a *OrbitDBAdapter) CountEvents(ctx context.Context, filter nostr.Filter) (int, error) {
	count := 0

	queryFn := func(doc interface{}) (bool, error) {
		event, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		// 实现与 QueryEvents 相同的过滤逻辑
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

	// 执行查询计数
	a.db.Query(ctx, queryFn)

	return count, nil
}

func (a *OrbitDBAdapter) ReplaceEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("事件不能为空")
	}

	doc := map[string]interface{}{
		"_id":        event.ID,
		"pubkey":     event.PubKey,
		"created_at": event.CreatedAt,
		"kind":       event.Kind,
		"content":    event.Content,
		"sig":        event.Sig,
		"tags":       event.Tags,
		"doc_type":   DocTypeNostrEvent, // 添加文档类型标识
	}

	_, err := a.db.Put(ctx, doc)

	if err != nil {
		return err
	}

	// 更新因果关系
	if a.causalityMgr != nil {
		// 尝试更新因果关系，但不影响事件存储
		if updateErr := a.causalityMgr.UpdateFromEvent(ctx, event); updateErr != nil {
			log.Printf("警告: 更新因果关系失败: %v", updateErr)
		}
	}

	// 更新用户统计
	if a.userStatsMgr != nil {
		// 尝试更新用户统计，但不影响事件存储
		if updateErr := a.userStatsMgr.UpdateUserStatsFromEvent(ctx, event); updateErr != nil {
			log.Printf("警告: 更新用户统计失败: %v", updateErr)
		}
	}

	return nil
}

// 辅助函数：检查切片中是否包含某个字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetSubspaceCausality 获取子空间的因果关系数据
func (a *OrbitDBAdapter) GetSubspaceCausality(ctx context.Context, subspaceID string) (*SubspaceCausality, error) {
	return a.causalityMgr.GetSubspaceCausality(ctx, subspaceID)
}

// QuerySubspaces 根据条件查询子空间
func (a *OrbitDBAdapter) QuerySubspaces(ctx context.Context, filter func(*SubspaceCausality) bool) ([]*SubspaceCausality, error) {
	return a.causalityMgr.QuerySubspaces(ctx, filter)
}

// UpdateFromEvent 从事件更新因果关系
func (a *OrbitDBAdapter) UpdateFromEvent(ctx context.Context, event *nostr.Event) error {
	return a.causalityMgr.UpdateFromEvent(ctx, event)
}

// GetCausalityEvents 获取与特定子空间相关的所有事件
func (a *OrbitDBAdapter) GetCausalityEvents(ctx context.Context, subspaceID string) ([]string, error) {
	return a.causalityMgr.GetCausalityEvents(ctx, subspaceID)
}

// GetCausalityKey 获取特定子空间的特定因果关系键
func (a *OrbitDBAdapter) GetCausalityKey(ctx context.Context, subspaceID string, keyID uint32) (uint64, error) {
	return a.causalityMgr.GetCausalityKey(ctx, subspaceID, keyID)
}

// GetAllCausalityKeys 获取特定子空间的所有因果关系键
func (a *OrbitDBAdapter) GetAllCausalityKeys(ctx context.Context, subspaceID string) (map[uint32]uint64, error) {
	return a.causalityMgr.GetAllCausalityKeys(ctx, subspaceID)
}

// GetUserStats 获取用户统计数据
func (a *OrbitDBAdapter) GetUserStats(ctx context.Context, userID string) (*UserStats, error) {
	return a.userStatsMgr.GetUserStats(ctx, userID)
}

// QueryUsersBySubspace 查询特定子空间的所有用户
func (a *OrbitDBAdapter) QueryUsersBySubspace(ctx context.Context, subspaceID string) ([]*UserStats, error) {
	return a.userStatsMgr.QueryUsersBySubspace(ctx, subspaceID)
}

// QueryUserStats 根据条件查询用户统计
func (a *OrbitDBAdapter) QueryUserStats(ctx context.Context, filter func(*UserStats) bool) ([]*UserStats, error) {
	return a.userStatsMgr.QueryUserStats(ctx, filter)
}

// 辅助函数：检查切片中是否包含某个整数
func containsInt(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
