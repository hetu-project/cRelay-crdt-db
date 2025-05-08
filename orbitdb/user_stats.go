package orbitdb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"berty.tech/go-orbit-db/iface"
	"github.com/nbd-wtf/go-nostr"
)

// UserStats 表示用户的统计数据
type UserStats struct {
	ID               string                       `json:"id"`                     // 用户ID，即用户的ETH地址
	DocType          string                       `json:"doc_type"`               // 文档类型，固定为"user_stats"
	TotalStats       map[uint32]uint64            `json:"total_stats"`            // 各种操作的总体统计
	SubspaceStats    map[string]map[uint32]uint64 `json:"subspace_stats"`         // 各个子空间的统计
	CreatedSubspaces []string                     `json:"created_subspaces"`      // 用户创建的子空间ID列表
	JoinedSubspaces  []string                     `json:"joined_subspaces"`       // 用户加入的子空间ID列表
	VoteStats        *VoteStats                   `json:"vote_stats,omitempty"`   // 投票统计
	InviteStats      *InviteStats                 `json:"invite_stats,omitempty"` // 邀请统计
	LastUpdated      int64                        `json:"last_updated"`           // 最后更新时间
}

// VoteStats 投票相关统计
type VoteStats struct {
	TotalVotes    uint64                        `json:"total_votes"`    // 总投票数
	YesVotes      uint64                        `json:"yes_votes"`      // 总赞成票
	NoVotes       uint64                        `json:"no_votes"`       // 总反对票
	SubspaceVotes map[string]*SubspaceVoteStats `json:"subspace_votes"` // 各子空间投票统计
}

// SubspaceVoteStats 子空间投票统计
type SubspaceVoteStats struct {
	TotalVotes uint64 `json:"total_votes"` // 子空间总投票数
	YesVotes   uint64 `json:"yes_votes"`   // 子空间赞成票
	NoVotes    uint64 `json:"no_votes"`    // 子空间反对票
}

// InviteStats 邀请相关统计
type InviteStats struct {
	TotalInvited    uint64                        `json:"total_invited"`    // 总邀请成功数
	SubspaceInvited map[string]uint64             `json:"subspace_invited"` // 各子空间邀请成功数
	InvitedUsers    map[string][]*InvitedUserInfo `json:"invited_users"`    // 接受邀请的用户信息
}

// InvitedUserInfo 被邀请用户信息
type InvitedUserInfo struct {
	UserID     string `json:"user_id"`     // 被邀请用户地址
	SubspaceID string `json:"subspace_id"` // 被邀请加入的子空间
	Timestamp  int64  `json:"timestamp"`   // 邀请接受时间
}

// UserStatsManager 管理用户统计的结构体
type UserStatsManager struct {
	db iface.DocumentStore
}

// NewUserStatsManager 创建一个新的用户统计管理器
func NewUserStatsManager(db iface.DocumentStore) *UserStatsManager {
	return &UserStatsManager{
		db: db,
	}
}

// GetUserStats 获取用户统计数据
func (um *UserStatsManager) GetUserStats(ctx context.Context, userID string) (*UserStats, error) {
	// 查询用户数据
	docs, err := um.db.Get(ctx, userID, nil)
	if err != nil {
		return nil, err
	}

	// 如果不存在，返回空值
	if len(docs) == 0 {
		return nil, nil
	}

	// 遍历结果查找user_stats类型的文档
	var userStatsDoc map[string]interface{}
	for _, doc := range docs {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			continue
		}

		docType, ok := docMap["doc_type"].(string)
		if !ok || docType != "user_stats" {
			continue
		}

		userStatsDoc = docMap
		break
	}

	if userStatsDoc == nil {
		return nil, nil
	}

	// 将文档转换为JSON再解析为结构体
	jsonData, err := json.Marshal(userStatsDoc)
	if err != nil {
		return nil, err
	}

	var userStats UserStats
	if err := json.Unmarshal(jsonData, &userStats); err != nil {
		return nil, err
	}

	return &userStats, nil
}

