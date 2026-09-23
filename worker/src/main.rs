//! Redis Streams consumer + Pub/Sub fan-out worker (std only).
//! Inbound: XREADGROUP on `messages:inbound`
//! Outbound: PUBLISH `chat:room:{room}` + history list + rate limit

use std::collections::HashMap;
use std::env;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::time::{SystemTime, UNIX_EPOCH};

const INBOUND_STREAM: &str = "messages:inbound";
const CONSUMER_GROUP: &str = "workers";
const CONSUMER_NAME: &str = "worker-1";

fn main() {
    let addr = env::var("REDIS_ADDR").unwrap_or_else(|_| "127.0.0.1:6379".into());
    let rate_limit: i64 = env::var("RATE_LIMIT")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(10);

    let mut redis = Redis::connect(&addr).unwrap_or_else(|e| {
        eprintln!("redis connect failed ({addr}): {e}");
        std::process::exit(1);
    });

    redis.ensure_group().unwrap_or_else(|e| {
        eprintln!("xgroup create failed: {e}");
        std::process::exit(1);
    });

    println!("worker ready (redis={addr}, rate_limit={rate_limit}/s)");

    loop {
        match redis.xread_group(10, 2000) {
            Ok(entries) => {
                for entry in entries {
                    if let Err(e) = handle_entry(&mut redis, &entry, rate_limit) {
                        eprintln!("handle {}: {e}", entry.id);
                    }
                    if let Err(e) = redis.xack(&entry.id) {
                        eprintln!("xack {}: {e}", entry.id);
                    }
                }
            }
            Err(e) => {
                eprintln!("xread: {e}");
                std::thread::sleep(std::time::Duration::from_millis(500));
            }
        }
    }
}

struct StreamEntry {
    id: String,
    fields: HashMap<String, String>,
}

fn handle_entry(redis: &mut Redis, entry: &StreamEntry, rate_limit: i64) -> Result<(), String> {
    let user = entry.fields.get("user").cloned().unwrap_or_default();
    let room = entry.fields.get("room").cloned().unwrap_or_default();
    let content = entry.fields.get("content").cloned().unwrap_or_default();
    let meta = entry.fields.get("meta").cloned().unwrap_or_default();
    let msg_type = entry
        .fields
        .get("type")
        .cloned()
        .filter(|s| !s.is_empty())
        .unwrap_or_else(|| "chat".into());
    let ephemeral = entry
        .fields
        .get("ephemeral")
        .cloned()
        .unwrap_or_default();
    let reply_to = entry.fields.get("reply_to").cloned().unwrap_or_default();
    let reply_content = entry
        .fields
        .get("reply_content")
        .cloned()
        .unwrap_or_default();
    let reply_user = entry.fields.get("reply_user").cloned().unwrap_or_default();
    let ts = entry
        .fields
        .get("ts")
        .and_then(|s| s.parse().ok())
        .unwrap_or_else(now_ms);

    if user.is_empty() || room.is_empty() || content.is_empty() {
        return Ok(());
    }

    if !redis.allow_rate(&user, rate_limit)? {
        let key = format!("ratelimit:{user}");
        let mut retry = redis.ttl(&key)?.max(1);
        if retry < 1 {
            retry = 1;
        }
        let payload = format!(
            "{{\"id\":\"{}\",\"user\":\"{}\",\"room\":\"{}\",\"content\":\"\",\"type\":\"rate_limit\",\"retry_after\":{},\"ts\":{}}}",
            escape(&new_id()),
            escape(&user),
            escape(&room),
            retry,
            now_ms()
        );
        redis.publish(&format!("chat:room:{room}"), &payload)?;
        redis.publish(&format!("chat:user:{user}"), &payload)?;
        return Ok(());
    }

    let id = new_id();
    let expire_at = if ephemeral == "24h" {
        now_ms() + 24 * 3600 * 1000
    } else {
        0
    };
    let payload = json_msg_full(
        &id,
        &user,
        &room,
        &content,
        &msg_type,
        ts,
        &meta,
        &ephemeral,
        expire_at,
        &reply_to,
        &reply_content,
        &reply_user,
    );
    let history_key = format!("history:{room}");
    redis.lpush(&history_key, &payload)?;
    redis.ltrim(&history_key, 0, 99)?;
    redis.sadd("chat:rooms", &room)?;
    redis.publish(&format!("chat:room:{room}"), &payload)?;
    println!("fan-out room={room} user={user} type={msg_type}");
    Ok(())
}

