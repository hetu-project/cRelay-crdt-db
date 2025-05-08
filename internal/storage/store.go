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

	"github.com/hetu-project/cRelay-crdt-db/orbitdb"
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

	// 新增方法：获取子空间的因果关系数据
	GetSubspaceCausality(ctx context.Context, subspaceID string) (*orbitdb.SubspaceCausality, error)

	// 新增方法：查询子空间
	QuerySubspaces(ctx context.Context, filter func(*orbitdb.SubspaceCausality) bool) ([]*orbitdb.SubspaceCausality, error)

	// UpdateFromEvent 从事件更新因果关系
	UpdateFromEvent(ctx context.Context, event *nostr.Event) error

	//GetCausalityEvents 获取与特定子空间相关的所有事件
	GetCausalityEvents(ctx context.Context, subspaceID string) ([]string, error)

	// GetCausalityKey 获取特定子空间的特定因果关系键
	GetCausalityKey(ctx context.Context, subspaceID string, keyID uint32) (uint64, error)

	// GetAllCausalityKeys 获取特定子空间的所有因果关系键
	GetAllCausalityKeys(ctx context.Context, subspaceID string) (map[uint32]uint64, error)

	// 新增用户统计相关方法

	// GetUserStats 获取用户统计数据
	GetUserStats(ctx context.Context, userID string) (*orbitdb.UserStats, error)

	// QueryUsersBySubspace 查询特定子空间的所有用户
	QueryUsersBySubspace(ctx context.Context, subspaceID string) ([]*orbitdb.UserStats, error)

	// QueryUserStats 根据条件查询用户统计
	QueryUserStats(ctx context.Context, filter func(*orbitdb.UserStats) bool) ([]*orbitdb.UserStats, error)
}

// StoreFactory 用于创建存储实例的工厂接口
type StoreFactory interface {
	// CreateStore 创建并初始化一个存储实例
	CreateStore() (Store, error)
}
