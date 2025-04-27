package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/nbd-wtf/go-nostr"

	"github.com/hetu-project/cRelay-crdt-db/internal/storage"
)

type EventHandlers struct {
	store storage.Store
}

func NewEventHandlers(store storage.Store) *EventHandlers {
	return &EventHandlers{store: store}
}

// SaveEvent 处理创建事件的请求
func (h *EventHandlers) SaveEvent(w http.ResponseWriter, r *http.Request) {
	var event nostr.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if err := h.store.SaveEvent(r.Context(), &event); err != nil {
		http.Error(w, "保存事件失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// GetEvent 处理获取单个事件的请求
func (h *EventHandlers) GetEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID := vars["id"]

	filter := nostr.Filter{
		IDs: []string{eventID},
	}

	events := make([]*nostr.Event, 0)
	eventChan, err := h.store.QueryEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, "查询事件失败", http.StatusInternalServerError)
		return
	}

	for event := range eventChan {
		events = append(events, event)
	}

	if len(events) == 0 {
		http.Error(w, "事件未找到", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(events[0])
}

// QueryEvents 处理查询多个事件的请求
func (h *EventHandlers) QueryEvents(w http.ResponseWriter, r *http.Request) {
	var filter nostr.Filter
	if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
		http.Error(w, "无效的过滤器", http.StatusBadRequest)
		return
	}

	limit := 100 // 默认限制
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	events := make([]*nostr.Event, 0)
	eventChan, err := h.store.QueryEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, "查询事件失败", http.StatusInternalServerError)
		return
	}

	count := 0
	for event := range eventChan {
		if count >= limit {
			break
		}
		events = append(events, event)
		count++
	}

	json.NewEncoder(w).Encode(events)
}

// DeleteEvent 处理删除事件的请求
func (h *EventHandlers) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID := vars["id"]

	filter := nostr.Filter{
		IDs: []string{eventID},
	}

	events := make([]*nostr.Event, 0)
	eventChan, err := h.store.QueryEvents(r.Context(), filter)
	if err != nil {
		http.Error(w, "查询事件失败", http.StatusInternalServerError)
		return
	}

	for event := range eventChan {
		events = append(events, event)
	}

	if len(events) == 0 {
		http.Error(w, "事件未找到", http.StatusNotFound)
		return
	}

	if err := h.store.DeleteEvent(r.Context(), events[0]); err != nil {
		http.Error(w, "删除事件失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