fn json_msg_full(
    id: &str,
    user: &str,
    room: &str,
    content: &str,
    msg_type: &str,
    ts: i64,
    meta: &str,
    ephemeral: &str,
    expire_at: i64,
    reply_to: &str,
    reply_content: &str,
    reply_user: &str,
) -> String {
    let mut s = format!(
        "{{\"id\":\"{}\",\"user\":\"{}\",\"room\":\"{}\",\"content\":\"{}\",\"type\":\"{}\",\"ts\":{}",
        escape(id),
        escape(user),
        escape(room),
        escape(content),
        escape(msg_type),
        ts
    );
    if !meta.is_empty() {
        s.push_str(&format!(",\"meta\":\"{}\"", escape(meta)));
    }
    if !ephemeral.is_empty() {
        s.push_str(&format!(",\"ephemeral\":\"{}\"", escape(ephemeral)));
    }
    if expire_at > 0 {
        s.push_str(&format!(",\"expire_at\":{expire_at}"));
    }
    if !reply_to.is_empty() {
        s.push_str(&format!(
            ",\"reply_to\":\"{}\",\"reply_content\":\"{}\",\"reply_user\":\"{}\"",
            escape(reply_to),
            escape(reply_content),
            escape(reply_user)
        ));
    }
    s.push('}');
    s
}

fn escape(s: &str) -> String {
    s.replace('\\', "\\\\")
        .replace('"', "\\\"")
        .replace('\n', "\\n")
        .replace('\r', "\\r")
        .replace('\t', "\\t")
}

fn now_ms() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_millis() as i64
}

fn new_id() -> String {
    format!("{:x}-{:x}", now_ms(), std::process::id())
}

struct Redis {
    stream: TcpStream,
    buf: Vec<u8>,
}

impl Redis {
    fn connect(addr: &str) -> Result<Self, String> {
        let stream = TcpStream::connect(addr).map_err(|e| e.to_string())?;
        stream
            .set_read_timeout(Some(std::time::Duration::from_secs(5)))
            .ok();
        Ok(Self {
            stream,
            buf: Vec::new(),
        })
    }

    fn ensure_group(&mut self) -> Result<(), String> {
        match self.cmd(&[
            "XGROUP",
            "CREATE",
            INBOUND_STREAM,
            CONSUMER_GROUP,
            "0",
            "MKSTREAM",
        ]) {
            Ok(_) => Ok(()),
            Err(e) if e.contains("BUSYGROUP") => Ok(()),
            Err(e) => Err(e),
        }
    }

    fn xread_group(&mut self, count: i64, block_ms: i64) -> Result<Vec<StreamEntry>, String> {
        self.stream
            .set_read_timeout(Some(std::time::Duration::from_millis(
                (block_ms as u64) + 1000,
            )))
            .ok();

        let count_s = count.to_string();
        let block_s = block_ms.to_string();
        let reply = self.cmd(&[
            "XREADGROUP",
            "GROUP",
            CONSUMER_GROUP,
            CONSUMER_NAME,
            "COUNT",
            &count_s,
            "BLOCK",
            &block_s,
            "STREAMS",
            INBOUND_STREAM,
            ">",
        ]);

        self.stream
            .set_read_timeout(Some(std::time::Duration::from_secs(5)))
            .ok();

        match reply {
            Ok(Resp::Null) => Ok(vec![]),
            Ok(Resp::Array(streams)) => parse_xread(streams),
            Ok(_) => Ok(vec![]),
            Err(e) if e.contains("timed out") || e.contains("os error 10060") => Ok(vec![]),
            Err(e) => Err(e),
        }
    }

    fn xack(&mut self, id: &str) -> Result<(), String> {
        self.cmd(&["XACK", INBOUND_STREAM, CONSUMER_GROUP, id])?;
        Ok(())
    }

    fn publish(&mut self, channel: &str, payload: &str) -> Result<(), String> {
        self.cmd(&["PUBLISH", channel, payload])?;
        Ok(())
    }

    fn lpush(&mut self, key: &str, value: &str) -> Result<(), String> {
        self.cmd(&["LPUSH", key, value])?;
        Ok(())
    }

    fn ltrim(&mut self, key: &str, start: i64, stop: i64) -> Result<(), String> {
        let s = start.to_string();
        let e = stop.to_string();
        self.cmd(&["LTRIM", key, &s, &e])?;
        Ok(())
    }

    fn sadd(&mut self, key: &str, member: &str) -> Result<(), String> {
        self.cmd(&["SADD", key, member])?;
        Ok(())
    }

