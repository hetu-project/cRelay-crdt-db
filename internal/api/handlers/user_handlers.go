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
	type SimpleUserInfo struct {
		ID             string    `json:"id"`
		JoinTime       time.Time `json:"join_time"`
		LastActiveTime time.Time `json:"last_active_time"`
	}

	simpleUsers := make([]SimpleUserInfo, 0, len(users))
	for _, user := range users {
		// 找到该用户在此子空间中最早的记录来估计加入时间
		var earliestTimestamp int64
		if stats, exists := user.SubspaceStats[subspaceID]; exists {
			for _, timestamp := range stats {
				if earliestTimestamp == 0 || timestamp < uint64(earliestTimestamp) {
					earliestTimestamp = int64(timestamp)
				}
			}
		}

		// 如果没有找到记录，使用最后更新时间
		if earliestTimestamp == 0 {
			earliestTimestamp = user.LastUpdated
		}

		simpleUsers = append(simpleUsers, SimpleUserInfo{
			ID:             user.ID,
			JoinTime:       time.Unix(earliestTimestamp, 0),
			LastActiveTime: time.Unix(user.LastUpdated, 0),
		})
	}

	// 返回JSON数据
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(simpleUsers)
}

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
