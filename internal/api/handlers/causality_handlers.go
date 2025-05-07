package handlers

import (
	// "context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/nbd-wtf/go-nostr"

	"github.com/hetu-project/cRelay-crdt-db/internal/storage"
	"github.com/hetu-project/cRelay-crdt-db/orbitdb"
)

// CausalityHandlers 处理因果关系相关的HTTP请求
type CausalityHandlers struct {
	store storage.Store
}

// NewCausalityHandlers 创建因果关系处理程序
func NewCausalityHandlers(store storage.Store) *CausalityHandlers {
	return &CausalityHandlers{
		store: store,
	}
}

// GetSubspaceCausality 处理获取子空间因果关系的请求
func (h *CausalityHandlers) GetSubspaceCausality(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subspaceID := vars["id"]

	// 获取子空间的因果关系
	causality, err := h.store.GetSubspaceCausality(r.Context(), subspaceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取子空间因果关系失败: %v", err), http.StatusInternalServerError)
		return
	}

	if causality == nil {
		http.Error(w, "子空间不存在", http.StatusNotFound)
		return
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(causality)
}

// GetCausalityKey 处理获取特定因果关系键的请求
func (h *CausalityHandlers) GetCausalityKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subspaceID := vars["id"]
	keyIDStr := vars["key"]

	// 将键ID转换为uint32
	keyID, err := strconv.ParseUint(keyIDStr, 10, 32)
	if err != nil {
		http.Error(w, "无效的键ID", http.StatusBadRequest)
		return
	}

	// 获取因果关系键的计数器值
	counter, err := h.store.GetCausalityKey(r.Context(), subspaceID, uint32(keyID))
	if err != nil {
		http.Error(w, fmt.Sprintf("获取因果关系键失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"subspace_id": subspaceID,
		"key":         keyID,
		"counter":     counter,
	})
}

// GetSubspaceEvents 处理获取子空间事件的请求
func (h *CausalityHandlers) GetSubspaceEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subspaceID := vars["id"]

	// 获取子空间事件ID列表
	eventIDs, err := h.store.GetCausalityEvents(r.Context(), subspaceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取子空间事件失败: %v", err), http.StatusInternalServerError)
		return
	}

	if len(eventIDs) == 0 {
		// 返回空数组
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	// 创建过滤器查询这些事件
	filter := nostr.Filter{
		IDs: eventIDs,
	}

	// 限制返回的事件数量
	limit := 100 // 默认限制
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// 查询事件
	events := make([]*nostr.Event, 0)
	eventChan, err := h.store.QueryEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, fmt.Sprintf("查询事件失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 收集事件
	count := 0
	for event := range eventChan {
		if count >= limit {
			break
		}
		events = append(events, event)
		count++
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// ListSubspaces 处理列出所有子空间的请求
func (h *CausalityHandlers) ListSubspaces(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	query := r.URL.Query()
	sinceStr := query.Get("since")
	untilStr := query.Get("until")

	// 解析时间范围
	var since, until *int64
	if sinceStr != "" {
		sinceVal, err := strconv.ParseInt(sinceStr, 10, 64)
		if err == nil {
			since = &sinceVal
		}
	}

	if untilStr != "" {
		untilVal, err := strconv.ParseInt(untilStr, 10, 64)
		if err == nil {
			until = &untilVal
		}
	}

	// 创建过滤器函数
	filter := func(c *orbitdb.SubspaceCausality) bool {
		if since != nil && c.Updated < *since {
			return false
		}
		if until != nil && c.Updated > *until {
			return false
		}
		return true
	}

	// 查询子空间
	subspaces, err := h.store.QuerySubspaces(r.Context(), filter)
	if err != nil {
		http.Error(w, fmt.Sprintf("查询子空间失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subspaces)
}

// CreateSubspaceEvent 创建一个子空间事件
// func (h *CausalityHandlers) CreateSubspaceEvent(w http.ResponseWriter, r *http.Request) {
// 	// 解析请求体
// 	var requestData struct {
// 		SubspaceID string `json:"subspace_id"`
// 		PubKey     string `json:"pubkey"`
// 		KeyID      uint32 `json:"key_id"`
// 		Content    string `json:"content,omitempty"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
// 		http.Error(w, "无效的请求体", http.StatusBadRequest)
// 		return
// 	}

// 	// 验证子空间ID
// 	if !orbitdb.IsValidSubspaceID(requestData.SubspaceID) {
// 		http.Error(w, "无效的子空间ID格式", http.StatusBadRequest)
// 		return
// 	}

// 	// 获取当前的计数器值
// 	counter, err := h.store.GetCausalityKey(r.Context(), requestData.SubspaceID, requestData.KeyID)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("获取因果关系键失败: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// 递增计数器
// 	newCounter := counter + 1

// 	// 创建事件
// 	event := &nostr.Event{
// 		PubKey:    requestData.PubKey,
// 		CreatedAt: nostr.Now(),
// 		Kind:      int(requestData.KeyID), // 使用KeyID作为事件Kind
// 		Tags: []nostr.Tag{
// 			{"d", "subspace_op"},
// 			{"sid", requestData.SubspaceID},
// 			{"causal", fmt.Sprintf("%d", requestData.KeyID), fmt.Sprintf("%d", newCounter)},
// 		},
// 		Content: requestData.Content,
// 	}

// 	// 计算事件ID
// 	err = event.Sign() // 注意：在实际使用中，应该由客户端签署
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("签署事件失败: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// 保存事件
// 	err = h.store.SaveEvent(r.Context(), event)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("保存事件失败: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// 返回JSON响应
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusCreated)
// 	json.NewEncoder(w).Encode(event)
// }
