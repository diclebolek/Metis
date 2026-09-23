package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var demoCoreUsers = []string{"alice", "bob", "carol", "dave", "eve"}

var demoExtraNames = []string{
	"ada", "ahmet", "asya", "atlas", "aylin", "baran", "berk", "bilge", "can", "cem",
	"deniz", "derin", "ece", "ege", "elif", "emir", "ebru", "ferhat", "gizem", "hande",
	"ilke", "irem", "kaan", "kaya", "lina", "mert", "mira", "naz", "nil", "onur",
	"oya", "poyraz", "rana", "sarp", "selin", "sera", "su", "talia", "umut", "yalın",
	"yeni", "zafer", "aslan", "bahar", "ceren", "doruk", "ela", "firat", "gokce", "hale",
	"idil", "jade", "kuzey", "lara", "melis", "nehir",
}

func seedDemoUsers(ctx context.Context, rdb *redis.Client) {
	pass := []byte("demo123")
	hash, err := bcrypt.GenerateFromPassword(pass, bcryptCost)
	if err != nil {
		log.Printf("seed users: %v", err)
		return
	}

	all := append([]string{}, demoCoreUsers...)
	all = append(all, demoExtraNames...)
	for _, user := range all {
		key := "user:" + user
		_ = rdb.HSet(ctx, key, map[string]interface{}{
			"password": string(hash),
			"demo":     "1",
		}).Err()
		_ = rdb.SAdd(ctx, usersSet, user).Err()
	}
	log.Printf("demo users ready (%d accounts, password: demo123)", len(all))
}

func seedDemoPresence(ctx context.Context, rdb *redis.Client) {
	rooms := []string{"general", "random", "dev"}
	all := append([]string{}, demoCoreUsers...)
	all = append(all, demoExtraNames...)
	args := toInterfaces(all)

	for _, room := range rooms {
		key := "presence:" + room
		if len(args) > 0 {
			_ = rdb.SAdd(ctx, key, args...).Err()
		}
		_ = rdb.Expire(ctx, key, 2*time.Minute).Err()
	}
}

func toInterfaces(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func refreshDemoPresenceLoop(ctx context.Context, rdb *redis.Client) {
	t := time.NewTicker(45 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			seedDemoPresence(ctx, rdb)
		}
	}
}

func seedMoreDemoNoise(ctx context.Context, rdb *redis.Client, now time.Time) {
	// a few extra messages from crowd users in general
	extras := []seedMsg{}
	for i := 0; i < 12 && i < len(demoExtraNames); i++ {
		extras = append(extras, seedMsg{
			User:    demoExtraNames[i],
			Content: fmt.Sprintf("Selam! Ben %s — demo kullanıcıyım 👋", demoExtraNames[i]),
			Type:    "chat",
			MinsAgo: 15 - i,
		})
	}
	if len(extras) == 0 {
		return
	}
	// append without wiping if history already has seed — only when first seed runs
	key := "history:general"
	n, _ := rdb.LLen(ctx, key).Result()
	if n > 20 {
		return
	}
	for i, m := range extras {
		ts := now.Add(-time.Duration(m.MinsAgo) * time.Minute).UnixMilli()
		payload := jsonMarshalChat(m.User, "general", m.Content, m.Type, ts, i+100)
		_ = rdb.LPush(ctx, key, payload).Err()
	}
}

func jsonMarshalChat(user, room, content, typ string, ts int64, i int) string {
	b, _ := json.Marshal(map[string]interface{}{
		"id":      fmt.Sprintf("seed-crowd-%d", i),
		"user":    user,
		"room":    room,
		"content": content,
		"type":    typ,
		"ts":      ts,
	})
	return string(b)
}