// UpdateUserStatsFromEvent 从事件更新用户统计
func (um *UserStatsManager) UpdateUserStatsFromEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("事件不能为空")
	}

	// 获取用户的现有统计数据
	userID := event.PubKey
	stats, err := um.GetUserStats(ctx, userID)
	if err != nil {
		return err
	}

	// 如果不存在，创建新的统计数据
	now := time.Now().Unix()
	if stats == nil {
		stats = &UserStats{
			ID:               userID,
			DocType:          "user_stats",
			TotalStats:       make(map[uint32]uint64),
			SubspaceStats:    make(map[string]map[uint32]uint64),
			CreatedSubspaces: []string{},
			JoinedSubspaces:  []string{},
			LastUpdated:      now,
		}
	}

	// 更新最后更新时间
	stats.LastUpdated = now

	// 根据事件类型更新统计
	kind := uint32(event.Kind)

	// 递增总统计
	stats.TotalStats[kind] = stats.TotalStats[kind] + 1

	// 查找事件中的子空间ID
	var subspaceID string
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "sid" {
			subspaceID = tag[1]
			break
		}
	}

	// 如果有子空间ID，更新子空间相关统计
	if subspaceID != "" {
		// 确保子空间统计存在
		if _, exists := stats.SubspaceStats[subspaceID]; !exists {
			stats.SubspaceStats[subspaceID] = make(map[uint32]uint64)
		}
		// 递增子空间统计
		stats.SubspaceStats[subspaceID][kind] = stats.SubspaceStats[subspaceID][kind] + 1

		switch kind {
		case 30100: // 创建子空间
			// 将子空间添加到创建的子空间列表
			if !containsString(stats.CreatedSubspaces, subspaceID) {
				stats.CreatedSubspaces = append(stats.CreatedSubspaces, subspaceID)
			}

		case 30200: // 加入子空间
			// 将子空间添加到加入的子空间列表
			if !containsString(stats.JoinedSubspaces, subspaceID) {
				stats.JoinedSubspaces = append(stats.JoinedSubspaces, subspaceID)
			}

		case 30302: // 投票
			// 初始化投票统计
			if stats.VoteStats == nil {
				stats.VoteStats = &VoteStats{
					SubspaceVotes: make(map[string]*SubspaceVoteStats),
				}
			}

			// 确保子空间投票统计存在
			if _, exists := stats.VoteStats.SubspaceVotes[subspaceID]; !exists {
				stats.VoteStats.SubspaceVotes[subspaceID] = &SubspaceVoteStats{}
			}

			// 递增总投票数
			stats.VoteStats.TotalVotes++
			// 递增子空间投票数
			stats.VoteStats.SubspaceVotes[subspaceID].TotalVotes++

			// 检查投票类型（是/否）
			for _, tag := range event.Tags {
				if len(tag) >= 2 && tag[0] == "vote" {
					voteValue := tag[1]
					if voteValue == "yes" {
						stats.VoteStats.YesVotes++
						stats.VoteStats.SubspaceVotes[subspaceID].YesVotes++
					} else if voteValue == "no" {
						stats.VoteStats.NoVotes++
						stats.VoteStats.SubspaceVotes[subspaceID].NoVotes++
					}
					break
				}
			}

		case 30303: // 邀请
			// 处理接受邀请的情况
			var inviterAddr string
			for _, tag := range event.Tags {
				if len(tag) >= 2 && tag[0] == "inviter_addr" {
					inviterAddr = tag[1]
					break
				}
			}

			if inviterAddr != "" {
				// 发起邀请的是其他用户，当前用户是受邀者
				// 需要更新邀请者的统计
				err = um.updateInviterStats(ctx, inviterAddr, userID, subspaceID, now)
				if err != nil {
					log.Printf("更新邀请者统计失败: %v", err)
				}
			}
		}
	}

	// 保存更新后的统计
	return um.saveUserStats(ctx, stats)
}

