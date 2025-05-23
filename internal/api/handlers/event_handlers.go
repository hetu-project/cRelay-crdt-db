package handlers

import (
	"encoding/json"
	// "fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/nbd-wtf/go-nostr"

	"github.com/hetu-project/cRelay-crdt-db/internal/storage"
)

// EventHandlers handles event-related API requests
type EventHandlers struct {
	store storage.Store
}

// NewEventHandlers creates a new EventHandlers
func NewEventHandlers(store storage.Store) *EventHandlers {
	return &EventHandlers{store: store}
}

// SaveEvent handles event creation requests
func (h *EventHandlers) SaveEvent(w http.ResponseWriter, r *http.Request) {
	var event nostr.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.store.SaveEvent(r.Context(), &event); err != nil {
		http.Error(w, "Failed to save event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// GetEvent handles requests to get a single event
func (h *EventHandlers) GetEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID := vars["id"]

	filter := nostr.Filter{
		IDs: []string{eventID},
	}

	events := make([]*nostr.Event, 0)
	eventChan, err := h.store.QueryEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, "Failed to query event", http.StatusInternalServerError)
		return
	}

	for event := range eventChan {
		events = append(events, event)
	}

	if len(events) == 0 {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(events[0])
}

// QueryEvents handles requests to query multiple events
func (h *EventHandlers) QueryEvents(w http.ResponseWriter, r *http.Request) {
	// Use generic map to parse request for more flexible filtering conditions
	var queryParams map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&queryParams); err != nil {
		http.Error(w, "Invalid filter format", http.StatusBadRequest)
		return
	}

	// Build standard nostr filter
	filter := nostr.Filter{}

	// Handle standard filter fields
	if ids, ok := queryParams["ids"].([]interface{}); ok {
		for _, id := range ids {
			if idStr, ok := id.(string); ok {
				filter.IDs = append(filter.IDs, idStr)
			}
		}
	}

	if authors, ok := queryParams["authors"].([]interface{}); ok {
		for _, author := range authors {
			if authorStr, ok := author.(string); ok {
				filter.Authors = append(filter.Authors, authorStr)
			}
		}
	}

	if kinds, ok := queryParams["kinds"].([]interface{}); ok {
		for _, kind := range kinds {
			if kindFloat, ok := kind.(float64); ok {
				filter.Kinds = append(filter.Kinds, int(kindFloat))
			}
		}
	}

	if limit, ok := queryParams["limit"].(float64); ok {
		filter.Limit = int(limit)
	}

	// Handle time filtering
	if since, ok := queryParams["since"].(float64); ok {
		timestamp := nostr.Timestamp(since)
		filter.Since = &timestamp
	}
	if until, ok := queryParams["until"].(float64); ok {
		timestamp := nostr.Timestamp(until)
		filter.Until = &timestamp
	}

	// Special handling for custom tag filtering
	filter.Tags = make(nostr.TagMap)

	// Handle sid tag
	if sid, ok := queryParams["sid"].([]interface{}); ok && len(sid) > 0 {
		sidValues := make([]string, 0)
		for _, s := range sid {
			if sidStr, ok := s.(string); ok {
				sidValues = append(sidValues, sidStr)
			}
		}
		filter.Tags["sid"] = sidValues
	}

	// Handle parent tag
	if parent, ok := queryParams["parent"].([]interface{}); ok && len(parent) > 0 {
		parentValues := make([]string, 0)
		for _, s := range parent {
			if parentStr, ok := s.(string); ok {
				parentValues = append(parentValues, parentStr)
			}
		}
		filter.Tags["parent"] = parentValues
	}

	limit := 100 // Default limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if filter.Limit == 0 || filter.Limit > limit {
		filter.Limit = limit
	}

	events := make([]*nostr.Event, 0)
	eventChan, err := h.store.QueryEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, "Failed to query events", http.StatusInternalServerError)
		return
	}

	count := 0
	for event := range eventChan {
		if count >= filter.Limit {
			break
		}
		events = append(events, event)
		count++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// DeleteEvent handles event deletion requests
func (h *EventHandlers) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID := vars["id"]

	filter := nostr.Filter{
		IDs: []string{eventID},
	}

	events := make([]*nostr.Event, 0)
	eventChan, err := h.store.QueryEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, "Failed to query event", http.StatusInternalServerError)
		return
	}

	for event := range eventChan {
		events = append(events, event)
	}

	if len(events) == 0 {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	if err := h.store.DeleteEvent(r.Context(), events[0]); err != nil {
		http.Error(w, "Failed to delete event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// func (h *EventHandlers) GetSubspace(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)
// 	id := vars["id"]

// 	// 验证子空间ID格式
// 	// if !isValidSubspaceID(id) {
// 	// 	http.Error(w, "无效的子空间ID格式", http.StatusBadRequest)
// 	// 	return
// 	// }

// 	// 获取子空间因果关系数据
// 	causality, err := h.store.GetSubspaceCausality(r.Context(), id)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("获取子空间数据失败: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	if causality == nil {
// 		http.Error(w, "子空间不存在", http.StatusNotFound)
// 		return
// 	}

// 	// 返回JSON数据
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(causality)
// }

// // GetSubspaceEvents 获取子空间事件
// func (h *EventHandlers) GetSubspaceEvents(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)
// 	id := vars["id"]

// 	// 验证子空间ID格式
// 	// if !isValidSubspaceID(id) {
// 	// 	http.Error(w, "无效的子空间ID格式", http.StatusBadRequest)
// 	// 	return
// 	// }

// 	// 查询子空间事件
// 	eventCh, err := h.store.QuerySubspaceEvents(r.Context(), id)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("查询子空间事件失败: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// 收集所有事件
// 	var events []*nostr.Event
// 	for event := range eventCh {
// 		events = append(events, event)
// 	}

// 	// 返回JSON数据
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(events)
// }

// // ListSubspaces 列出所有子空间
// func (api *CausalityAPI) ListSubspaces(w http.ResponseWriter, r *http.Request) {
// 	// 获取查询参数
// 	query := r.URL.Query()
// 	sinceStr := query.Get("since")
// 	untilStr := query.Get("until")

// 	// 解析时间范围
// 	var since, until *int64
// 	if sinceStr != "" {
// 		var sinceVal int64
// 		if _, err := fmt.Sscanf(sinceStr, "%d", &sinceVal); err == nil {
// 			since = &sinceVal
// 		}
// 	}

// 	if untilStr != "" {
// 		var untilVal int64
// 		if _, err := fmt.Sscanf(untilStr, "%d", &untilVal); err == nil {
// 			until = &untilVal
// 		}
// 	}

// 	// 创建过滤器函数
// 	filter := func(c *SubspaceCausality) bool {
// 		if since != nil && c.Updated < *since {
// 			return false
// 		}
// 		if until != nil && c.Updated > *until {
// 			return false
// 		}
// 		return true
// 	}

// 	// 查询子空间
// 	subspaces, err := api.causalityMgr.QuerySubspaces(r.Context(), filter)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("查询子空间失败: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	// 返回JSON数据
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(subspaces)
// }

// // CreateSubspaceEvent 创建一个子空间事件
// func (api *CausalityAPI) CreateSubspaceEvent(subspaceID string, pubkey string, keyID string, counter uint64) (*nostr.Event, error) {
// 	// 验证子空间ID
// 	if !isValidSubspaceID(subspaceID) {
// 		return nil, fmt.Errorf("无效的子空间ID格式")
// 	}

// 	// 创建事件
// 	event := &nostr.Event{
// 		PubKey:    pubkey,
// 		CreatedAt: nostr.Now(),
// 		Kind:      30304, // 子空间操作的自定义事件类型
// 		Tags: []nostr.Tag{
// 			{"d", "subspace_op"},
// 			{"sid", subspaceID},
// 			{"causal", keyID, fmt.Sprintf("%d", counter)},
// 		},
// 		Content: "", // 内容可以根据需要设置
// 	}

// 	// 计算事件ID
// 	err := event.Sign() // 注意：在实际使用中，应该由客户端签署
// 	if err != nil {
// 		return nil, err
// 	}

// 	// 保存事件
// 	ctx := context.Background()
// 	err = api.adapter.SaveEvent(ctx, event)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return event, nil
// }

// // AddCausalityRoutesToRouter 添加因果关系路由到主路由器
// func AddCausalityRoutesToRouter(router *mux.Router, adapter *OrbitDBAdapter) {
// 	api := NewCausalityAPI(adapter)
// 	if api != nil {
// 		api.RegisterRoutes(router)
// 	}
// }
