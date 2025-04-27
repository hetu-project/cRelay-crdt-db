// package storage

// import (
// 	"context"

// 	"github.com/nbd-wtf/go-nostr"
// )

// type Store interface {
// 	// SaveEvent 保存一个事件
// 	SaveEvent(ctx context.Context, event *nostr.Event) error

// 	// QueryEvents 查询匹配过滤器的事件
// 	QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error)

//		// DeleteEvent 删除一个事件
//		DeleteEvent(ctx context.Context, event *nostr.Event) error
//	}
package storage

import (
	"context"
	"errors"

	"github.com/nbd-wtf/go-nostr"
)

// 定义存储系统中可能出现的错误
var (
	ErrEventNotFound      = errors.New("事件未找到")
	ErrInvalidEventFormat = errors.New("无效的事件格式")
	ErrStorageNotStarted  = errors.New("存储系统未启动")
)

// Store 定义了与 nostr 事件交互的存储接口
type Store interface {
	// SaveEvent 保存一个 nostr 事件
	SaveEvent(ctx context.Context, event *nostr.Event) error

	// GetEvent 通过 ID 获取一个事件
	// GetEvent(id string) (*nostr.Event, error)

	// QueryEvents 查询匹配过滤器的事件
	QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error)

	// DeleteEvent 删除一个事件
	DeleteEvent(ctx context.Context, event *nostr.Event) error

	// Close 关闭存储连接
	// Close() error
}

// StoreFactory 用于创建存储实例的工厂接口
type StoreFactory interface {
	// CreateStore 创建并初始化一个存储实例
	CreateStore() (Store, error)
}
