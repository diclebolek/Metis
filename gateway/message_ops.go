package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/redis/go-redis/v9"
)

func (h *Hub) findHistoryIndex(ctx context.Context, room, id string) (int64, map[string]interface{}, error) {
	items, err := h.redis.LRange(ctx, "history:"+room, 0, -1).Result()
	if err != nil {
		return -1, nil, err
	}
	for i, raw := range items {
		var m map[string]interface{}
		if json.Unmarshal([]byte(raw), &m) != nil {
			continue
		}
		if str(m["id"]) == id {
			return int64(i), m, nil
		}
	}
	return -1, nil, redis.Nil
}

func (h *Hub) saveHistoryAt(ctx context.Context, room string, index int64, m map[string]interface{}) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return h.redis.LSet(ctx, "history:"+room, index, string(raw)).Err()
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

func (h *Hub) handleEditMessage(ctx context.Context, user, room, mid, content string) bool {
	content = strings.TrimSpace(content)
	if mid == "" || content == "" || len(content) > 2000 {
		return false
	}
	idx, m, err := h.findHistoryIndex(ctx, room, mid)
	if err != nil || m == nil {
		return false
	}
	if str(m["user"]) != user {
		return false
	}
	t := str(m["type"])
	if t != "chat" && t != "" {
		return false
	}
	m["content"] = content
	m["edited"] = true
	if err := h.saveHistoryAt(ctx, room, idx, m); err != nil {
		return false
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"type":       "edit",
		"message_id": mid,
		"content":    content,
		"user":       user,
		"room":       room,
		"edited":     true,
	})
	_ = h.redis.Publish(ctx, "chat:room:"+room, payload).Err()
	return true
}

func (h *Hub) handleUnsendMessage(ctx context.Context, user, room, mid string) bool {
	if mid == "" {
		return false
	}
	idx, m, err := h.findHistoryIndex(ctx, room, mid)
	if err != nil || m == nil {
		return false
	}
	if str(m["user"]) != user {
		return false
	}
	m["type"] = "unsent"
	m["content"] = ""
	m["meta"] = ""
	if err := h.saveHistoryAt(ctx, room, idx, m); err != nil {
		return false
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"type":       "unsend",
		"message_id": mid,
		"user":       user,
		"room":       room,
	})
	_ = h.redis.Publish(ctx, "chat:room:"+room, payload).Err()
	return true
}
