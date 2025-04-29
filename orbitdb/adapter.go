package orbitdb

import (
	"context"
	"fmt"
	"log"

	"berty.tech/go-orbit-db/iface"
	"github.com/nbd-wtf/go-nostr"
)

// OrbitDBAdapter 实现 eventstore.Store 接口
type OrbitDBAdapter struct {
	db iface.DocumentStore
}

// NewOrbitDBAdapter 创建一个新的 OrbitDB 适配器
func NewOrbitDBAdapter(db iface.DocumentStore) *OrbitDBAdapter {
	return &OrbitDBAdapter{
		db: db,
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
	}

	_, err := a.db.Put(ctx, doc)

	return err
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

			if len(filter.IDs) == 0 && len(filter.Authors) == 0 && len(filter.Kinds) == 0 {
				return true, nil
			}
			// 可以添加更多过滤条件...

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

// 辅助函数：检查切片中是否包含某个字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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
