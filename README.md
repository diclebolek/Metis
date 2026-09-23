# denek

Real-time chat platform built with a **Go** WebSocket gateway, a **Rust** message worker, and **Redis** for Streams, Pub/Sub, presence, and rate limiting.

## Architecture

```
Client (browser)
   │  REST + WebSocket
   ▼
Go Gateway ──XADD──► Redis Stream (messages:inbound)
   ▲                         │
   │                         ▼
   └──── Pub/Sub ◄──── Rust Worker
        chat:room:*      rate limit · history · fan-out
```

| Component | Responsibility |
|-----------|----------------|
| `gateway/` | HTTP API, WebSocket hub, presence, typing relay |
| `worker/` | Consume inbound stream, enforce rate limits, publish + store history |
| `web/` | Lightweight chat UI |
| Redis | Queue, Pub/Sub, presence sets, message history |

## Features

- Room-based real-time messaging
- Online presence per room
- Typing indicators
- Per-user rate limiting
- Message history (last 100 messages / room)
- Default rooms: `general`, `random`, `dev`

## Requirements

- Go 1.22+
- Rust (stable toolchain + cargo)
- Docker (for Redis)

## Quick start

```bash
# 1. Start Redis
docker compose up -d

# 2. Start the Rust worker
cd worker && cargo run --release

# 3. Start the Go gateway (separate terminal)
cd gateway && go run .

# 4. Open the UI
# http://localhost:8080
```

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/rooms` | List rooms + online counts |
| POST | `/api/rooms` | Create room `{"name":"general"}` |
| GET | `/api/rooms/{name}/messages` | Recent messages |
| GET | `/api/rooms/{name}/presence` | Online users |
| WS | `/ws?user=alice&room=general` | Chat socket |

### WebSocket client payload

```json
{"type":"chat","content":"hello"}
{"type":"typing"}
```

## Configuration

| Variable | Default | Used by |
|----------|---------|---------|
| `REDIS_ADDR` | `127.0.0.1:6379` | gateway, worker |
| `HTTP_ADDR` | `:8080` | gateway |
| `RATE_LIMIT` | `10` (messages/sec) | worker |

Copy `.env.example` if you want a local reference file.

## License

MIT
