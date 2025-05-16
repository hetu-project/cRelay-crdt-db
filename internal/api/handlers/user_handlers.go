package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/hetu-project/cRelay-crdt-db/internal/storage"
	"github.com/hetu-project/cRelay-crdt-db/orbitdb"
)

// UserHandlers 处理用户相关的HTTP请求
type UserHandlers struct {
	store storage.Store
}

// NewUserHandlers 创建用户处理程序
func NewUserHandlers(store storage.Store) *UserHandlers {
	return &UserHandlers{
		store: store,
	}
}

// GetUserStats 处理获取用户统计的请求
func (h *UserHandlers) GetUserStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	// 获取用户统计
	stats, err := h.store.GetUserStats(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取用户统计数据失败: %v", err), http.StatusInternalServerError)
		return
	}

	if stats == nil {
		http.Error(w, "用户统计数据不存在", http.StatusNotFound)
		return
	}

	// 返回JSON数据
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GetUserSubspaces 处理获取用户子空间的请求
func (h *UserHandlers) GetUserSubspaces(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	// 获取用户统计
	stats, err := h.store.GetUserStats(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取用户统计数据失败: %v", err), http.StatusInternalServerError)
		return
	}

	if stats == nil {
		http.Error(w, "用户统计数据不存在", http.StatusNotFound)
		return
	}

	// 构造响应数据结构
	response := map[string]interface{}{
		"created_subspaces": stats.CreatedSubspaces,
		"joined_subspaces":  stats.JoinedSubspaces,
	}

	// 返回JSON数据
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetUserInvites 处理获取用户邀请的请求
func (h *UserHandlers) GetUserInvites(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	// 获取用户统计
	stats, err := h.store.GetUserStats(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取用户统计数据失败: %v", err), http.StatusInternalServerError)
		return
	}

	if stats == nil || stats.InviteStats == nil {
		// 返回空数据而不是错误
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_invited":    0,
			"subspace_invited": map[string]uint64{},
			"invited_users":    map[string][]interface{}{},
		})
		return
	}

	// 返回JSON数据
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats.InviteStats)
}

