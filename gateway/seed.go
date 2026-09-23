package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const demoChatSeedKey = "seed:demo_chat_v1"

type seedMsg struct {
	User    string
	Content string
	Type    string
	MinsAgo int
}

func seedDemoChats(ctx context.Context, rdb *redis.Client) {
	ok, err := rdb.SetNX(ctx, demoChatSeedKey, "1", 0).Result()
	if err != nil || !ok {
		return
	}

	now := time.Now()
	seedRoom(ctx, rdb, "general", []seedMsg{
		{"alice", "Merhaba ekip 👋 Metis'e hoş geldiniz", "chat", 90},
		{"bob", "Hey Alice! Go + Rust + Redis güzel duruyor.", "chat", 85},
		{"carol", "Presence ve typing da çalışıyor mu?", "chat", 80},
		{"dave", "Evet, az önce denedim. Rate limit de var.", "chat", 75},
		{"eve", "Dosya paylaşımı da eklenmiş, 📎 ile deneyin.", "chat", 70},
		{"alice", "Özel mesajlar için Kişiler listesinden birine tıklayın.", "chat", 60},
		{"bob", "Tamam, Carol'a DM atıyorum.", "chat", 55},
	}, now)

	seedRoom(ctx, rdb, "dev", []seedMsg{
		{"dave", "Worker stream'den okuyup Pub/Sub'a basıyor.", "chat", 120},
		{"alice", "Gateway sadece WebSocket + REST, doğru mu?", "chat", 110},
		{"eve", "Evet. Ağır iş Rust tarafında.", "chat", 100},
		{"carol", "Localhost:8080 üzerinden test ediyorum.", "chat", 50},
		{"bob", "PR açmadan önce README'deki demo hesapları kullanın.", "chat", 40},
	}, now)

	seedRoom(ctx, rdb, "random", []seedMsg{
		{"eve", "Kahve molası ☕", "chat", 200},
		{"dave", "Ben çaycı kampındayım.", "chat", 190},
		{"carol", "Bugün deploy yok, sadece demo.", "chat", 30},
		{"alice", "O zaman rahat mesajlaşalım 😄", "chat", 20},
	}, now)

	// Sample DMs (sorted peer names → room id)
	seedDM(ctx, rdb, "alice", "bob", []seedMsg{
		{"alice", "Bob, yarın standup saat kaç?", "chat", 45},
		{"bob", "10:30 — general'da buluşalım.", "chat", 40},
		{"alice", "Süper, not aldım.", "chat", 35},
		{"bob", "Bu arada demo seed mesajları da geldi 👍", "chat", 10},
	}, now)

	seedDM(ctx, rdb, "alice", "carol", []seedMsg{
		{"carol", "Alice, UI sidebar'ı çok daha iyi olmuş.", "chat", 25},
		{"alice", "Teşekkürler! Kart kutuları netleştirdi.", "chat", 20},
		{"carol", "Okundu tiklerini de beğendim ✓✓", "chat", 15},
	}, now)

	seedDM(ctx, rdb, "bob", "dave", []seedMsg{
		{"bob", "Redis Streams consumer group sorunsuz mu?", "chat", 55},
		{"dave", "Evet, worker XREADGROUP ile çekiyor.", "chat", 50},
		{"bob", "Rate limit aşınca system mesajı geliyor.", "chat", 48},
		{"dave", "Doğru — saniyede 10 mesaj default.", "chat", 12},
	}, now)

	seedDM(ctx, rdb, "carol", "eve", []seedMsg{
		{"eve", "Carol, test için bu sohbeti kullanabilirsin.", "chat", 18},
		{"carol", "Harika, teşekkürler Eve!", "chat", 8},
	}, now)

	seedMoreDemoNoise(ctx, rdb, now)
	log.Printf("demo conversations seeded (rooms + DMs)")
}

func seedDM(ctx context.Context, rdb *redis.Client, a, b string, msgs []seedMsg, now time.Time) {
	u1, u2 := a, b
	if u1 > u2 {
		u1, u2 = u2, u1
	}
	room := "dm:" + u1 + ":" + u2
	_ = rdb.SAdd(ctx, roomsSet, room).Err()
	_ = rdb.SAdd(ctx, "user:"+a+":dms", room).Err()
	_ = rdb.SAdd(ctx, "user:"+b+":dms", room).Err()
	seedRoom(ctx, rdb, room, msgs, now)
}

func seedRoom(ctx context.Context, rdb *redis.Client, room string, msgs []seedMsg, now time.Time) {
	_ = rdb.SAdd(ctx, roomsSet, room).Err()
	key := "history:" + room
	_ = rdb.Del(ctx, key).Err()
	for i, m := range msgs {
		ts := now.Add(-time.Duration(m.MinsAgo) * time.Minute).UnixMilli()
		msgType := m.Type
		if msgType == "" {
			msgType = "chat"
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"id":      fmt.Sprintf("seed-%s-%d", room, i),
			"user":    m.User,
			"room":    room,
			"content": m.Content,
			"type":    msgType,
			"ts":      ts,
		})
		// LPUSH in chronological order → newest ends at list head
		_ = rdb.LPush(ctx, key, string(payload)).Err()
	}
	_ = rdb.LTrim(ctx, key, 0, 99).Err()
}
