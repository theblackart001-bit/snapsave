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
    .input-wrap button {
      padding:12px 20px; font-size:15px; font-weight:600;
      color:white; background:var(--accent); border:none; cursor:pointer; transition:background 0.2s;
    }
    .input-wrap button:hover { background:var(--accent-hover); }
    .input-wrap button:disabled { opacity:0.4; cursor:not-allowed; }
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
    }
    .dl-btn:hover { border-color:var(--btn-color); background:color-mix(in srgb, var(--btn-color) 6%, transparent); }
    .dl-btn:disabled { opacity:0.5; cursor:not-allowed; }
    .dl-btn svg { width:18px; height:18px; }
    .dl-btn .label { font-size:13px; font-weight:500; color:var(--text-primary); }
    .dl-btn .sublabel { font-size:11px; color:var(--text-secondary); }
    .spinner { width:20px; height:20px; border:2px solid currentColor; border-top-color:transparent; border-radius:50%; animation:spin 0.6s linear infinite; }
    .skeleton { display:flex; gap:14px; padding:14px; }
    .skeleton .thumb { width:140px; height:90px; border-radius:10px; flex-shrink:0; }
    .skeleton .lines { flex:1; display:flex; flex-direction:column; gap:12px; }
    .skeleton .line { height:20px; border-radius:4px; }
    .shimmer { background:linear-gradient(90deg,var(--bg-tertiary) 25%,var(--border) 50%,var(--bg-tertiary) 75%); background-size:200% 100%; animation:shimmer 1.5s infinite; }
    .footer { margin-top:auto; padding-top:8px; padding-bottom:8px; text-align:center; font-size:11px; color:var(--text-secondary); opacity:0.4; }
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
    <p>YouTube, Instagram, TikTok, Threads, Facebook</p>
    <div class="platforms">
      <span class="platform-icon" style="background:#ff000020;color:#ff0000;border:1px solid #ff000030" title="YouTube">▶</span>
      <span class="platform-icon" style="background:#e1306c20;color:#e1306c;border:1px solid #e1306c30" title="Instagram">📷</span>
      <span class="platform-icon" style="background:#00f2ea20;color:#00f2ea;border:1px solid #00f2ea30" title="TikTok">♪</span>
      <span class="platform-icon" style="background:#ffffff20;color:#ffffff;border:1px solid #ffffff30" title="Threads">@</span>
      <span class="platform-icon" style="background:#1877f220;color:#1877f2;border:1px solid #1877f230" title="Facebook">f</span>
    </div>
  </div>

  <div class="input-wrap fade-up" style="animation-delay:0.1s">
    <input id="urlInput" type="url" placeholder="영상 URL을 붙여넣으세요...">
    <button id="analyzeBtn">분석</button>
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
      <button class="dl-btn" id="dlVideo" style="--btn-color:#6c5ce7"><svg viewBox="0 0 24 24" fill="none" stroke="#6c5ce7" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg><span class="label">영상 다운로드</span><span class="sublabel">MP4 1080p</span></button>
      <button class="dl-btn" id="dlAudio" style="--btn-color:#51cf66"><svg viewBox="0 0 24 24" fill="none" stroke="#51cf66" stroke-width="2"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg><span class="label">음원 추출</span><span class="sublabel">MP3</span></button>
      <button class="dl-btn" id="dlThumb" style="--btn-color:#74b9ff"><svg viewBox="0 0 24 24" fill="none" stroke="#74b9ff" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg><span class="label">썸네일</span><span class="sublabel">JPG</span></button>
    </div>
  </div>

  <div id="setupOverlay" class="hidden">
    <h1>SnapSave</h1>
    <p id="setupMsg" style="color:var(--text-secondary);font-size:16px;">필요한 도구를 준비하는 중...</p>
    <div class="progress-wrap"><div id="setupBar" class="progress-bar"></div></div>
    <p id="setupPct" style="color:var(--text-secondary);font-size:13px;"></p>
  </div>

  <div class="footer" style="opacity:1;">
    <a href="https://litt.ly/booupplan" target="_blank" style="display:inline-block;padding:8px 20px;border-radius:8px;background:linear-gradient(135deg,#6c5ce7,#a29bfe);color:#fff;text-decoration:none;font-size:12px;font-weight:600;letter-spacing:0.3px;transition:all 0.3s;box-shadow:0 2px 12px rgba(108,92,231,0.3);" onmouseenter="this.style.transform='scale(1.03)';this.style.boxShadow='0 4px 20px rgba(108,92,231,0.5)'" onmouseleave="this.style.transform='scale(1)';this.style.boxShadow='0 2px 12px rgba(108,92,231,0.3)'">🔥 아직도 영상만 다운받고 계신가요? 월 300만원 버는 유튜브 비법 →</a>
  </div>

  <script>
    const COLORS = { YouTube:"#ff0000", Instagram:"#e1306c", TikTok:"#00f2ea", Threads:"#ffffff", Facebook:"#1877f2" };
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

    function fmt(sec) {
      const h=Math.floor(sec/3600), m=Math.floor((sec%3600)/60), s=sec%60;
      if(h>0) return h+":"+String(m).padStart(2,"0")+":"+String(s).padStart(2,"0");
      return m+":"+String(s).padStart(2,"0");
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
        $("skeleton").classList.add("hidden");
        const c = COLORS[info.platform]||"#6c5ce7";
        $("cardThumb").src=info.thumbnail;
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
      btn.innerHTML='<div class="spinner" style="color:var(--text-secondary)"></div><span class="label">다운로드 중...</span><span class="sublabel">잠시만 기다려주세요</span>';
      clear();
      try {
        await api("download",{url:currentUrl,type,quality:type==="video"?"1080p":undefined});
        showSuccess("다운로드 완료! 다운로드 폴더를 확인하세요.");
      } catch(e) { showError(e.message); }
      finally { btn.disabled=false; btn.innerHTML=orig; }
    }

    $("analyzeBtn").addEventListener("click", analyze);
    $("urlInput").addEventListener("keydown", e=>{ if(e.key==="Enter") analyze(); });
    $("dlVideo").addEventListener("click", ()=>dl("video"));
    $("dlAudio").addEventListener("click", ()=>dl("audio"));
    $("dlThumb").addEventListener("click", ()=>dl("thumbnail"));

    // Open external links in system browser
    document.addEventListener("click", e => {
      const a = e.target.closest("a[href]");
      if (a && a.href.startsWith("http")) { e.preventDefault(); window.open(a.href); }
    });
  </script>
</body>
</html>`
