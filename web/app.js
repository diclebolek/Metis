(() => {
  const $ = (id) => document.getElementById(id);
  const store = {
    get token() { return localStorage.getItem("metis_token") || localStorage.getItem("denek_token") || ""; },
    set token(v) { localStorage.setItem("metis_token", v); localStorage.removeItem("denek_token"); },
    get user() { return localStorage.getItem("metis_user") || localStorage.getItem("denek_user") || ""; },
    set user(v) { localStorage.setItem("metis_user", v); localStorage.removeItem("denek_user"); },
    clear() {
      localStorage.removeItem("metis_token"); localStorage.removeItem("metis_user");
      localStorage.removeItem("denek_token"); localStorage.removeItem("denek_user");
    },
  };

  const EMOJIS = "😀😁😂🤣😊😍🤩😘😜🤔😎😭😡👍👎👏🙏🔥✨🎉❤️💙💚💜🧡💯✅❌⭐🚀💻☕🍕🎮🎵🌈🦄🐱🐶".split("");
  const GIFS = [
    { t: "wave", u: "https://media.giphy.com/media/3o7abKhOpu0NwenH3O/giphy.gif" },
    { t: "party", u: "https://media.giphy.com/media/l0MYt5jPR6QX5pnqM/giphy.gif" },
    { t: "yes", u: "https://media.giphy.com/media/111ebonMs90YLu/giphy.gif" },
    { t: "no", u: "https://media.giphy.com/media/l2JehQ2LutLiQFAAw/giphy.gif" },
    { t: "lol", u: "https://media.giphy.com/media/10JhviFuU2cGn1/giphy.gif" },
    { t: "clap", u: "https://media.giphy.com/media/7rj2Zg2X0b3YI/giphy.gif" },
    { t: "wow", u: "https://media.giphy.com/media/3ohs7KViF6rA4ZzJbO/giphy.gif" },
    { t: "coffee", u: "https://media.giphy.com/media/3o6Zt481isNVuQI1l6/giphy.gif" },
    { t: "code", u: "https://media.giphy.com/media/13HgwGsXF0aiGY/giphy.gif" },
    { t: "ship", u: "https://media.giphy.com/media/xUPGcguWZHRC2HyBRS/giphy.gif" },
  ];
  const REACT_SET = ["👍", "❤️", "😂", "🔥", "🎉", "👀"];
  const STICKERS = ["🌟", "💫", "🦋", "🦊", "🐱", "🌙", "🌊", "🍀", "🎯", "🧩", "🪄", "🧿"];
  const FOCUS_LABEL = { here: "bu odada", typing: "yazıyor", busy: "meşgul", away: "uzakta" };
  const ICONS = {
    reply: '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M9 14L4 9l5-5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M4 9h9a7 7 0 0 1 7 7v2" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    pin: '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 17v5M9 3l.6 2.5L7 9l3 1.5L9.5 16H14.5L14 10.5 17 9l-2.6-3.5L15 3H9z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    edit: '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    trash: '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M8 7l.8 12.2A1 1 0 0 0 9.8 20h4.4a1 1 0 0 0 1-.8L16 7M10 11v6M14 11v6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    unsend: '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M3 10h11a5 5 0 0 1 0 10H9" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/><path d="M7 6L3 10l4 4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    copy: '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><rect x="9" y="9" width="11" height="11" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M5 15V5a2 2 0 0 1 2-2h10" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>',
    profile: '<svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle cx="12" cy="8" r="3.5" stroke="currentColor" stroke-width="1.8"/><path d="M5.5 19.5c1.5-3 4-4.5 6.5-4.5s5 1.5 6.5 4.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>',
  };

  function actIcon(name, title, onClick) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "act-icon";
    b.title = title;
    b.setAttribute("aria-label", title);
    b.innerHTML = ICONS[name] || "";
    b.onclick = onClick;
    return b;
  }

  let mode = "login";
  let me = "";
  let allPeople = [];
  let peopleFocus = {}; // username -> {status, room}
  let mutedUsers = new Set();
  let blockedUsers = new Set();
  let profileUser = "";
  let soundOn = true;
  let mainPanel = null;
  let sidePanel = null;
  let popAnchor = null;
  let unreadMap = {};
  let theme = localStorage.getItem("metis_theme") || "dark";

  function authHeaders(json = true) {
    const h = { Authorization: "Bearer " + store.token };
    if (json) h["Content-Type"] = "application/json";
    return h;
  }

  function playBeep() {
    if (!soundOn) return;
    try {
      const ctx = new (window.AudioContext || window.webkitAudioContext)();
      const o = ctx.createOscillator();
      const g = ctx.createGain();
      o.frequency.value = 880; g.gain.value = 0.04;
      o.connect(g); g.connect(ctx.destination);
      o.start();
      g.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.18);
      o.stop(ctx.currentTime + 0.2);
    } catch (_) {}
  }

  function notify(title, body, fromUser) {
    if (fromUser && mutedUsers.has(fromUser)) return;
    if (document.hidden && Notification.permission === "granted") {
      try { new Notification(title, { body, silent: true }); } catch (_) {}
    }
    playBeep();
  }

  function navButton({ prefix, label, badge, icon, active, sub, onClick }) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "nav-item" + (active ? " active" : "");
    let extra = "";
    if (icon === "message") {
      extra = `<span class="badge-icon" title="Mesaj yaz"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg></span>`;
    } else if (badge) {
      extra = `<span class="badge">${badge}</span>`;
    }
    btn.innerHTML = `<span class="prefix">${prefix}</span><span class="label-wrap"><span class="label"></span>${sub ? '<small class="sub"></small>' : ""}</span>${extra}`;
    btn.querySelector(".label").textContent = label;
    if (sub) btn.querySelector(".sub").textContent = sub;
    btn.onclick = onClick;
    return btn;
  }

  function toast(text) {
    const host = $("toastHost");
    if (!host) return;
    const el = document.createElement("div");
    el.className = "toast";
    el.textContent = text;
    host.appendChild(el);
    setTimeout(() => el.remove(), 3200);
  }

  function applyTheme() {
    document.body.classList.remove("theme-light", "theme-dark");
    document.body.classList.add(theme === "light" ? "theme-light" : "theme-dark");
    document.documentElement.style.colorScheme = theme === "light" ? "light" : "dark";
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute("content", theme === "light" ? "#e8eef5" : "#0f1419");
    const btn = $("themeToggle");
    if (btn) btn.textContent = theme === "light" ? "Tema: açık" : "Tema: koyu";
    document.querySelectorAll(".theme-opt").forEach((b) => {
      b.classList.toggle("active", b.getAttribute("data-theme") === theme);
    });
  }

  function setTheme(next) {
    const v = next === "light" ? "light" : "dark";
    if (theme === v) return;
    theme = v;
    localStorage.setItem("metis_theme", theme);
    applyTheme();
    toast(theme === "light" ? "Açık tema açıldı" : "Koyu tema açıldı");
  }

  class ChatPanel {
    constructor(root, { closable = false } = {}) {
      root.innerHTML = "";
      root.appendChild($("panelTpl").content.cloneNode(true));
      this.root = root;
      this.els = {
        title: root.querySelector(".room-title"),
        status: root.querySelector(".conn-status"),
        online: root.querySelector(".online-pills"),
        messages: root.querySelector(".messages"),
        typing: root.querySelector(".typing"),
        form: root.querySelector(".composer"),
        text: root.querySelector(".text"),
        file: root.querySelector(".file-input"),
        search: root.querySelector(".msg-search"),
        searchToggle: root.querySelector(".search-toggle"),
        searchPop: root.querySelector(".search-pop"),
        mute: root.querySelector(".mute-btn"),
        close: root.querySelector(".close-side"),
        replyBar: root.querySelector(".reply-bar"),
        replyPreview: root.querySelector(".reply-preview"),
        replyClear: root.querySelector(".reply-clear"),
        ephemeral: root.querySelector(".ephemeral-sel"),
        rateHint: root.querySelector(".rate-hint"),
        pinBar: root.querySelector(".pin-bar"),
        call: root.querySelector(".call-btn"),
        toolsToggle: root.querySelector(".tools-toggle"),
        toolsBubble: root.querySelector(".tools-bubble"),
      };
      this.room = "";
      this.ws = null;
      this.typingUsers = new Set();
      this.typingTimer = null;
      this.readByPeer = new Set();
      this.history = [];
      this.reactions = {};
      this.reply = null;
      this.rateUntil = 0;
      this.closable = closable;
      if (closable) this.els.close.classList.remove("hidden");

      this.els.form.onsubmit = (e) => {
        e.preventDefault();
        const btn = this.els.form.querySelector(".send-btn");
        if (btn) {
          btn.classList.remove("sending");
          void btn.offsetWidth;
          btn.classList.add("sending");
          setTimeout(() => btn.classList.remove("sending"), 360);
        }
        this.sendChat(this.els.text.value.trim());
        this.els.text.value = "";
      };
      this.els.text.addEventListener("input", () => this.sendTyping());
      this.els.text.addEventListener("focus", () => this.closeSearch());
      this.els.search.addEventListener("input", () => this.renderFiltered());
      this.els.searchToggle.onclick = (e) => {
        e.preventDefault();
        e.stopPropagation();
        const willOpen = this.els.searchPop.classList.contains("hidden");
        if (willOpen) this.openSearch();
        else this.closeSearch();
      };
      this.els.search.addEventListener("keydown", (e) => {
        if (e.key === "Escape") this.closeSearch();
      });
      this.els.searchPop?.addEventListener("click", (e) => e.stopPropagation());
      this.els.mute.onclick = () => {
        soundOn = !soundOn;
        this.els.mute.textContent = soundOn ? "🔔" : "🔕";
      };
      this.els.close.onclick = () => closeSidePanel();
      this.els.replyClear.onclick = () => this.clearReply();
      this.els.call.onclick = () => this.startCall();
      this.els.toolsToggle.onclick = (e) => {
        e.preventDefault();
        e.stopPropagation();
        const willOpen = this.els.toolsBubble.classList.contains("hidden");
        // Diğer panellerin balonunu kapat
        document.querySelectorAll(".tools-bubble").forEach((b) => {
          if (b !== this.els.toolsBubble) b.classList.add("hidden");
        });
        document.querySelectorAll(".tools-toggle").forEach((t) => {
          if (t !== this.els.toolsToggle) {
            t.classList.remove("open");
            t.setAttribute("aria-expanded", "false");
          }
        });
        if (willOpen) {
          hidePops();
          this.placeToolsBubble();
          this.els.toolsToggle.classList.add("open");
          this.els.toolsToggle.setAttribute("aria-expanded", "true");
        } else {
          this.closeToolsBubble();
        }
      };
      root.querySelectorAll("[data-act]").forEach((btn) => {
        btn.onclick = (e) => {
          e.stopPropagation();
          const act = btn.getAttribute("data-act");
          if (act === "emoji") {
            this.closeToolsBubble();
            openEmojiPop(this.els.toolsToggle, (em) => this.insertOrSendEmoji(em));
          }
          if (act === "sticker") {
            this.closeToolsBubble();
            openStickerPop(this.els.toolsToggle, (em) => this.sendPayload({ type: "sticker", content: em }));
          }
          if (act === "gif") {
            this.closeToolsBubble();
            openGifPop(this.els.toolsToggle, (url) => this.sendPayload({ type: "gif", content: url }));
          }
          if (act === "photo") {
            this.closeToolsBubble();
            this.els.file.click();
          }
        };
      });
      this.els.file.onchange = () => this.uploadFiles();
    }

    closeToolsBubble() {
      const bubble = this.els.toolsBubble;
      const wrap = this.root.querySelector(".tools-wrap");
      bubble?.classList.add("hidden");
      this.els.toolsToggle?.classList.remove("open");
      this.els.toolsToggle?.setAttribute("aria-expanded", "false");
      if (bubble && wrap && bubble.parentElement !== wrap) {
        wrap.appendChild(bubble);
      }
    }

    openSearch() {
      this.closeToolsBubble();
      this.els.searchPop?.classList.remove("hidden");
      this.els.searchToggle?.classList.add("open");
      this.els.searchToggle?.setAttribute("aria-expanded", "true");
      requestAnimationFrame(() => this.els.search?.focus());
    }

    closeSearch() {
      if (!this.els.searchPop) return;
      const wasOpen = !this.els.searchPop.classList.contains("hidden");
      const hadQuery = !!(this.els.search?.value || "").trim();
      this.els.searchPop.classList.add("hidden");
      this.els.searchToggle?.classList.remove("open");
      this.els.searchToggle?.setAttribute("aria-expanded", "false");
      if (this.els.search) this.els.search.value = "";
      if (wasOpen || hadQuery) this.renderFiltered();
    }

    placeToolsBubble() {
      const btn = this.els.toolsToggle;
      const bubble = this.els.toolsBubble;
      if (!btn || !bubble) return;
      // Panel transform/overflow kırpmasın diye body'ye taşı
      document.body.appendChild(bubble);
      bubble.classList.remove("hidden");
      const r = btn.getBoundingClientRect();
      const gap = 10;
      const w = Math.min(220, window.innerWidth - 24);
      bubble.style.width = w + "px";
      bubble.style.left = Math.max(8, Math.min(r.left, window.innerWidth - w - 8)) + "px";
      // Ölçüm için görünür olmalı
      const h = Math.min(bubble.offsetHeight || 260, window.innerHeight - 16);
      if (r.top >= h + gap + 8) {
        bubble.style.top = (r.top - h - gap) + "px";
      } else {
        bubble.style.top = Math.min(r.bottom + gap, window.innerHeight - h - 8) + "px";
      }
    }

    setStatus(on, text) {
      this.els.status.textContent = text;
      this.els.status.classList.toggle("on", on);
    }

    roomLabel(room) {
      if (room.startsWith("dm:")) {
        const peer = room.split(":").filter((p) => p !== "dm" && p !== me)[0];
        return "Özel · @" + peer;
      }
      return "Oda · #" + room;
    }

    connect(room) {
      if (this.ws) { this.ws.onclose = null; this.ws.close(); }
      this.room = room;
      this.history = [];
      this.reactions = {};
      this.readByPeer.clear();
      this.typingUsers.clear();
      this.clearReply();
      this.els.messages.innerHTML = "";
      this.els.title.textContent = this.roomLabel(room);
      this.els.title.onclick = () => openChatInfo(this.room, this.history);
      const proto = location.protocol === "https:" ? "wss" : "ws";
      this.ws = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(store.token)}&room=${encodeURIComponent(room)}`);
      this.ws.onopen = async () => {
        this.setStatus(true, "bağlı");
        await this.loadHistory();
        await this.loadPresence();
        await this.loadPins();
        this.els.text.focus();
        postFocus($("myFocus")?.value || "here", room);
        loadRooms();
      };
      this.ws.onmessage = (ev) => {
        try { this.onEvent(JSON.parse(ev.data)); } catch (_) {}
      };
      this.ws.onclose = () => this.setStatus(false, "koptu");
    }

    async loadPins() {
      const res = await fetch("/api/rooms/" + encodeURIComponent(this.room) + "/pins", { headers: authHeaders() });
      if (!res.ok) return;
      const pins = await res.json();
      this.renderPins(pins);
    }

    renderPins(pins) {
      const bar = this.els.pinBar;
      if (!bar) return;
      bar.innerHTML = "";
      if (!pins || !pins.length) {
        bar.classList.add("hidden");
        return;
      }
      bar.classList.remove("hidden");
      const p = pins[0];
      bar.textContent = "📌 " + (p.content || "").slice(0, 80);
    }

    async loadHistory() {
      const res = await fetch("/api/rooms/" + encodeURIComponent(this.room) + "/messages", { headers: authHeaders() });
      if (!res.ok) return;
      this.history = await res.json();
      const reads = await fetch("/api/rooms/" + encodeURIComponent(this.room) + "/reads", { headers: authHeaders() })
        .then((r) => r.ok ? r.json() : {});
      Object.entries(reads || {}).forEach(([u, mid]) => { if (u !== me) this.readByPeer.add(mid); });
      this.renderFiltered();
      const last = [...this.history].reverse().find((m) => m.id && m.user !== me);
      if (last) this.markRead(last.id);
    }

    async loadPresence() {
      const res = await fetch("/api/rooms/" + encodeURIComponent(this.room) + "/presence", { headers: authHeaders() });
      if (!res.ok) return;
      const data = await res.json();
      this.renderOnline(data.online || []);
    }

    renderOnline(list) {
      const el = this.els.online;
      el.innerHTML = "";
      this.onlineList = list || [];
      const arr = this.onlineList;
      const show = arr.slice(0, 4);
      show.forEach((name) => {
        const pill = document.createElement("div");
        pill.className = "pill";
        pill.title = name;
        pill.innerHTML = '<span class="dot"></span><span></span>';
        pill.querySelector("span:last-child").textContent = name;
        el.appendChild(pill);
      });
      if (arr.length > 4) {
        const more = document.createElement("button");
        more.type = "button";
        more.className = "pill more";
        more.textContent = `+${arr.length - 4}`;
        more.title = "Tüm çevrimiçiler";
        more.onclick = (e) => {
          e.stopPropagation();
          this.showOnlineAll(more);
        };
        el.appendChild(more);
      }
    }

    showOnlineAll(anchor) {
      if (this._onlinePop && document.body.contains(this._onlinePop)) {
        this._onlinePop.remove();
        this._onlinePop = null;
        return;
      }
      document.querySelectorAll(".online-pop").forEach((p) => p.remove());
      const pop = document.createElement("div");
      pop.className = "online-pop popover";
      const title = document.createElement("div");
      title.className = "online-pop-title";
      title.textContent = `Çevrimiçi · ${this.onlineList.length}`;
      pop.appendChild(title);
      const list = document.createElement("div");
      list.className = "online-pop-list";
      (this.onlineList || []).forEach((name) => {
        const row = document.createElement("button");
        row.type = "button";
        row.className = "online-pop-item";
        row.innerHTML = '<span class="dot"></span><span></span>';
        row.querySelector("span:last-child").textContent = name;
        row.onclick = (e) => {
          e.stopPropagation();
          pop.remove();
          this._onlinePop = null;
          if (name !== me) openDm(name);
        };
        list.appendChild(row);
      });
      pop.appendChild(list);
      document.body.appendChild(pop);
      this._onlinePop = pop;
      const r = anchor.getBoundingClientRect();
      const w = Math.min(240, window.innerWidth - 24);
      pop.style.width = w + "px";
      pop.style.left = Math.max(12, Math.min(r.right - w, window.innerWidth - w - 12)) + "px";
      pop.style.top = Math.min(r.bottom + 8, window.innerHeight - 280) + "px";

      const cleanup = () => {
        document.removeEventListener("mousedown", onDoc, true);
        document.removeEventListener("keydown", onKey, true);
        if (this._onlinePop === pop) this._onlinePop = null;
      };
      const onDoc = (ev) => {
        if (anchor.contains(ev.target) || ev.target === anchor) return;
        if (pop.contains(ev.target)) return;
        pop.remove();
        cleanup();
      };
      const onKey = (ev) => {
        if (ev.key !== "Escape") return;
        pop.remove();
        cleanup();
      };
      requestAnimationFrame(() => {
        document.addEventListener("mousedown", onDoc, true);
        document.addEventListener("keydown", onKey, true);
      });
    }

    renderFiltered() {
      const q = (this.els.search.value || "").trim().toLowerCase();
      this.els.messages.innerHTML = "";
      this.history.forEach((m) => {
        if (q && !(m.content || "").toLowerCase().includes(q) && !(m.user || "").toLowerCase().includes(q)) return;
        this.appendMsg(m, { history: true });
      });
    }

    onEvent(m) {
      if (m.type === "typing") {
        if (m.user === me) return;
        this.typingUsers.add(m.user);
        this.updateTyping();
        clearTimeout(this._typingClear);
        this._typingClear = setTimeout(() => { this.typingUsers.delete(m.user); this.updateTyping(); }, 1500);
        return;
      }
      if (m.type === "presence") {
        this.renderOnline(m.online || []);
        loadRooms();
        return;
      }
      if (m.type === "focus") {
        if (m.user) peopleFocus[m.user] = { status: m.status, room: m.room };
        renderPeople($("peopleSearch")?.value || "");
        return;
      }
      if (m.type === "rate_limit") {
        if (m.user && m.user !== me) return;
        const sec = Math.max(1, Number(m.retry_after) || 1);
        this.rateUntil = Date.now() + sec * 1000;
        this.showRateHint(sec);
        return;
      }
      if (m.type === "call") {
        if (m.to === me || !m.to) {
          toast((m.from || "biri") + " arıyor… (WebRTC sinyali)");
        }
        return;
      }
      if (m.type === "pin") {
        this.loadPins();
        return;
      }
      if (m.type === "read") {
        if (m.user !== me && m.message_id) {
          this.readByPeer.add(m.message_id);
          const tick = this.els.messages.querySelector(`[data-id="${CSS.escape(m.message_id)}"] .ticks`);
          if (tick) tick.textContent = "✓✓";
        }
        return;
      }
      if (m.type === "reaction") {
        this.addReactionLocal(m.message_id, m.content);
        return;
      }
      if (m.type === "edit") {
        const item = this.history.find((x) => x.id === m.message_id);
        if (item) {
          item.content = m.content;
          item.edited = true;
        }
        const node = this.els.messages.querySelector(`[data-id="${CSS.escape(m.message_id)}"]`);
        if (node) {
          const body = node.querySelector(".msg-body");
          if (body) body.textContent = m.content;
          let tag = node.querySelector(".edited-tag");
          if (!tag) {
            tag = document.createElement("span");
            tag.className = "edited-tag";
            tag.textContent = "düzenlendi";
            node.querySelector(".meta")?.appendChild(tag);
          }
        }
        return;
      }
      if (m.type === "unsend") {
        const idx = this.history.findIndex((x) => x.id === m.message_id);
        if (idx >= 0) {
          this.history[idx].type = "unsent";
          this.history[idx].content = "";
        }
        const node = this.els.messages.querySelector(`[data-id="${CSS.escape(m.message_id)}"]`);
        if (node) {
          const block = node.closest(".msg-block") || node;
          block.querySelector(".reactions")?.remove();
          node.className = "msg system";
          node.innerHTML = "";
          node.textContent = "Mesaj geri alındı";
        }
        return;
      }
      this.history.push(m);
      this.appendMsg(m);
      if (m.user && m.user !== me && m.type !== "system") {
        if (blockedUsers.has(m.user)) return;
        notify(m.user, m.type === "gif" ? "GIF gönderdi" : m.type === "image" ? "fotoğraf gönderdi" : m.content, m.user);
        if (m.id) this.markRead(m.id);
      }
    }

    appendMsg(m, opts = {}) {
      if (m.user && blockedUsers.has(m.user) && m.user !== me && m.type !== "system") return;
      const block = document.createElement("div");
      block.className = "msg-block" + (m.user === me && m.type !== "system" && m.type !== "unsent" ? " me" : "");
      const el = document.createElement("div");
      el.className = "msg" + (m.type === "system" || m.type === "unsent" ? " system" : m.user === me ? " me" : "");
      if (m.id) el.dataset.id = m.id;
      if (m.ephemeral) el.classList.add("ephemeral");

      if (m.type === "system") {
        el.textContent = m.content;
      } else if (m.type === "unsent") {
        el.textContent = "Mesaj geri alındı";
      } else {
        const meta = document.createElement("div");
        meta.className = "meta";
        const who = document.createElement("button");
        who.type = "button";
        who.className = "msg-user";
        who.textContent = m.user;
        who.title = "Profili aç";
        who.onclick = (e) => {
          e.stopPropagation();
          openProfile(m.user, { room: this.room, history: this.history });
        };
        const time = document.createElement("span");
        time.textContent = " · " + new Date(m.ts).toLocaleTimeString();
        const left = document.createElement("span");
        left.append(who, time);
        meta.appendChild(left);
        if (m.ephemeral) {
          const ep = document.createElement("span");
          ep.className = "ephemeral-tag";
          ep.textContent = m.ephemeral === "after_read" ? "okununca silinir" : "24s";
          meta.appendChild(ep);
        }
        if (m.edited) {
          const tag = document.createElement("span");
          tag.className = "edited-tag";
          tag.textContent = "düzenlendi";
          meta.appendChild(tag);
        }
        if (m.user === me && m.id) {
          const ticks = document.createElement("span");
          ticks.className = "ticks";
          ticks.textContent = this.readByPeer.has(m.id) ? "✓✓" : "✓";
          meta.appendChild(ticks);
        }
        el.appendChild(meta);

        if (m.reply_to) {
          const q = document.createElement("div");
          q.className = "quote";
          q.textContent = (m.reply_user ? "@" + m.reply_user + ": " : "") + (m.reply_content || "").slice(0, 120);
          el.appendChild(q);
        }

        if (m.type === "image" || m.type === "gif") {
          const img = document.createElement("img");
          img.className = m.type === "gif" ? "gif" : "";
          img.src = m.content;
          img.alt = m.type;
          img.loading = "lazy";
          el.appendChild(img);
        } else if (m.type === "album") {
          const wrap = document.createElement("div");
          wrap.className = "album";
          try {
            (JSON.parse(m.content) || []).forEach((url) => {
              const img = document.createElement("img");
              img.src = url;
              img.loading = "lazy";
              wrap.appendChild(img);
            });
          } catch (_) {
            wrap.textContent = m.content;
          }
          el.appendChild(wrap);
        } else if (m.type === "sticker") {
          const st = document.createElement("div");
          st.className = "sticker";
          st.textContent = m.content;
          el.appendChild(st);
        } else if (m.type === "file") {
          let name = "dosya";
          try { name = JSON.parse(m.meta || "{}").name || name; } catch (_) {}
          const a = document.createElement("a");
          a.className = "file"; a.href = m.content; a.target = "_blank";
          a.textContent = "📄 " + name;
          el.appendChild(a);
        } else {
          const body = document.createElement("div");
          body.className = "msg-body";
          const onlyEmoji = /^[\p{Extended_Pictographic}\s]+$/u.test(m.content || "") && [...(m.content || "")].filter((c) => c.trim()).length <= 6;
          if (onlyEmoji) body.classList.add("emoji-only");
          body.textContent = m.content;
          el.appendChild(body);
        }

        if (m.id) {
          const actions = document.createElement("div");
          actions.className = "msg-actions";
          const reactRow = document.createElement("div");
          reactRow.className = "react-row";
          REACT_SET.forEach((em) => {
            const b = document.createElement("button");
            b.type = "button";
            b.className = "react-emoji";
            b.textContent = em;
            b.title = "İfade: " + em;
            b.onclick = (e) => { e.stopPropagation(); this.sendPayload({ type: "reaction", message_id: m.id, content: em }); };
            reactRow.appendChild(b);
          });
          actions.appendChild(reactRow);
          const sep = document.createElement("span");
          sep.className = "act-sep";
          sep.setAttribute("aria-hidden", "true");
          actions.appendChild(sep);
          const tools = document.createElement("div");
          tools.className = "act-tools";
          tools.appendChild(actIcon("reply", "Yanıtla", (e) => {
            e.stopPropagation();
            quoteIntoSideOrSelf(m, this);
          }));
          tools.appendChild(actIcon("pin", "Pinle", async (e) => {
            e.stopPropagation();
            await fetch("/api/rooms/" + encodeURIComponent(this.room) + "/pins", {
              method: "POST", headers: authHeaders(),
              body: JSON.stringify({ message_id: m.id, content: m.content, user: m.user }),
            });
            this.loadPins();
          }));
          if (m.user === me) {
            if (m.type === "chat" || !m.type) {
              tools.appendChild(actIcon("edit", "Düzenle", (e) => {
                e.stopPropagation();
                const next = prompt("Mesajı düzenle:", m.content || "");
                if (next == null) return;
                const trimmed = next.trim();
                if (!trimmed) return;
                this.sendPayload({ type: "edit", message_id: m.id, content: trimmed });
              }));
            }
            tools.appendChild(actIcon("trash", "Sil", (e) => {
              e.stopPropagation();
              if (!confirm("Mesaj silinsin mi?")) return;
              this.sendPayload({ type: "unsend", message_id: m.id });
            }));
          }
          tools.appendChild(actIcon("copy", "Kopyala", (e) => {
            e.stopPropagation();
            navigator.clipboard?.writeText(m.content || "");
          }));
          if (m.user !== me) {
            tools.appendChild(actIcon("profile", "Profil", (e) => {
              e.stopPropagation();
              openProfile(m.user, { room: this.room, history: this.history });
            }));
          }
          actions.appendChild(tools);
          el.appendChild(actions);

          el.addEventListener("click", (e) => {
            if (e.target.closest("button,a,img")) return;
            quoteIntoSideOrSelf(m, this);
          });
        }
      }

      block.appendChild(el);
      if (m.id && m.type !== "system" && m.type !== "unsent") {
        const reacts = document.createElement("div");
        reacts.className = "reactions";
        reacts.dataset.for = m.id;
        this.paintReactions(reacts, m.id);
        block.appendChild(reacts);
      }
      this.els.messages.appendChild(block);
      if (!opts.history || !this.els.search.value) {
        this.els.messages.scrollTop = this.els.messages.scrollHeight;
      }
    }

    paintReactions(container, mid) {
      container.innerHTML = "";
      const map = this.reactions[mid] || {};
      Object.entries(map).forEach(([em, n]) => {
        if (!n) return;
        const chip = document.createElement("button");
        chip.type = "button";
        chip.className = "react-chip";
        chip.textContent = `${em} ${n}`;
        chip.onclick = () => this.sendPayload({ type: "reaction", message_id: mid, content: em });
        container.appendChild(chip);
      });
    }

    addReactionLocal(mid, emoji) {
      if (!mid || !emoji) return;
      if (!this.reactions[mid]) this.reactions[mid] = {};
      this.reactions[mid][emoji] = (this.reactions[mid][emoji] || 0) + 1;
      const box = this.els.messages.querySelector(`.reactions[data-for="${CSS.escape(mid)}"]`);
      if (box) this.paintReactions(box, mid);
    }

    updateTyping() {
      const names = [...this.typingUsers];
      this.els.typing.textContent = names.length ? names.join(", ") + " yazıyor…" : "";
    }

    sendPayload(obj) {
      if (!this.ws || this.ws.readyState !== 1) return;
      if (this.rateUntil > Date.now() && (obj.type === "chat" || obj.type === "image" || obj.type === "gif" || obj.type === "file" || obj.type === "sticker" || obj.type === "album")) {
        const sec = Math.ceil((this.rateUntil - Date.now()) / 1000);
        this.showRateHint(sec);
        return;
      }
      this.ws.send(JSON.stringify(obj));
    }

    showRateHint(sec) {
      const el = this.els.rateHint;
      if (!el) return;
      el.classList.remove("hidden");
      el.textContent = `${sec} saniye sonra gönderilebilir`;
      toast(`${sec} saniye sonra gönderilebilir`);
      clearTimeout(this._rateTimer);
      this._rateTimer = setTimeout(() => {
        const left = Math.ceil((this.rateUntil - Date.now()) / 1000);
        if (left > 0) this.showRateHint(left);
        else el.classList.add("hidden");
      }, 1000);
    }

    setReply(m) {
      this.reply = {
        id: m.id,
        user: m.user,
        content: (m.content || "").slice(0, 160),
      };
      this.els.replyBar.classList.remove("hidden");
      this.els.replyPreview.textContent = `@${m.user}: ${(m.content || "").slice(0, 100)}`;
      this.els.text.focus();
    }

    clearReply() {
      this.reply = null;
      this.els.replyBar?.classList.add("hidden");
      if (this.els.replyPreview) this.els.replyPreview.textContent = "";
    }

    sendChat(content) {
      if (!content) return;
      if (content.startsWith("/gif ")) {
        const q = content.slice(5).trim().toLowerCase();
        const g = GIFS.find((x) => x.t.includes(q)) || GIFS[0];
        this.sendPayload({ type: "gif", content: g.u });
        return;
      }
      const payload = {
        type: "chat",
        content,
        ephemeral: this.els.ephemeral?.value || "",
      };
      if (this.reply) {
        payload.reply_to = this.reply.id;
        payload.reply_user = this.reply.user;
        payload.reply_content = this.reply.content;
      }
      this.sendPayload(payload);
      this.clearReply();
      if (this.els.ephemeral) this.els.ephemeral.value = "";
    }

    insertOrSendEmoji(em) {
      const t = this.els.text;
      if (t.value.trim() === "") this.sendPayload({ type: "chat", content: em, ephemeral: this.els.ephemeral?.value || "" });
      else {
        t.value += em;
        t.focus();
      }
    }

    sendTyping() {
      if (!this.ws || this.ws.readyState !== 1) return;
      if (this.typingTimer) return;
      this.sendPayload({ type: "typing" });
      this.typingTimer = setTimeout(() => { this.typingTimer = null; }, 1200);
    }

    markRead(id) {
      if (!id) return;
      this.sendPayload({ type: "read", message_id: id });
    }

    async uploadFiles() {
      const files = [...(this.els.file.files || [])];
      this.els.file.value = "";
      if (!files.length) return;
      const urls = [];
      for (const file of files) {
        const fd = new FormData();
        fd.append("file", file);
        const res = await fetch("/api/upload", { method: "POST", headers: { Authorization: "Bearer " + store.token }, body: fd });
        if (!res.ok) { alert(await res.text()); return; }
        const data = await res.json();
        urls.push(data);
      }
      if (urls.length === 1) {
        this.sendPayload({ type: urls[0].type, content: urls[0].url, meta: urls[0].meta, ephemeral: this.els.ephemeral?.value || "" });
      } else {
        this.sendPayload({
          type: "album",
          content: JSON.stringify(urls.map((u) => u.url)),
          meta: JSON.stringify({ count: urls.length }),
          ephemeral: this.els.ephemeral?.value || "",
        });
      }
    }

    async startCall() {
      if (!this.room.startsWith("dm:")) {
        toast("Arama şu an özel mesajlarda");
        return;
      }
      const peer = this.room.split(":").filter((p) => p !== "dm" && p !== me)[0];
      await fetch("/api/signal", {
        method: "POST", headers: authHeaders(),
        body: JSON.stringify({ to: peer, action: "invite", room: this.room }),
      });
      toast("Arama sinyali gönderildi → @" + peer);
    }

    destroy() {
      if (this.ws) { this.ws.onclose = null; this.ws.close(); }
      this.root.innerHTML = "";
    }
  }

  function setSplit(on) {
    $("panels").classList.toggle("split", on);
    $("panelSide").classList.toggle("hidden", !on);
  }

  function ensureMain() {
    if (!mainPanel) mainPanel = new ChatPanel($("panelMain"));
    return mainPanel;
  }

  function openSide(room) {
    setSplit(true);
    if (sidePanel) sidePanel.destroy();
    sidePanel = new ChatPanel($("panelSide"), { closable: true });
    sidePanel.connect(room);
  }

  function closeSidePanel() {
    if (sidePanel) { sidePanel.destroy(); sidePanel = null; }
    setSplit(false);
    loadRooms();
  }

  async function openRoom(room) {
    ensureMain().connect(room);
    // keep side if it's a different DM
    loadRooms();
  }

  async function openDm(to) {
    const res = await fetch("/api/dm", { method: "POST", headers: authHeaders(), body: JSON.stringify({ to }) });
    if (!res.ok) { alert(await res.text()); return; }
    const data = await res.json();
    const mainIsPublic = mainPanel && mainPanel.room && !mainPanel.room.startsWith("dm:");
    if (mainIsPublic) openSide(data.name);
    else {
      closeSidePanel();
      ensureMain().connect(data.name);
    }
    await loadRooms();
  }

  function quoteIntoSideOrSelf(m, fromPanel) {
    // Soldaki odadan tıklanınca sağ panel açıksa alıntı sağa gider; paneller kapanmaz.
    if (!fromPanel.closable && sidePanel) {
      sidePanel.setReply(m);
      return;
    }
    fromPanel.setReply(m);
  }

  async function postFocus(status, room) {
    const r = room || mainPanel?.room || "";
    await fetch("/api/focus", {
      method: "POST", headers: authHeaders(),
      body: JSON.stringify({ status, room: r }),
    }).catch(() => {});
  }

  function openStickerPop(anchor, onPick) {
    const pop = $("stickerPop");
    pop.innerHTML = "";
    STICKERS.forEach((s) => {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = s;
      b.onclick = () => { onPick(s); hidePops(); };
      pop.appendChild(b);
    });
    placePop(pop, anchor);
    popAnchor = anchor;
  }

  async function loadRooms() {
    const res = await fetch("/api/rooms", { headers: authHeaders() });
    if (!res.ok) return;
    const rooms = await res.json();
    const roomEl = $("rooms");
    const dmEl = $("dms");
    roomEl.innerHTML = "";
    dmEl.innerHTML = "";
    const activeMain = mainPanel?.room;
    const activeSide = sidePanel?.room;
    const publics = rooms.filter((r) => r.kind !== "dm");
    const dms = rooms.filter((r) => r.kind === "dm");
    if (!publics.length) roomEl.innerHTML = '<div class="empty-hint">Oda yok</div>';
    publics.forEach((r) => {
      let sub = "";
      if (r.kind === "timed" || r.kind === "event") {
        const left = r.expires_at ? Math.max(0, r.expires_at - Math.floor(Date.now() / 1000)) : 0;
        const mins = Math.ceil(left / 60);
        sub = (r.kind === "event" ? "etkinlik · " : "süreli · ") + (mins > 60 ? Math.ceil(mins / 60) + "s" : mins + "dk");
      }
      const badge = r.unread > 0 ? String(r.unread) : (r.online_count > 0 ? String(r.online_count) : "");
      roomEl.appendChild(navButton({
        prefix: r.kind === "timed" || r.kind === "event" ? "⏳" : "#",
        label: r.name, badge, sub,
        active: r.name === activeMain,
        onClick: () => openRoom(r.name),
      }));
    });
    if (!dms.length) dmEl.innerHTML = '<div class="empty-hint">Henüz DM yok</div>';
    dms.forEach((r) => {
      const badge = r.unread > 0 ? String(r.unread) : (r.online_count > 0 ? String(r.online_count) : "");
      dmEl.appendChild(navButton({
        prefix: "@", label: r.peer, badge,
        active: r.name === activeMain || r.name === activeSide,
        onClick: () => {
          const mainIsPublic = mainPanel && mainPanel.room && !mainPanel.room.startsWith("dm:");
          if (mainIsPublic) openSide(r.name);
          else { closeSidePanel(); openRoom(r.name); }
        },
      }));
    });
  }

  async function loadUsers() {
    const res = await fetch("/api/users", { headers: authHeaders() });
    if (!res.ok) return;
    const data = await res.json();
    // backward compat: string[] or objects
    if (data.length && typeof data[0] === "string") {
      allPeople = data.filter((u) => u !== me).sort();
      peopleFocus = {};
    } else {
      allPeople = data.map((u) => u.username).filter((u) => u !== me).sort();
      peopleFocus = {};
      data.forEach((u) => {
        peopleFocus[u.username] = { status: u.status, room: u.room };
      });
    }
    renderPeople($("peopleSearch").value || "");
  }

  function renderPeople(query) {
    const el = $("people");
    el.innerHTML = "";
    const q = (query || "").trim().toLowerCase();
    const list = allPeople.filter((u) => !q || u.includes(q));
    if (!list.length) {
      el.innerHTML = `<div class="empty-hint">${q ? "Eşleşen yok" : "Kullanıcı yok"}</div>`;
      return;
    }
    list.slice(0, 80).forEach((u) => {
      const f = peopleFocus[u] || {};
      let sub = FOCUS_LABEL[f.status] || "";
      if (f.room && f.status === "here") {
        sub = "#" + (f.room.startsWith("dm:") ? "dm" : f.room);
      } else if (f.status === "typing" && f.room) {
        sub = "yazıyor · #" + (f.room.startsWith("dm:") ? "dm" : f.room);
      }
      el.appendChild(navButton({
        prefix: "@", label: u, icon: "message", sub,
        onClick: () => openDm(u),
      }));
    });
  }

  function extractLinks(history) {
    const urlRe = /https?:\/\/[^\s<>"']+/gi;
    const out = [];
    const seen = new Set();
    (history || []).forEach((m) => {
      if (m.type === "unsent") return;
      const text = m.content || "";
      const matches = text.match(urlRe) || [];
      matches.forEach((u) => {
        if (seen.has(u)) return;
        seen.add(u);
        out.push({ url: u, user: m.user, ts: m.ts });
      });
      if ((m.type === "image" || m.type === "gif" || m.type === "file") && m.content && m.content.startsWith("http")) {
        if (!seen.has(m.content)) {
          seen.add(m.content);
          out.push({ url: m.content, user: m.user, ts: m.ts });
        }
      }
    });
    return out;
  }

  function extractMediaFromHistory(history, username) {
    return (history || []).filter((m) => {
      if (!(m.type === "image" || m.type === "gif")) return false;
      if (!m.content) return false;
      if (username && m.user !== username) return false;
      return true;
    }).map((m) => ({ url: m.content, type: m.type, user: m.user, ts: m.ts }));
  }

  function setProfileTab(tab) {
    document.querySelectorAll(".tab-btn").forEach((b) => b.classList.toggle("active", b.dataset.tab === tab));
    $("tabMedia").classList.toggle("hidden", tab !== "media");
    $("tabLinks").classList.toggle("hidden", tab !== "links");
    $("tabSettings")?.classList.toggle("hidden", tab !== "settings");
  }

  function fillSelfSettings() {
    const focusSel = $("settingsFocus");
    const soundCb = $("settingsSound");
    if (focusSel) focusSel.value = $("myFocus")?.value || "here";
    if (soundCb) soundCb.checked = soundOn;
    document.querySelectorAll(".theme-opt").forEach((b) => {
      b.classList.toggle("active", b.getAttribute("data-theme") === theme);
    });
  }

  async function freshHistory(room) {
    if (!room) return [];
    const res = await fetch("/api/rooms/" + encodeURIComponent(room) + "/messages", { headers: authHeaders() });
    if (!res.ok) return [];
    return res.json();
  }

  async function openChatInfo(room, history) {
    if (!room) return;
    const hist = (await freshHistory(room)) || history || [];
    if (room.startsWith("dm:")) {
      const peer = room.split(":").filter((p) => p !== "dm" && p !== me)[0];
      openProfile(peer, { room, history: hist, fromHeader: true });
      return;
    }
    openRoomInfo(room, hist);
  }

  function openRoomInfo(room, history) {
    profileUser = "";
    const modal = $("profileModal");
    modal.classList.remove("hidden");
    setProfileTab("media");
    $("profileName").textContent = "#" + room;
    $("profileAvatar").textContent = "#";
    $("profileSub").textContent = "oda bilgisi · medya ve bağlantılar";
    $("profileDm").classList.add("hidden");
    $("profileMute").classList.add("hidden");
    $("profileBlock").classList.add("hidden");
    $("tabSettingsBtn")?.classList.add("hidden");
    renderMediaGrid(extractMediaFromHistory(history));
    renderLinksList(extractLinks(history));
  }

  function renderMediaGrid(media) {
    const grid = $("profileMedia");
    grid.innerHTML = "";
    if (!media.length) {
      grid.innerHTML = '<div class="empty-media">Bu sohbette henüz foto / GIF yok. 🖼️ ile gönderince burada listelenir.</div>';
      return;
    }
    media.forEach((item) => {
      const a = document.createElement("a");
      a.href = item.url; a.target = "_blank"; a.rel = "noopener";
      const src = item.url.startsWith("http") || item.url.startsWith("/") ? item.url : "/" + item.url;
      a.innerHTML = `<img src="${src}" alt="" loading="lazy" />`;
      grid.appendChild(a);
    });
  }

  function renderLinksList(links) {
    const box = $("profileLinks");
    box.innerHTML = "";
    if (!links.length) {
      box.innerHTML = '<div class="empty-media">Paylaşılan bağlantı yok</div>';
      return;
    }
    links.forEach((l) => {
      const a = document.createElement("a");
      a.className = "link-row";
      a.href = l.url; a.target = "_blank"; a.rel = "noopener";
      a.innerHTML = `<strong></strong><span></span>`;
      a.querySelector("strong").textContent = l.url;
      a.querySelector("span").textContent = (l.user || "") + (l.ts ? " · " + new Date(l.ts).toLocaleString() : "");
      box.appendChild(a);
    });
  }

  async function openProfile(username, opts = {}) {
    if (!username || username === "system") return;
    profileUser = username;
    const modal = $("profileModal");
    modal.classList.remove("hidden");
    const isSelf = username === me;
    $("tabSettingsBtn")?.classList.toggle("hidden", !isSelf);
    setProfileTab(isSelf && opts.settings ? "settings" : "media");
    $("profileName").textContent = "@" + username;
    $("profileAvatar").textContent = (username[0] || "?").toUpperCase();
    $("profileSub").textContent = "yükleniyor…";
    $("profileMedia").innerHTML = '<div class="empty-media">Yükleniyor…</div>';
    $("profileLinks").innerHTML = "";

    let history = opts.history || [];
    if (opts.room) {
      history = await freshHistory(opts.room);
    }

    const res = await fetch("/api/users/" + encodeURIComponent(username), { headers: authHeaders() });
    if (!res.ok) {
      $("profileSub").textContent = "profil alınamadı";
      return;
    }
    const p = await res.json();
    if (p.muted) mutedUsers.add(username); else mutedUsers.delete(username);
    if (p.blocked) blockedUsers.add(username); else blockedUsers.delete(username);

    const chatMediaAll = extractMediaFromHistory(history);
    const chatMediaUser = extractMediaFromHistory(history, username);
    $("profileSub").textContent = p.self
      ? "profilin · ayarlardan temayı ve odağı yönet"
      : (p.muted ? "bildirimler kapalı · " : "") + (p.blocked ? "engelli · " : "") + `${chatMediaAll.length} medya bu sohbette`;

    const hideDm = p.self || opts.fromHeader;
    $("profileDm").classList.toggle("hidden", hideDm);
    $("profileMute").classList.toggle("hidden", !!p.self);
    $("profileBlock").classList.toggle("hidden", !!p.self);
    $("profileMute").textContent = p.muted ? "Sessizden çıkar" : "Bildirimleri sessize al";
    $("profileBlock").textContent = p.blocked ? "Engeli kaldır" : "Engelle";

    if (p.self) fillSelfSettings();

    let media = opts.fromHeader ? chatMediaAll : chatMediaUser;
    if (!media.length) {
      const mediaRes = await fetch("/api/users/" + encodeURIComponent(username) + "/media", { headers: authHeaders() });
      if (mediaRes.ok) {
        media = (await mediaRes.json() || []).map((x) => ({ url: x.url, type: x.type }));
      }
    }
    renderMediaGrid(media);
    renderLinksList(extractLinks(opts.fromHeader ? history : history.filter((m) => m.user === username)));
  }

  function closeProfile() {
    $("profileModal").classList.add("hidden");
    profileUser = "";
  }

  async function toggleMute() {
    if (!profileUser) return;
    const muted = mutedUsers.has(profileUser);
    const res = await fetch("/api/users/" + encodeURIComponent(profileUser) + "/mute", {
      method: muted ? "DELETE" : "POST",
      headers: authHeaders(),
    });
    if (!res.ok) { alert(await res.text()); return; }
    openProfile(profileUser);
  }

  async function toggleBlock() {
    if (!profileUser) return;
    const blocked = blockedUsers.has(profileUser);
    if (!blocked && !confirm(profileUser + " engellensin mi? DM ve mesajları gizlenir.")) return;
    const res = await fetch("/api/users/" + encodeURIComponent(profileUser) + "/block", {
      method: blocked ? "DELETE" : "POST",
      headers: authHeaders(),
    });
    if (!res.ok) { alert(await res.text()); return; }
    if (!blocked) {
      blockedUsers.add(profileUser);
      if (sidePanel?.room?.includes(profileUser)) closeSidePanel();
    } else {
      blockedUsers.delete(profileUser);
    }
    openProfile(profileUser);
    if (mainPanel) mainPanel.renderFiltered();
  }

  function hidePops() {
    $("emojiPop").classList.add("hidden");
    $("gifPop").classList.add("hidden");
    $("stickerPop")?.classList.add("hidden");
  }

  function placePop(pop, anchor) {
    const r = anchor.getBoundingClientRect();
    pop.classList.remove("hidden");
    const top = Math.min(r.top - pop.offsetHeight - 8, window.innerHeight - pop.offsetHeight - 12);
    const left = Math.min(Math.max(12, r.left), window.innerWidth - pop.offsetWidth - 12);
    pop.style.top = Math.max(12, top) + "px";
    pop.style.left = left + "px";
  }

  function openEmojiPop(anchor, onPick) {
    const pop = $("emojiPop");
    pop.innerHTML = '<div class="emoji-grid"></div>';
    const grid = pop.querySelector(".emoji-grid");
    EMOJIS.forEach((em) => {
      const b = document.createElement("button");
      b.type = "button"; b.textContent = em;
      b.onclick = () => { onPick(em); hidePops(); };
      grid.appendChild(b);
    });
    placePop(pop, anchor);
    popAnchor = anchor;
  }

  function openGifPop(anchor, onPick) {
    const pop = $("gifPop");
    pop.innerHTML = '<div class="gif-grid"></div>';
    const grid = pop.querySelector(".gif-grid");
    GIFS.forEach((g) => {
      const b = document.createElement("button");
      b.type = "button"; b.title = g.t;
      b.innerHTML = `<img src="${g.u}" alt="${g.t}" loading="lazy" />`;
      b.onclick = () => { onPick(g.u); hidePops(); };
      grid.appendChild(b);
    });
    placePop(pop, anchor);
    popAnchor = anchor;
  }

  document.addEventListener("click", (e) => {
    if (!e.target.closest(".popover") && !e.target.closest("[data-act]")) hidePops();
    const inTools =
      e.target.closest(".tools-wrap") ||
      e.target.closest(".tools-bubble") ||
      e.target.closest(".tools-toggle");
    if (!inTools) {
      mainPanel?.closeToolsBubble?.();
      sidePanel?.closeToolsBubble?.();
    }
    if (!e.target.closest(".header-search")) {
      mainPanel?.closeSearch?.();
      sidePanel?.closeSearch?.();
    }
  });

  function showApp() {
    me = store.user;
    $("meLabel").textContent = me;
    $("meHint").textContent = "oturum açık";
    $("avatar").textContent = me[0] || "?";
    $("auth").classList.add("hidden");
    $("workspace").classList.remove("hidden");
    if ("Notification" in window && Notification.permission === "default") Notification.requestPermission().catch(() => {});
    loadUsers();
    ensureMain();
    openRoom("general");
  }

  async function doAuth() {
    $("authErr").textContent = "";
    const username = $("username").value.trim();
    const password = $("password").value;
    const path = mode === "login" ? "/api/auth/login" : "/api/auth/register";
    const res = await fetch(path, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
    });
    const text = await res.text();
    if (!res.ok) { $("authErr").textContent = text || "hata"; return; }
    const data = JSON.parse(text);
    store.token = data.token; store.user = data.username;
    showApp();
  }

  $("tabLogin").onclick = () => {
    mode = "login"; $("tabLogin").classList.add("active-tab"); $("tabRegister").classList.remove("active-tab");
    $("authBtn").textContent = "Giriş yap";
  };
  $("tabRegister").onclick = () => {
    mode = "register"; $("tabRegister").classList.add("active-tab"); $("tabLogin").classList.remove("active-tab");
    $("authBtn").textContent = "Kayıt ol";
  };
  $("authBtn").onclick = doAuth;
  $("password").addEventListener("keydown", (e) => { if (e.key === "Enter") doAuth(); });
  $("logoutBtn").onclick = (e) => {
    e.stopPropagation();
    store.clear();
    location.reload();
  };
  $("meCard")?.addEventListener("click", (e) => {
    if (e.target.closest("#logoutBtn")) return;
    if (!me) return;
    openProfile(me, { settings: true });
  });
  $("meCard")?.addEventListener("keydown", (e) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      if (!me) return;
      openProfile(me, { settings: true });
    }
  });
  $("profileModal")?.addEventListener("click", (e) => {
    const opt = e.target.closest?.(".theme-opt");
    if (opt) {
      e.preventDefault();
      e.stopPropagation();
      setTheme(opt.getAttribute("data-theme"));
    }
  });
  $("profileModal")?.addEventListener("change", (e) => {
    const t = e.target;
    if (!(t instanceof HTMLElement)) return;
    if (t.id === "settingsFocus") {
      if ($("myFocus")) $("myFocus").value = t.value;
      postFocus(t.value, mainPanel?.room || "");
      toast("Odak: " + (FOCUS_LABEL[t.value] || t.value));
    }
    if (t.id === "settingsSound") {
      soundOn = !!t.checked;
      document.querySelectorAll(".mute-btn").forEach((b) => {
        b.textContent = soundOn ? "🔔" : "🔕";
      });
    }
  });
  $("settingsNotify")?.addEventListener("click", async () => {
    if (!("Notification" in window)) { toast("Bu tarayıcı bildirim desteklemiyor"); return; }
    const perm = await Notification.requestPermission();
    toast(perm === "granted" ? "Bildirimler açık" : "Bildirim izni verilmedi");
  });
  $("settingsLogout")?.addEventListener("click", () => {
    store.clear();
    location.reload();
  });
  $("peopleSearch").addEventListener("input", (e) => renderPeople(e.target.value));
  function setRoomCreateOpen(open) {
    const panel = $("roomCreate");
    const section = panel?.closest(".nav-section");
    panel.classList.toggle("hidden", !open);
    section?.classList.toggle("expanded", open);
    if (open) {
      requestAnimationFrame(() => {
        panel.scrollIntoView({ block: "end", behavior: "smooth" });
        $("newRoom")?.focus();
      });
    }
  }

  $("toggleRoomCreate").onclick = () => {
    setRoomCreateOpen($("roomCreate").classList.contains("hidden"));
  };
  $("cancelRoomCreate").onclick = () => setRoomCreateOpen(false);
  $("addRoom").onclick = async () => {
    const name = $("newRoom").value.trim().toLowerCase();
    if (!name) return;
    let ttl = $("roomTTL")?.value || "";
    let kind = $("roomKind")?.value || "room";
    if ((kind === "timed" || kind === "event") && !ttl) ttl = "2h";
    if (ttl && kind === "room") kind = "timed";
    const res = await fetch("/api/rooms", {
      method: "POST", headers: authHeaders(),
      body: JSON.stringify({ name, ttl, kind }),
    });
    if (!res.ok) { alert(await res.text()); return; }
    $("newRoom").value = "";
    if ($("roomTTL")) $("roomTTL").value = "";
    if ($("roomKind")) $("roomKind").value = "room";
    setRoomCreateOpen(false);
    await loadRooms();
    openRoom(name);
  };

  $("themeToggle")?.addEventListener("click", () => {
    setTheme(theme === "light" ? "dark" : "light");
  });
  $("myFocus")?.addEventListener("change", (e) => {
    postFocus(e.target.value, mainPanel?.room || "");
  });
  let searchTimer = null;
  $("globalSearch")?.addEventListener("input", (e) => {
    clearTimeout(searchTimer);
    const q = e.target.value.trim();
    searchTimer = setTimeout(async () => {
      const hits = $("searchHits");
      if (!hits) return;
      hits.innerHTML = "";
      if (q.length < 2) return;
      const res = await fetch("/api/search?q=" + encodeURIComponent(q), { headers: authHeaders() });
      if (!res.ok) return;
      const list = await res.json();
      list.forEach((h) => {
        hits.appendChild(navButton({
          prefix: h.room.startsWith("dm:") ? "@" : "#",
          label: (h.content || "").slice(0, 48),
          sub: h.user + " · " + h.room,
          onClick: () => {
            if (h.room.startsWith("dm:")) {
              const mainIsPublic = mainPanel && mainPanel.room && !mainPanel.room.startsWith("dm:");
              if (mainIsPublic) openSide(h.room);
              else openRoom(h.room);
            } else openRoom(h.room);
          },
        }));
      });
    }, 250);
  });
  $("inviteBtn")?.addEventListener("click", async () => {
    const room = mainPanel?.room;
    if (!room || room.startsWith("dm:")) { toast("Önce bir oda aç"); return; }
    const res = await fetch("/api/invite", {
      method: "POST", headers: authHeaders(),
      body: JSON.stringify({ room }),
    });
    if (!res.ok) { alert(await res.text()); return; }
    const data = await res.json();
    const url = location.origin + data.url;
    try { await navigator.clipboard.writeText(url); } catch (_) {}
    toast("Davet kopyalandı: " + data.code);
  });

  applyTheme();

  // invite deep link
  (async () => {
    const code = new URLSearchParams(location.search).get("invite");
    if (!code || !store.token) return;
    const res = await fetch("/api/invite?code=" + encodeURIComponent(code), { headers: authHeaders() });
    if (res.ok) {
      const data = await res.json();
      if (data.room) openRoom(data.room);
    }
  })();

  $("profileClose").onclick = closeProfile;
  $("profileModal").addEventListener("click", (e) => { if (e.target === $("profileModal")) closeProfile(); });
  $("profileDm").onclick = () => {
    const u = profileUser;
    closeProfile();
    if (u) openDm(u);
  };
  $("profileMute").onclick = toggleMute;
  $("profileBlock").onclick = toggleBlock;
  document.querySelectorAll(".tab-btn").forEach((btn) => {
    btn.onclick = () => setProfileTab(btn.dataset.tab);
  });
  document.querySelector(".demo-chip")?.addEventListener("click", () => {
    $("username").value = "alice";
    $("password").value = "demo123";
  });

  (async () => {
    if (!store.token) return;
    const res = await fetch("/api/me", { headers: authHeaders() });
    if (!res.ok) { store.clear(); return; }
    store.user = (await res.json()).username;
    showApp();
  })();
})();
