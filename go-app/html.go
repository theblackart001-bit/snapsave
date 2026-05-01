package main

const indexHTML = `<!DOCTYPE html>
<html lang="ko">
<head>
  <meta charset="UTF-8">
  <title>SnapSave</title>
  <style>
    :root {
      --bg-primary: #0c0c0f; --bg-secondary: #16161a; --bg-tertiary: #1e1e24;
      --text-primary: #ffffffee; --text-secondary: #a0a0b0;
      --accent: #6c5ce7; --accent-hover: #7c6cf7;
      --border: #2a2a35; --danger: #ff6b6b; --success: #51cf66;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: var(--bg-primary); color: var(--text-primary);
      font-family: 'Segoe UI', system-ui, sans-serif;
      height: 100vh; display: flex; flex-direction: column; align-items: center;
      padding: 24px 16px 12px; -webkit-font-smoothing: antialiased; user-select: none;
      overflow: hidden;
    }
    @keyframes fadeUp { from { opacity:0; transform:translateY(16px); } to { opacity:1; transform:translateY(0); } }
    @keyframes spin { to { transform:rotate(360deg); } }
    @keyframes shimmer { 0% { background-position:-200% 0; } 100% { background-position:200% 0; } }
    .fade-up { animation: fadeUp 0.5s ease-out forwards; }
    .header { text-align:center; margin-bottom:16px; }
    .header h1 {
      font-size:36px; font-weight:700; letter-spacing:-1px; margin-bottom:6px;
      background:linear-gradient(135deg,#6c5ce7,#a29bfe,#74b9ff);
      -webkit-background-clip:text; -webkit-text-fill-color:transparent;
    }
    .header p { color:var(--text-secondary); font-size:14px; }
    .platforms { display:flex; gap:10px; justify-content:center; margin-top:10px; }
    .platform-icon {
      width:30px; height:30px; border-radius:50%;
      display:flex; align-items:center; justify-content:center;
      font-size:14px; font-weight:700;
    }
    .input-wrap {
      width:100%; max-width:640px; display:flex; border-radius:16px; overflow:hidden;
      background:var(--bg-secondary); border:1px solid var(--border);
    }
    .input-wrap input {
      flex:1; padding:12px 16px; font-size:15px;
      background:transparent; border:none; outline:none; color:var(--text-primary);
    }
    .input-wrap input::placeholder { color:var(--text-secondary); }
    .clear-btn {
      padding:0 14px; font-size:18px; color:var(--text-secondary); background:transparent;
      border:none; cursor:pointer; transition:color 0.2s; display:none;
    }
    .clear-btn:hover { color:var(--text-primary); }
    .input-wrap button.analyze {
      padding:12px 20px; font-size:15px; font-weight:600;
      color:white; background:var(--accent); border:none; cursor:pointer; transition:background 0.2s;
    }
    .input-wrap button.analyze:hover { background:var(--accent-hover); }
    .input-wrap button.analyze:disabled { opacity:0.4; cursor:not-allowed; }
    .message { margin-top:10px; padding:8px 16px; border-radius:10px; font-size:13px; width:100%; max-width:640px; }
    .message.error { background:rgba(255,107,107,0.1); border:1px solid rgba(255,107,107,0.2); color:var(--danger); }
    .message.success { background:rgba(81,207,102,0.1); border:1px solid rgba(81,207,102,0.2); color:var(--success); }
    .card { margin-top:16px; width:100%; max-width:640px; border-radius:14px; overflow:hidden; background:var(--bg-secondary); border:1px solid var(--border); }
    .card-top { display:flex; }
    .card-thumb { width:180px; flex-shrink:0; position:relative; background:var(--bg-tertiary); }
    .card-thumb img { width:100%; height:100%; object-fit:cover; max-height:140px; }
    .card-duration { position:absolute; bottom:8px; right:8px; padding:2px 8px; border-radius:4px; font-size:12px; font-family:monospace; font-weight:500; background:rgba(0,0,0,0.8); color:#fff; }
    .card-info { flex:1; padding:14px; }
    .card-platform { display:inline-block; padding:2px 8px; border-radius:999px; font-size:12px; font-weight:700; margin-bottom:8px; }
    .card-title { font-size:15px; font-weight:600; line-height:1.3; margin-bottom:6px; color:var(--text-primary); display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; }
    .card-uploader { font-size:13px; color:var(--text-secondary); }
    .card-controls { padding:12px; border-top:1px solid var(--border); display:grid; grid-template-columns:repeat(3,1fr); gap:8px; }
    .dl-btn {
      display:flex; flex-direction:column; align-items:center; gap:4px;
      padding:10px 8px; border-radius:10px; border:1px solid var(--border);
      background:var(--bg-tertiary); cursor:pointer; transition:all 0.2s;
      position:relative; overflow:hidden;
    }
    .dl-btn:hover { border-color:var(--btn-color); background:color-mix(in srgb, var(--btn-color) 6%, transparent); }
    .dl-btn:disabled { opacity:0.85; cursor:not-allowed; }
    .dl-btn svg { width:18px; height:18px; }
    .dl-btn .label { font-size:13px; font-weight:500; color:var(--text-primary); position:relative; z-index:1; }
    .dl-btn .sublabel { font-size:11px; color:var(--text-secondary); position:relative; z-index:1; }
    /* Progress fill — animates left-to-right behind label/sublabel during download */
    .dl-btn .dl-fill {
      position:absolute; left:0; top:0; bottom:0; width:0%;
      background:color-mix(in srgb, var(--btn-color) 22%, transparent);
      transition:width 0.25s ease-out; z-index:0; pointer-events:none;
    }
    .spinner { width:20px; height:20px; border:2px solid currentColor; border-top-color:transparent; border-radius:50%; animation:spin 0.6s linear infinite; }
    .skeleton { display:flex; gap:14px; padding:14px; }
    .skeleton .thumb { width:140px; height:90px; border-radius:10px; flex-shrink:0; }
    .skeleton .lines { flex:1; display:flex; flex-direction:column; gap:12px; }
    .skeleton .line { height:20px; border-radius:4px; }
    .shimmer { background:linear-gradient(90deg,var(--bg-tertiary) 25%,var(--border) 50%,var(--bg-tertiary) 75%); background-size:200% 100%; animation:shimmer 1.5s infinite; }
    .footer { margin-top:auto; padding-top:8px; padding-bottom:8px; text-align:center; font-size:11px; color:var(--text-secondary); opacity:0.4; }
    .history-toggle {
      position:fixed; top:16px; right:16px; width:40px; height:40px; border-radius:12px;
      background:var(--bg-secondary); border:1px solid var(--border); cursor:pointer;
      display:flex; align-items:center; justify-content:center; font-size:18px;
      color:var(--text-secondary); transition:all 0.2s; z-index:10;
    }
    .history-toggle:hover { border-color:var(--accent); color:var(--text-primary); }
    .history-panel {
      position:fixed; top:0; right:-400px; width:380px; height:100vh;
      background:var(--bg-primary); border-left:1px solid var(--border);
      z-index:100; transition:right 0.3s ease; display:flex; flex-direction:column;
    }
    .history-panel.open { right:0; }
    .history-header {
      display:flex; align-items:center; justify-content:space-between;
      padding:16px 20px; border-bottom:1px solid var(--border);
    }
    .history-header h3 { font-size:16px; font-weight:600; }
    .history-header-btns { display:flex; gap:8px; }
    .history-header-btns button {
      padding:6px 12px; border-radius:8px; font-size:12px; font-weight:500;
      border:1px solid var(--border); background:var(--bg-secondary); color:var(--text-secondary);
      cursor:pointer; transition:all 0.2s;
    }
    .history-header-btns button:hover { color:var(--text-primary); border-color:var(--accent); }
    .history-list { flex:1; overflow-y:auto; padding:12px; }
    .history-item {
      display:flex; gap:10px; padding:10px; border-radius:10px; margin-bottom:8px;
      background:var(--bg-secondary); border:1px solid var(--border); cursor:default; transition:border-color 0.2s;
    }
    .history-item:hover { border-color:var(--accent); }
    .history-item img { width:60px; height:42px; border-radius:6px; object-fit:cover; flex-shrink:0; background:var(--bg-tertiary); }
    .history-item-info { flex:1; min-width:0; }
    .history-item-title { font-size:13px; font-weight:500; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
    .history-item-meta { font-size:11px; color:var(--text-secondary); margin-top:3px; display:flex; gap:8px; }
    .history-item-type {
      display:inline-block; padding:1px 6px; border-radius:4px; font-size:10px; font-weight:600;
    }
    .history-empty { text-align:center; color:var(--text-secondary); padding:40px 20px; font-size:14px; }
    .history-overlay { position:fixed; inset:0; background:rgba(0,0,0,0.4); z-index:99; display:none; }
    .history-overlay.open { display:block; }
    .hidden { display:none !important; }
    #setupOverlay { position:fixed; inset:0; background:var(--bg-primary); z-index:999; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:16px; }
    #setupOverlay h1 { font-size:36px; font-weight:700; background:linear-gradient(135deg,#6c5ce7,#a29bfe,#74b9ff); -webkit-background-clip:text; -webkit-text-fill-color:transparent; }
    .progress-wrap { width:300px; height:6px; border-radius:3px; background:var(--bg-tertiary); overflow:hidden; margin-top:8px; }
    .progress-bar { width:0%; height:100%; background:var(--accent); border-radius:3px; transition:width 0.3s; }
  </style>
</head>
<body>
  <div class="header fade-up">
    <h1>SnapSave</h1>
    <p>YouTube, Instagram, TikTok, Threads, Facebook, 샤오홍슈, 더우인</p>
    <div class="platforms">
      <span class="platform-icon" style="background:#ff000020;color:#ff0000;border:1px solid #ff000030" title="YouTube">▶</span>
      <span class="platform-icon" style="background:#e1306c20;color:#e1306c;border:1px solid #e1306c30" title="Instagram">📷</span>
      <span class="platform-icon" style="background:#00f2ea20;color:#00f2ea;border:1px solid #00f2ea30" title="TikTok">♪</span>
      <span class="platform-icon" style="background:#ffffff20;color:#ffffff;border:1px solid #ffffff30" title="Threads">@</span>
      <span class="platform-icon" style="background:#1877f220;color:#1877f2;border:1px solid #1877f230" title="Facebook">f</span>
      <span class="platform-icon" style="background:#ff220020;color:#ff2200;border:1px solid #ff220030" title="샤오홍슈">红</span>
      <span class="platform-icon" style="background:#00f0ff20;color:#00f0ff;border:1px solid #00f0ff30" title="더우인">抖</span>
    </div>
  </div>

  <div class="input-wrap fade-up" style="animation-delay:0.1s">
    <input id="urlInput" type="url" placeholder="영상 URL을 붙여넣으세요...">
    <button id="clearBtn" class="clear-btn" title="링크 지우기">✕</button>
    <button id="analyzeBtn" class="analyze">분석</button>
  </div>

  <div id="errorMsg" class="message error hidden fade-up"></div>
  <div id="successMsg" class="message success hidden fade-up"></div>

  <div id="skeleton" class="card hidden">
    <div class="skeleton"><div class="thumb shimmer"></div><div class="lines"><div class="line shimmer" style="width:75%"></div><div class="line shimmer" style="width:50%"></div><div class="line shimmer" style="width:33%"></div></div></div>
  </div>

  <div id="videoCard" class="card hidden fade-up">
    <div class="card-top">
      <div class="card-thumb"><img id="cardThumb" src="" alt=""><span id="cardDuration" class="card-duration"></span></div>
      <div class="card-info"><span id="cardPlatform" class="card-platform"></span><h2 id="cardTitle" class="card-title"></h2><p id="cardUploader" class="card-uploader"></p></div>
    </div>
    <div class="card-controls">
      <button class="dl-btn" id="dlVideo" style="--btn-color:#6c5ce7"><svg viewBox="0 0 24 24" fill="none" stroke="#6c5ce7" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg><span class="label">영상 다운로드</span><span class="sublabel">MP4 최고화질</span></button>
      <button class="dl-btn" id="dlAudio" style="--btn-color:#51cf66"><svg viewBox="0 0 24 24" fill="none" stroke="#51cf66" stroke-width="2"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg><span class="label">음원 추출</span><span class="sublabel">MP3</span></button>
      <button class="dl-btn" id="dlThumb" style="--btn-color:#74b9ff"><svg viewBox="0 0 24 24" fill="none" stroke="#74b9ff" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg><span class="label">썸네일</span><span class="sublabel">JPG</span></button>
    </div>
  </div>

  <button id="historyToggle" class="history-toggle" title="다운로드 기록">📋</button>
  <div id="historyOverlay" class="history-overlay"></div>
  <div id="historyPanel" class="history-panel">
    <div class="history-header">
      <h3>다운로드 기록</h3>
      <div class="history-header-btns">
        <button id="clearHistoryBtn">전체 삭제</button>
        <button id="closeHistoryBtn">닫기</button>
      </div>
    </div>
    <div id="historyList" class="history-list"></div>
  </div>

  <div id="setupOverlay" class="hidden">
    <h1>SnapSave</h1>
    <p id="setupMsg" style="color:var(--text-secondary);font-size:16px;">필요한 도구를 준비하는 중...</p>
    <div class="progress-wrap"><div id="setupBar" class="progress-bar"></div></div>
    <p id="setupPct" style="color:var(--text-secondary);font-size:13px;"></p>
  </div>

  <div class="footer" style="opacity:1;display:flex;flex-direction:column;align-items:center;gap:2px;">
    <div style="animation:bounce 1s ease-in-out infinite;font-size:20px;line-height:1;">
      <span style="color:#ff4757;filter:drop-shadow(0 0 4px rgba(255,71,87,0.5));">▼</span>
      <span style="color:#ff4757;filter:drop-shadow(0 0 4px rgba(255,71,87,0.5));">▼</span>
      <span style="color:#ff4757;filter:drop-shadow(0 0 4px rgba(255,71,87,0.5));">▼</span>
    </div>
    <a href="https://litt.ly/booupplan" target="_blank" style="display:inline-block;padding:10px 24px;border-radius:10px;background:linear-gradient(135deg,#ff4757,#ff6b81);color:#fff;text-decoration:none;font-size:13px;font-weight:700;letter-spacing:0.3px;transition:all 0.3s;box-shadow:0 2px 16px rgba(255,71,87,0.4);animation:pulse-btn 2s ease-in-out infinite;" onmouseenter="this.style.transform='scale(1.05)'" onmouseleave="this.style.transform='scale(1)'">🔥 아직도 영상만 다운받고 계신가요? 월 300 버는 유튜브 비법 →</a>
  </div>
  <style>
    @keyframes bounce { 0%,100%{transform:translateY(0)} 50%{transform:translateY(5px)} }
    @keyframes pulse-btn { 0%,100%{box-shadow:0 2px 16px rgba(255,71,87,0.4)} 50%{box-shadow:0 4px 24px rgba(255,71,87,0.7)} }
  </style>

  <script>
    const COLORS = { YouTube:"#ff0000", Instagram:"#e1306c", TikTok:"#00f2ea", Threads:"#ffffff", Facebook:"#1877f2", Xiaohongshu:"#ff2200", Douyin:"#00f0ff" };
    const $=id=>document.getElementById(id);

    async function api(path, body) {
      const res = await fetch("/api/" + path, { method:"POST", headers:{"Content-Type":"application/json"}, body:JSON.stringify(body) });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "오류 발생");
      return data;
    }

    // Setup check
    (async()=>{
      const r = await fetch("/api/check-setup"); const d = await r.json();
      if (!d.ready) {
        const o = $("setupOverlay"); o.classList.remove("hidden"); o.style.display="flex";
        $("setupMsg").textContent = "yt-dlp, ffmpeg를 다운로드하는 중... (첫 실행만)";
        $("setupBar").style.width = "30%";
        try {
          const r2 = await fetch("/api/setup", {method:"POST"}); const d2 = await r2.json();
          if (d2.ready) { $("setupBar").style.width = "100%"; $("setupPct").textContent = "완료!"; }
          else { $("setupMsg").textContent = d2.error || "설치 실패"; return; }
        } catch(e) { $("setupMsg").textContent = e.message; return; }
        setTimeout(()=>{ o.style.display="none"; }, 500);
      }
    })();

    let currentUrl = "";
    let currentInfo = {};

    function fmt(sec) {
      const h=Math.floor(sec/3600), m=Math.floor((sec%3600)/60), s=sec%60;
      if(h>0) return h+":"+String(m).padStart(2,"0")+":"+String(s).padStart(2,"0");
      return m+":"+String(s).padStart(2,"0");
    }

    // Hosts whose thumbnails block hotlinking — must go through /api/thumb.
    const PROXY_HOSTS = ["cdninstagram.com","fbcdn.net","fbsbx.com","tiktokcdn.com","tiktokcdn-us.com","muscdn.com","ttwstatic.com","xhscdn.com","douyinpic.com","douyinstatic.com"];
    function thumbProxy(rawUrl) {
      if (!rawUrl) return "";
      // Server-resolved local URLs (e.g. /api/local-thumb?id=...) are passed through.
      if (rawUrl.startsWith("/api/")) return rawUrl;
      try {
        const u = new URL(rawUrl);
        const host = u.hostname.toLowerCase();
        const needsProxy = PROXY_HOSTS.some(h => host === h || host.endsWith("." + h));
        return needsProxy ? "/api/thumb?url=" + encodeURIComponent(rawUrl) : rawUrl;
      } catch (_) {
        return rawUrl;
      }
    }

    function showError(m) { $("errorMsg").textContent=m; $("errorMsg").classList.remove("hidden"); $("successMsg").classList.add("hidden"); }
    function showSuccess(m) { $("successMsg").textContent=m; $("successMsg").classList.remove("hidden"); $("errorMsg").classList.add("hidden"); setTimeout(()=>$("successMsg").classList.add("hidden"),5000); }
    function clear() { $("errorMsg").classList.add("hidden"); $("successMsg").classList.add("hidden"); }

    async function analyze() {
      const url = $("urlInput").value.trim(); if(!url) return;
      currentUrl = url; clear();
      $("videoCard").classList.add("hidden"); $("skeleton").classList.remove("hidden");
      $("analyzeBtn").disabled=true; $("analyzeBtn").textContent="분석 중...";
      try {
        const info = await api("info", {url});
        currentInfo = info;
        $("skeleton").classList.add("hidden");
        const c = COLORS[info.platform]||"#6c5ce7";
        $("cardThumb").src = thumbProxy(info.thumbnail);
        $("cardDuration").textContent=info.duration>0?fmt(info.duration):"";
        $("cardDuration").style.display=info.duration>0?"":"none";
        $("cardPlatform").textContent=info.platform;
        $("cardPlatform").style.cssText="display:inline-block;padding:2px 8px;border-radius:999px;font-size:12px;font-weight:700;margin-bottom:8px;background:"+c+"20;color:"+c+";border:1px solid "+c+"30";
        $("cardTitle").textContent=info.title;
        $("cardUploader").textContent=info.uploader;
        $("videoCard").classList.remove("hidden");
      } catch(e) { $("skeleton").classList.add("hidden"); showError(e.message); }
      finally { $("analyzeBtn").disabled=false; $("analyzeBtn").textContent="분석"; }
    }

    async function dl(type) {
      if(!currentUrl) return;
      const btn = type==="video"?$("dlVideo"):type==="audio"?$("dlAudio"):$("dlThumb");
      const orig=btn.innerHTML;
      btn.disabled=true;
      btn.innerHTML='<div class="dl-fill" style="width:0%"></div><span class="label">시작 중...</span><span class="sublabel">0%</span>';
      const fill = btn.querySelector(".dl-fill");
      const labelEl = btn.querySelector(".label");
      const subEl = btn.querySelector(".sublabel");
      clear();

      const setProgress = (pct, stageText) => {
        const p = Math.max(0, Math.min(100, pct));
        if (fill) fill.style.width = p.toFixed(1) + "%";
        if (subEl) subEl.textContent = Math.round(p) + "%";
        if (labelEl && stageText) labelEl.textContent = stageText;
      };

      try {
        const res = await fetch("/api/download-stream", {
          method:"POST",
          headers:{"Content-Type":"application/json"},
          body: JSON.stringify({
            url:currentUrl, type,
            title:currentInfo.title||"", platform:currentInfo.platform||"",
            thumbnail:currentInfo.thumbnail||""
          })
        });
        if (!res.ok || !res.body) throw new Error("다운로드 시작 실패");

        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";
        let lastError = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split("\n");
          buf = lines.pop() || "";
          for (const line of lines) {
            const t = line.trim();
            if (!t) continue;
            let evt;
            try { evt = JSON.parse(t); } catch(_) { continue; }
            if (evt.type === "progress") {
              setProgress(evt.percent || 0, evt.stageText || "");
            } else if (evt.type === "error") {
              lastError = evt.message || "다운로드 실패";
            }
          }
        }

        if (lastError) throw new Error(lastError);
        setProgress(100, "완료");
        showSuccess("다운로드 완료! 다운로드 폴더를 확인하세요.");
      } catch(e) {
        showError(e.message || "다운로드 실패");
      } finally {
        // Brief pause so the 100% bar is visible, then restore button.
        setTimeout(() => { btn.disabled=false; btn.innerHTML=orig; }, 600);
      }
    }

    $("analyzeBtn").addEventListener("click", analyze);
    $("urlInput").addEventListener("keydown", e=>{ if(e.key==="Enter") analyze(); });
    $("urlInput").addEventListener("input", ()=>{ $("clearBtn").style.display = $("urlInput").value ? "block" : "none"; });
    $("clearBtn").addEventListener("click", ()=>{ $("urlInput").value=""; $("clearBtn").style.display="none"; $("urlInput").focus(); });
    $("dlVideo").addEventListener("click", ()=>dl("video"));
    $("dlAudio").addEventListener("click", ()=>dl("audio"));
    $("dlThumb").addEventListener("click", ()=>dl("thumbnail"));
    // Auto-update is handled at startup (main.go gates the WebView on
    // hasStartupUpdate). The main UI no longer needs an in-app update banner.

    // History panel
    const TYPE_COLORS = { "영상":"#6c5ce7", "음원":"#51cf66", "썸네일":"#74b9ff" };
    function toggleHistory(open) {
      $("historyPanel").classList.toggle("open", open);
      $("historyOverlay").classList.toggle("open", open);
      if (open) loadHistoryUI();
    }
    async function loadHistoryUI() {
      const res = await fetch("/api/history"); const list = await res.json();
      const el = $("historyList");
      if (!list || list.length === 0) { el.innerHTML='<div class="history-empty">다운로드 기록이 없습니다</div>'; return; }
      el.innerHTML = list.map(h => {
        const c = COLORS[h.platform] || "#6c5ce7";
        const tc = TYPE_COLORS[h.type] || "#6c5ce7";
        return '<div class="history-item">'
          + (h.thumbnail ? '<img src="'+thumbProxy(h.thumbnail)+'" alt="">' : '')
          + '<div class="history-item-info">'
          + '<div class="history-item-title">'+(h.title||"제목 없음")+'</div>'
          + '<div class="history-item-meta">'
          + '<span class="history-item-type" style="background:'+tc+'20;color:'+tc+'">'+h.type+'</span>'
          + '<span style="color:'+c+'">'+h.platform+'</span>'
          + '<span>'+h.date+'</span>'
          + '</div></div></div>';
      }).join("");
    }
    $("historyToggle").addEventListener("click", ()=>toggleHistory(true));
    $("closeHistoryBtn").addEventListener("click", ()=>toggleHistory(false));
    $("historyOverlay").addEventListener("click", ()=>toggleHistory(false));
    $("clearHistoryBtn").addEventListener("click", async ()=>{
      await fetch("/api/history/clear",{method:"POST"});
      loadHistoryUI();
    });

    // Open external links in system browser
    document.addEventListener("click", e => {
      const a = e.target.closest("a[href]");
      if (a && a.href.startsWith("http")) { e.preventDefault(); window.open(a.href); }
    });
  </script>
</body>
</html>`