// 更新邀请者的邀请统计
func (um *UserStatsManager) updateInviterStats(ctx context.Context, inviterID, invitedID, subspaceID string, timestamp int64) error {
	// 获取邀请者的统计
	inviterStats, err := um.GetUserStats(ctx, inviterID)
	if err != nil {
		return err
	}

	// 如果不存在，创建新的统计
	if inviterStats == nil {
		inviterStats = &UserStats{
			ID:            inviterID,
			DocType:       "user_stats",
			TotalStats:    make(map[uint32]uint64),
			SubspaceStats: make(map[string]map[uint32]uint64),
			LastUpdated:   timestamp,
		}
	}

	// 初始化邀请统计
	if inviterStats.InviteStats == nil {
		inviterStats.InviteStats = &InviteStats{
			SubspaceInvited: make(map[string]uint64),
			InvitedUsers:    make(map[string][]*InvitedUserInfo),
		}
	}

	// 更新邀请统计
	inviterStats.InviteStats.TotalInvited++
	inviterStats.InviteStats.SubspaceInvited[subspaceID]++

	// 添加被邀请用户信息
	userInfo := &InvitedUserInfo{
		UserID:     invitedID,
		SubspaceID: subspaceID,
		Timestamp:  timestamp,
	}

	// 追加到用户列表
	if _, exists := inviterStats.InviteStats.InvitedUsers[subspaceID]; !exists {
		inviterStats.InviteStats.InvitedUsers[subspaceID] = []*InvitedUserInfo{}
	}
	inviterStats.InviteStats.InvitedUsers[subspaceID] = append(inviterStats.InviteStats.InvitedUsers[subspaceID], userInfo)

	// 保存更新后的统计
	return um.saveUserStats(ctx, inviterStats)
}

// saveUserStats 保存用户统计数据
func (um *UserStatsManager) saveUserStats(ctx context.Context, stats *UserStats) error {
	doc := map[string]interface{}{
		"_id":               stats.ID,
		"id":                stats.ID,
		"doc_type":          stats.DocType,
		"total_stats":       stats.TotalStats,
		"subspace_stats":    stats.SubspaceStats,
		"created_subspaces": stats.CreatedSubspaces,
		"joined_subspaces":  stats.JoinedSubspaces,
		"last_updated":      stats.LastUpdated,
	}

	if stats.VoteStats != nil {
		doc["vote_stats"] = stats.VoteStats
	}

	if stats.InviteStats != nil {
		doc["invite_stats"] = stats.InviteStats
	}

	_, err := um.db.Put(ctx, doc)
	return err
}

// QueryUsersBySubspace 查询特定子空间的所有用户
func (um *UserStatsManager) QueryUsersBySubspace(ctx context.Context, subspaceID string) ([]*UserStats, error) {
	var results []*UserStats

	queryFn := func(doc interface{}) (bool, error) {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		// 检查是否是用户统计类型
		docType, ok := docMap["doc_type"].(string)
		if !ok || docType != "user_stats" {
			return false, nil
		}

		// 检查是否包含指定子空间
		joinedSubspaces, ok := docMap["joined_subspaces"].([]interface{})
		if ok {
			for _, sid := range joinedSubspaces {
				if sidStr, ok := sid.(string); ok && sidStr == subspaceID {
					// 将文档转换为JSON
					jsonData, err := json.Marshal(docMap)
					if err != nil {
						return false, nil
					}

					var userStats UserStats
					if err := json.Unmarshal(jsonData, &userStats); err != nil {
						return false, nil
					}

					results = append(results, &userStats)
					return true, nil
				}
			}
		}

		return false, nil
	}

	// 执行查询
	um.db.Query(ctx, queryFn)

	return results, nil
}

// QueryUserStats 根据条件查询用户统计
func (um *UserStatsManager) QueryUserStats(ctx context.Context, filter func(*UserStats) bool) ([]*UserStats, error) {
	var results []*UserStats

	queryFn := func(doc interface{}) (bool, error) {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		// 检查是否是用户统计类型
		docType, ok := docMap["doc_type"].(string)
		if !ok || docType != "user_stats" {
			return false, nil
		}

		// 将文档转换为JSON
		jsonData, err := json.Marshal(docMap)
		if err != nil {
			return false, nil
		}

		var userStats UserStats
		if err := json.Unmarshal(jsonData, &userStats); err != nil {
			return false, nil
		}

		// 应用过滤器
		if filter == nil || filter(&userStats) {
			results = append(results, &userStats)
		}

		return true, nil
	}

	// 执行查询
	um.db.Query(ctx, queryFn)

	return results, nil
}

// containsString 检查字符串数组是否包含特定字符串
func containsString(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}