    fn allow_rate(&mut self, user: &str, limit: i64) -> Result<bool, String> {
        let key = format!("ratelimit:{user}");
        let count = match self.cmd(&["INCR", &key])? {
            Resp::Int(n) => n,
            other => return Err(format!("unexpected INCR reply: {other:?}")),
        };
        if count == 1 {
            let _ = self.cmd(&["EXPIRE", &key, "1"]);
        }
        Ok(count <= limit)
    }

    fn ttl(&mut self, key: &str) -> Result<i64, String> {
        match self.cmd(&["TTL", key])? {
            Resp::Int(n) => Ok(n),
            other => Err(format!("unexpected TTL reply: {other:?}")),
        }
    }

    fn cmd(&mut self, args: &[&str]) -> Result<Resp, String> {
        let mut out = format!("*{}\r\n", args.len());
        for a in args {
            out.push_str(&format!("${}\r\n{}\r\n", a.len(), a));
        }
        self.stream
            .write_all(out.as_bytes())
            .map_err(|e| e.to_string())?;
        self.read_resp()
    }

    fn read_resp(&mut self) -> Result<Resp, String> {
        let line = self.read_line()?;
        if line.is_empty() {
            return Err("empty reply".into());
        }
        let (tag, rest) = line.split_at(1);
        match tag {
            "+" => Ok(Resp::Simple(rest.to_string())),
            "-" => Err(rest.to_string()),
            ":" => Ok(Resp::Int(rest.parse().unwrap_or(0))),
            "$" => {
                let n: i64 = rest.parse().unwrap_or(-1);
                if n < 0 {
                    return Ok(Resp::Null);
                }
                let data = self.read_exact(n as usize)?;
                let _ = self.read_line()?;
                Ok(Resp::Bulk(String::from_utf8_lossy(&data).into_owned()))
            }
            "*" => {
                let n: i64 = rest.parse().unwrap_or(-1);
                if n < 0 {
                    return Ok(Resp::Null);
                }
                let mut items = Vec::with_capacity(n as usize);
                for _ in 0..n {
                    items.push(self.read_resp()?);
                }
                Ok(Resp::Array(items))
            }
            other => Err(format!("unknown RESP type: {other}")),
        }
    }

    fn read_line(&mut self) -> Result<String, String> {
        loop {
            if let Some(pos) = self.buf.windows(2).position(|w| w == b"\r\n") {
                let line = String::from_utf8_lossy(&self.buf[..pos]).into_owned();
                self.buf.drain(..pos + 2);
                return Ok(line);
            }
            let mut tmp = [0u8; 4096];
            let n = self.stream.read(&mut tmp).map_err(|e| e.to_string())?;
            if n == 0 {
                return Err("connection closed".into());
            }
            self.buf.extend_from_slice(&tmp[..n]);
        }
    }

    fn read_exact(&mut self, n: usize) -> Result<Vec<u8>, String> {
        while self.buf.len() < n {
            let mut tmp = [0u8; 4096];
            let read = self.stream.read(&mut tmp).map_err(|e| e.to_string())?;
            if read == 0 {
                return Err("connection closed".into());
            }
            self.buf.extend_from_slice(&tmp[..read]);
        }
        Ok(self.buf.drain(..n).collect())
    }
}

#[derive(Debug)]
enum Resp {
    Simple(String),
    Bulk(String),
    Int(i64),
    Array(Vec<Resp>),
    Null,
}

fn parse_xread(streams: Vec<Resp>) -> Result<Vec<StreamEntry>, String> {
    let mut out = Vec::new();
    for stream in streams {
        let Resp::Array(parts) = stream else {
            continue;
        };
        if parts.len() < 2 {
            continue;
        }
        let Resp::Array(entries) = &parts[1] else {
            continue;
        };
        for entry in entries {
            let Resp::Array(ev) = entry else {
                continue;
            };
            if ev.len() < 2 {
                continue;
            }
            let id = match &ev[0] {
                Resp::Bulk(s) | Resp::Simple(s) => s.clone(),
                _ => continue,
            };
            let mut fields = HashMap::new();
            if let Resp::Array(kv) = &ev[1] {
                let mut i = 0;
                while i + 1 < kv.len() {
                    let k = match &kv[i] {
                        Resp::Bulk(s) | Resp::Simple(s) => s.clone(),
                        _ => {
                            i += 2;
                            continue;
                        }
                    };
                    let v = match &kv[i + 1] {
                        Resp::Bulk(s) | Resp::Simple(s) => s.clone(),
                        Resp::Int(n) => n.to_string(),
                        _ => String::new(),
                    };
                    fields.insert(k, v);
                    i += 2;
                }
            }
            out.push(StreamEntry { id, fields });
        }
    }
    Ok(out)
}
