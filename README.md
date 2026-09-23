# Metis

Gerçek zamanlı sohbet — **Go** WebSocket gateway, **Rust** mesaj işçisi, **Redis** (Streams · Pub/Sub · oturum · varlık · hız sınırı).

Ayrı bir SQL veritabanı yok: Redis bu projenin kaynak gerçeğidir. Kalıcı arşiv sohbetlerinden farklı olarak **süreli odalar** Redis TTL ile silinir.

```
Tarayıcı
  │  REST + WebSocket
  ▼
Go Gateway ──XADD──► Redis Stream (messages:inbound)
  │  oturum · varlık · yükleme · engel/sessize
  ▲                         │
  │                         ▼
  └──── Pub/Sub ◄──── Rust Worker
       chat:room:*      hız sınırı · geçmiş · dağıtım
```

## Bileşenler

| Yol | Görev |
|-----|--------|
| `gateway/` | Auth, REST, WebSocket hub, yüklemeler, varlık, profil, özellik API’leri |
| `worker/` | Inbound stream tüketimi, hız sınırı, geçmiş + oda Pub/Sub |
| `web/` | Metis arayüzü (PWA, split panel, araç balonu, ayarlar) |
| `uploads/` | Yerel dosya/görseller (gateway sunar) |
| `docker-compose.yml` | Redis |

## Özellikler

### Ayırt edici
- **Süreli odalar** — 2 saat / gece yarısı / 24 saat; TTL bitince oda + geçmiş silinir (toplantı / etkinlik türleri)
- **Kaybolan mesaj** — okununca veya 24 saat sonra (oda TTL’sinden bağımsız)
- **Alıntı → sağ panel** — soldaki mesaja tıklayınca sağdaki DM açık kalır, alıntı oraya gider
- **Odak durumu** — bu odada / yazıyor / meşgul; kişi listesinde oda görünür
- **Hız sınırı geri bildirimi** — sessiz düşme yok; “X saniye sonra gönderilebilir”

### Sohbet
- Odalar + DM, **split pane**
- Emoji, sticker, GIF, çoklu fotoğraf / albüm
- Yanıt (quote), pin, tepki, düzenle / sil
- Okunmamış sayacı, global arama
- Davet linki, WebRTC sinyal (çağrı daveti)
- Tema (açık/koyu), PWA + service worker
- Kendi profilinden **ayarlar** (tema, odak, ses, bildirim, çıkış)

## Hızlı başlangıç

Gereksinimler: Docker (Redis), Go 1.21+, Rust (stable), modern tarayıcı.

```bash
# 1) Redis
docker compose up -d

# 2) Worker (ayrı terminal)
cd worker
cargo run --release

# 3) Gateway (ayrı terminal)
cd gateway
go run .

# → http://localhost:8080
```

Demo: `alice` / `demo123` (bob, carol, … ~60 örnek kullanıcı).

Ortam değişkenleri için `.env.example` dosyasına bakın (`REDIS_ADDR`, `HTTP_ADDR`, `RATE_LIMIT`, `UPLOAD_DIR`).

## API özeti

| Yöntem | Yol | Açıklama |
|--------|-----|----------|
| POST | `/api/auth/register` · `/api/auth/login` | Kayıt / giriş |
| GET | `/api/me` · `/api/users` | Oturum · kullanıcı listesi (+ odak) |
| GET/POST | `/api/rooms` | Oda listesi / oluştur (`ttl`, `kind`) |
| GET | `/api/rooms/{room}/messages` | Geçmiş |
| POST | `/api/dm` | DM aç |
| POST | `/api/upload` | Dosya (çoklu albüm istemci tarafında) |
| POST | `/api/focus` | Odak durumu |
| GET | `/api/search?q=` | Global arama |
| POST/GET | `/api/invite` | Davet kodu |
| WS | `/ws?token=&room=` | Canlı oda |

## Geliştirme notları

- Worker **yalnızca std**: MinGW bağlayıcı bağımlılığı yok.
- Hız sınırı varsayılanı `RATE_LIMIT` (ör. 10/sn); aşımda `type: rate_limit` + `retry_after`.
- Süreli oda meta: `roommeta:{name}`; süpürücü Redis anahtarlarını temizler.
- Üretimde: TLS, güçlü şifreler, Redis auth, upload boyutu/CORS gözden geçirin.

## Lisans

MIT