// GetSubspaceUsers 处理获取子空间用户的请求
func (h *UserHandlers) GetSubspaceUsers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subspaceID := vars["id"]

	// 查询子空间用户
	users, err := h.store.QueryUsersBySubspace(r.Context(), subspaceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("查询子空间用户失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 构造简化的响应数据
	type EnhancedUserInfo struct {
		ID             string                     `json:"id"`                   // 用户ID
		JoinTime       time.Time                  `json:"join_time"`            // 加入时间
		LastActiveTime time.Time                  `json:"last_active_time"`     // 最后活跃时间
		TotalEvents    uint64                     `json:"total_events"`         // 在此子空间的总事件数
		EventBreakdown map[uint32]uint64          `json:"event_breakdown"`      // 事件类型分布
		VoteStats      *orbitdb.SubspaceVoteStats `json:"vote_stats,omitempty"` // 投票统计
		HasInvited     bool                       `json:"has_invited"`          // 是否邀请过其他用户
		InviteCount    uint64                     `json:"invite_count"`         // 邀请数量
	}

	enhancedUsers := make([]EnhancedUserInfo, 0, len(users))
	for _, user := range users {
		// 找到该用户在此子空间中最早的记录来估计加入时间
		var earliestTimestamp int64
		var totalEvents uint64

		// 获取该子空间的事件类型分布
		var eventBreakdown map[uint32]uint64
		if stats, exists := user.SubspaceStats[subspaceID]; exists {
			eventBreakdown = make(map[uint32]uint64)
			for eventType, count := range stats {
				eventBreakdown[eventType] = count
				totalEvents += count

				// 查找最早时间戳
				if earliestTimestamp == 0 || int64(count) < earliestTimestamp {
					earliestTimestamp = int64(count)
				}
			}
		}

		// 如果没有找到记录，使用最后更新时间
		if earliestTimestamp == 0 {
			earliestTimestamp = user.LastUpdated
		}

		// 获取投票统计
		var voteStats *orbitdb.SubspaceVoteStats
		if user.VoteStats != nil && user.VoteStats.SubspaceVotes != nil {
			if subspaceVote, exists := user.VoteStats.SubspaceVotes[subspaceID]; exists {
				voteStats = subspaceVote
			}
		}

		// 获取邀请统计
		hasInvited := false
		var inviteCount uint64
		if user.InviteStats != nil && user.InviteStats.SubspaceInvited != nil {
			if count, exists := user.InviteStats.SubspaceInvited[subspaceID]; exists && count > 0 {
				hasInvited = true
				inviteCount = count
			}
		}

		enhancedUsers = append(enhancedUsers, EnhancedUserInfo{
			ID:             user.ID,
			JoinTime:       time.Unix(earliestTimestamp, 0),
			LastActiveTime: time.Unix(user.LastUpdated, 0),
			TotalEvents:    totalEvents,
			EventBreakdown: eventBreakdown,
			VoteStats:      voteStats,
			HasInvited:     hasInvited,
			InviteCount:    inviteCount,
		})
	}

	// 返回JSON数据
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(enhancedUsers)
}

// GetSubspaceUsersStats 获取指定子空间内所有用户的统计数据
// func (h *UserHandlers) GetSubspaceUsersStats(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)
// 	subspaceID := vars["id"]

// 	// 查询子空间用户
// 	users, err := h.store.QueryUsersBySubspace(r.Context(), subspaceID)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("查询子空间用户失败: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// 构造详细的用户统计响应
// 	type SubspaceActivity struct {
// 		TotalEvents    uint64            `json:"total_events"`               // 该子空间的总事件数
// 		EventBreakdown map[uint32]uint64 `json:"event_breakdown"`            // 事件类型分布
// 		JoinTime       time.Time         `json:"join_time,omitempty"`        // 加入时间
// 		LastActiveTime time.Time         `json:"last_active_time,omitempty"` // 最后活跃时间
// 	}

// 	type UserSubspaceStats struct {
// 		UserID         string                     `json:"user_id"`                   // 用户ID
// 		TotalEvents    uint64                     `json:"total_events"`              // 在此子空间总事件数
// 		EventBreakdown map[uint32]uint64          `json:"event_breakdown"`           // 事件类型分布
// 		JoinTime       time.Time                  `json:"join_time"`                 // 加入时间
// 		LastActiveTime time.Time                  `json:"last_active_time"`          // 最后活跃时间
// 		VotingActivity *orbitdb.SubspaceVoteStats `json:"voting_activity,omitempty"` // 投票活动
// 		InviteActivity struct {
// 			TotalInvited uint64                     `json:"total_invited"`           // 邀请总数
// 			InvitedUsers []*orbitdb.InvitedUserInfo `json:"invited_users,omitempty"` // 被邀请用户列表
// 		} `json:"invite_activity"`
// 		AllSubspacesStats map[string]SubspaceActivity `json:"all_subspaces_stats"` // 用户所有子空间的统计数据
// 	}

// 	userStats := make([]UserSubspaceStats, 0, len(users))

// 	for _, user := range users {
// 		// 初始化用户统计
// 		stats := UserSubspaceStats{
// 			UserID:            user.ID,
// 			EventBreakdown:    make(map[uint32]uint64),
// 			LastActiveTime:    time.Unix(user.LastUpdated, 0),
// 			AllSubspacesStats: make(map[string]SubspaceActivity),
// 		}

// 		// 处理当前子空间的统计
// 		var earliestTimestamp int64
// 		if subStats, exists := user.SubspaceStats[subspaceID]; exists {
// 			for eventType, count := range subStats {
// 				stats.EventBreakdown[eventType] = count
// 				stats.TotalEvents += count

// 				// 查找最早时间戳估计加入时间
// 				if earliestTimestamp == 0 || int64(count) < earliestTimestamp {
// 					earliestTimestamp = int64(count)
// 				}
// 			}
// 		}

// 		// 如果没有找到记录，使用最后更新时间
// 		if earliestTimestamp == 0 {
// 			earliestTimestamp = user.LastUpdated
// 		}
// 		stats.JoinTime = time.Unix(earliestTimestamp, 0)

// 		// 添加投票活动
// 		if user.VoteStats != nil && user.VoteStats.SubspaceVotes != nil {
// 			if voteStats, exists := user.VoteStats.SubspaceVotes[subspaceID]; exists {
// 				stats.VotingActivity = voteStats
// 			}
// 		}

// 		// 添加邀请活动
// 		if user.InviteStats != nil {
// 			if count, exists := user.InviteStats.SubspaceInvited[subspaceID]; exists {
// 				stats.InviteActivity.TotalInvited = count
// 			}
// 			if invitedUsers, exists := user.InviteStats.InvitedUsers[subspaceID]; exists {
// 				stats.InviteActivity.InvitedUsers = invitedUsers
// 			}
// 		}

// 		// 处理用户所有子空间的统计
// 		for sid, subStats := range user.SubspaceStats {
// 			var subTotalEvents uint64
// 			var subEarliestTimestamp int64
// 			subEventBreakdown := make(map[uint32]uint64)

// 			for eventType, count := range subStats {
// 				subEventBreakdown[eventType] = count
// 				subTotalEvents += count

// 				// 查找最早时间戳估计加入时间
// 				if subEarliestTimestamp == 0 || int64(count) < subEarliestTimestamp {
// 					subEarliestTimestamp = int64(count)
// 				}
// 			}

// 			// 如果没有找到记录，使用最后更新时间
// 			if subEarliestTimestamp == 0 {
// 				subEarliestTimestamp = user.LastUpdated
// 			}

// 			stats.AllSubspacesStats[sid] = SubspaceActivity{
// 				TotalEvents:    subTotalEvents,
// 				EventBreakdown: subEventBreakdown,
// 				JoinTime:       time.Unix(subEarliestTimestamp, 0),
// 				LastActiveTime: time.Unix(user.LastUpdated, 0),
// 			}
// 		}

// 		userStats = append(userStats, stats)
// 	}

// 	// 返回JSON数据
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(userStats)
// }

// ListTopUsers 列出活跃度最高的用户
func (h *UserHandlers) ListTopUsers(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	query := r.URL.Query()
	limitStr := query.Get("limit")
	sortBy := query.Get("sort_by") // 可以是 "total_events", "votes", "invites" 等

	limit := 10 // 默认限制
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if sortBy == "" {
		sortBy = "total_events" // 默认按总事件数排序
	}

	// 创建过滤器函数
	filter := func(stats *orbitdb.UserStats) bool {
		// 这里可以添加更多的过滤条件
		return true
	}

	// 查询所有用户统计
	users, err := h.store.QueryUserStats(r.Context(), filter)
	if err != nil {
		http.Error(w, fmt.Sprintf("查询用户统计失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 根据排序字段对用户进行排序
	switch sortBy {
	case "total_events":
		// 按总事件数排序
		sortUsersByTotalEvents(users)
	case "votes":
		// 按投票数排序
		sortUsersByVotes(users)
	case "invites":
		// 按邀请数排序
		sortUsersByInvites(users)
	}

	// 限制结果数量
	if len(users) > limit {
		users = users[:limit]
	}

	// 构造响应数据
	type UserRanking struct {
		ID             string            `json:"id"`
		TotalEvents    uint64            `json:"total_events"`
		EventBreakdown map[uint32]uint64 `json:"event_breakdown"`
		SubspaceCount  int               `json:"subspace_count"`
		LastActive     time.Time         `json:"last_active"`
	}

	rankings := make([]UserRanking, 0, len(users))
	for _, user := range users {
		var totalEvents uint64
		for _, count := range user.TotalStats {
			totalEvents += count
		}

		rankings = append(rankings, UserRanking{
			ID:             user.ID,
			TotalEvents:    totalEvents,
			EventBreakdown: user.TotalStats,
			SubspaceCount:  len(user.JoinedSubspaces),
			LastActive:     time.Unix(user.LastUpdated, 0),
		})
	}

	// 返回JSON数据
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rankings)
}

// 辅助函数：按总事件数排序
func sortUsersByTotalEvents(users []*orbitdb.UserStats) {
	// 实现排序逻辑
	for i := 0; i < len(users)-1; i++ {
		for j := i + 1; j < len(users); j++ {
			var totalEventsI, totalEventsJ uint64
			for _, count := range users[i].TotalStats {
				totalEventsI += count
			}
			for _, count := range users[j].TotalStats {
				totalEventsJ += count
			}

			// 降序排序
			if totalEventsJ > totalEventsI {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
}

// 辅助函数：按投票数排序
func sortUsersByVotes(users []*orbitdb.UserStats) {
	for i := 0; i < len(users)-1; i++ {
		for j := i + 1; j < len(users); j++ {
			var votesI, votesJ uint64
			if users[i].VoteStats != nil {
				votesI = users[i].VoteStats.TotalVotes
			}
			if users[j].VoteStats != nil {
				votesJ = users[j].VoteStats.TotalVotes
			}

			// 降序排序
			if votesJ > votesI {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
}

// 辅助函数：按邀请数排序
func sortUsersByInvites(users []*orbitdb.UserStats) {
	for i := 0; i < len(users)-1; i++ {
		for j := i + 1; j < len(users); j++ {
			var invitesI, invitesJ uint64
			if users[i].InviteStats != nil {
				invitesI = users[i].InviteStats.TotalInvited
			}
			if users[j].InviteStats != nil {
				invitesJ = users[j].InviteStats.TotalInvited
			}

			// 降序排序
			if invitesJ > invitesI {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
}
