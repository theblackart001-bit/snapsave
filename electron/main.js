const { app, BrowserWindow, shell, dialog, ipcMain } = require("electron");
const { execFile } = require("child_process");
const { autoUpdater } = require("electron-updater");
const path = require("path");
const fs = require("fs");
const os = require("os");
const https = require("https");

app.disableHardwareAcceleration();
app.commandLine.appendSwitch("disable-gpu");

let mainWindow;
const isDev = !app.isPackaged;

// Tools stored in AppData/snapsave-bin
const binDir = isDev
  ? path.join(__dirname, "..", "bin")
  : path.join(app.getPath("userData"), "bin");

const ytdlpPath = path.join(binDir, "yt-dlp.exe");
const ffmpegPath = path.join(binDir, "ffmpeg.exe");

// ── Download helpers ──

function downloadFile(url, dest, onProgress) {
  return new Promise((resolve, reject) => {
    const follow = (u) => {
      https.get(u, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return follow(res.headers.location);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`HTTP ${res.statusCode}`));
        }
        const total = parseInt(res.headers["content-length"], 10) || 0;
        let downloaded = 0;
        const file = fs.createWriteStream(dest);
        res.on("data", (chunk) => {
          downloaded += chunk.length;
          file.write(chunk);
          if (total && onProgress) onProgress(downloaded, total);
        });
        res.on("end", () => { file.end(() => resolve()); });
        res.on("error", reject);
      }).on("error", reject);
    };
    follow(url);
  });
}

async function ensureTools(progressCb) {
  if (!fs.existsSync(binDir)) fs.mkdirSync(binDir, { recursive: true });

  const need = [];
  if (!fs.existsSync(ytdlpPath)) need.push("yt-dlp");
  if (!fs.existsSync(ffmpegPath)) need.push("ffmpeg");
  if (need.length === 0) return;

  // Download yt-dlp
  if (need.includes("yt-dlp")) {
    progressCb("yt-dlp 다운로드 중...", 0);
    await downloadFile(
      "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe",
      ytdlpPath,
      (dl, tot) => progressCb("yt-dlp 다운로드 중...", Math.round((dl / tot) * 100))
    );
  }

  // Download ffmpeg (gyan.dev essentials build - smallest full build ~25MB zip)
  if (need.includes("ffmpeg")) {
    progressCb("ffmpeg 다운로드 중...", 0);
    const zipPath = path.join(binDir, "ffmpeg.zip");
    await downloadFile(
      "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip",
      zipPath,
      (dl, tot) => progressCb("ffmpeg 다운로드 중...", Math.round((dl / tot) * 100))
    );

    // Extract just ffmpeg.exe from zip
    progressCb("ffmpeg 압축 해제 중...", 0);
    const { execFileSync } = require("child_process");
    // Use PowerShell to extract
    execFileSync("powershell", [
      "-NoProfile", "-Command",
      `$zip = [System.IO.Compression.ZipFile]::OpenRead('${zipPath.replace(/'/g, "''")}');` +
      `$entry = $zip.Entries | Where-Object { $_.Name -eq 'ffmpeg.exe' } | Select-Object -First 1;` +
      `[System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, '${ffmpegPath.replace(/'/g, "''")}', $true);` +
      `$zip.Dispose()`
    ], { timeout: 120000 });

    // Cleanup zip
    try { fs.unlinkSync(zipPath); } catch {}
  }
}

// ── IPC: setup progress ──

ipcMain.handle("check-setup", async () => {
  return { ready: fs.existsSync(ytdlpPath) && fs.existsSync(ffmpegPath) };
});

ipcMain.handle("run-setup", async () => {
  await ensureTools((msg, pct) => {
    if (mainWindow) mainWindow.webContents.send("setup-progress", { msg, pct });
  });
  return { ready: true };
});

// ── yt-dlp runner ──

const COOKIE_BROWSERS = ["chrome", "edge", "firefox", "brave"];

function needsCookies(url) {
  return /instagram\.com|threads\.net|facebook\.com|fb\.watch/i.test(url);
}

function runYtdlp(args, opts = {}) {
  return new Promise((resolve, reject) => {
    const ffmpegDir = path.dirname(ffmpegPath);
    const cookieArgs = opts.cookieBrowser
      ? ["--cookies-from-browser", opts.cookieBrowser]
      : [];
    execFile(
      ytdlpPath,
      ["--ffmpeg-location", ffmpegDir, ...cookieArgs, ...args],
      { maxBuffer: 10 * 1024 * 1024, timeout: 120000 },
      (error, stdout, stderr) => {
        if (error) {
          const errorLines = (stderr || error.message)
            .split("\n")
            .filter((l) => !l.startsWith("WARNING:"))
            .join("\n")
            .trim();
          reject(new Error(errorLines || "다운로드 중 오류가 발생했습니다"));
        } else {
          resolve(stdout);
        }
      }
    );
  });
}

// Friendly error mapper — runs after every cookie attempt has failed.
function friendlyError(err, url) {
  const msg = (err && err.message ? err.message : String(err)).toLowerCase();
  if (
    msg.includes("not available to everyone") ||
    msg.includes("can't be seen") ||
    msg.includes("cannot be seen")
  ) {
    return new Error(
      "이 콘텐츠는 작성자가 일부 대상에게만 공개한 게시물입니다. " +
      "(예: 친한 친구 전용·지역 제한·연령 제한). " +
      "해당 계정을 팔로우하거나 대상에 포함된 인스타그램에 로그인된 브라우저가 있어야 다운로드할 수 있습니다."
    );
  }
  if (msg.includes("login required") || msg.includes("requires authentication")) {
    return new Error(
      "로그인이 필요한 콘텐츠입니다. Chrome/Edge/Firefox/Brave 중 하나에 인스타그램·페이스북 계정으로 로그인한 뒤 다시 시도하세요."
    );
  }
  if (msg.includes("rate-limit") || msg.includes("rate limit")) {
    return new Error("플랫폼에서 일시적으로 요청을 제한했습니다. 잠시 후 다시 시도하세요.");
  }
  if (msg.includes("private")) {
    return new Error("비공개 게시물입니다. 접근 권한이 있는 계정으로 로그인된 브라우저가 필요합니다.");
  }
  return err;
}

// Try each browser's cookies until one succeeds. For platforms that usually
// need login, try with cookies first; otherwise try plain first to keep YouTube fast.
async function runYtdlpWithCookieFallback(url, args) {
  const order = needsCookies(url)
    ? [...COOKIE_BROWSERS, undefined]
    : [undefined, ...COOKIE_BROWSERS];

  let lastError = null;
  for (const browser of order) {
    try {
      return await runYtdlp(args, { cookieBrowser: browser });
    } catch (e) {
      const err = e instanceof Error ? e : new Error(String(e));
      const msg = err.message.toLowerCase();
      lastError = err;

      // Cookie extraction failed → try next browser
      if (
        msg.includes("could not copy") ||
        msg.includes("could not find") ||
        msg.includes("unable to read") ||
        msg.includes("permission denied") ||
        msg.includes("no such file") ||
        msg.includes("no profiles found") ||
        msg.includes("could not load")
      ) {
        continue;
      }

      // Login/visibility errors → a different browser's session may work
      if (
        msg.includes("login required") ||
        msg.includes("requires authentication") ||
        msg.includes("rate-limit") ||
        msg.includes("not available to everyone") ||
        msg.includes("can't be seen") ||
        msg.includes("private")
      ) {
        continue;
      }

      throw friendlyError(err, url);
    }
  }
  throw friendlyError(lastError ?? new Error("다운로드 중 오류가 발생했습니다"), url);
}

function detectPlatform(url) {
  if (/youtube\.com|youtu\.be/i.test(url)) return "YouTube";
  if (/instagram\.com/i.test(url)) return "Instagram";
  if (/tiktok\.com/i.test(url)) return "TikTok";
  if (/threads\.net/i.test(url)) return "Threads";
  if (/facebook\.com|fb\.watch/i.test(url)) return "Facebook";
  return "Unknown";
}

// IPC: get video info
ipcMain.handle("get-info", async (_event, url) => {
  const raw = await runYtdlpWithCookieFallback(url, ["-j", "--no-playlist", url]);
  const data = JSON.parse(raw);
  return {
    id: data.id,
    title: data.title || "Untitled",
    thumbnail: data.thumbnail || "",
    duration: data.duration || 0,
    uploader: data.uploader || data.channel || "Unknown",
    platform: detectPlatform(url),
  };
});

// IPC: get available video qualities (probe formats)
ipcMain.handle("get-qualities", async (_event, url) => {
  try {
    const raw = await runYtdlpWithCookieFallback(url, ["-j", "--no-playlist", url]);
    const data = JSON.parse(raw);
    const formats = Array.isArray(data.formats) ? data.formats : [];
    const heights = new Set();
    for (const f of formats) {
      if (f.vcodec && f.vcodec !== "none" && typeof f.height === "number") {
        heights.add(f.height);
      }
    }
    const qualities = Array.from(heights).sort((a, b) => b - a);
    return { qualities };
  } catch (err) {
    console.error("get-qualities failed", err);
    return { qualities: [], error: err.message };
  }
});

// CDNs that block hotlinking from arbitrary origins — must be fetched server-side
// with a same-origin Referer.
const PROXIED_THUMBNAIL_HOSTS = [
  "cdninstagram.com",
  "fbcdn.net",
  "tiktokcdn.com",
  "tiktokcdn-us.com",
  "muscdn.com",
  "ttwstatic.com",
  "xhscdn.com",
  "douyinpic.com",
  "douyinstatic.com",
];

function shouldProxyThumbnail(hostname) {
  return PROXIED_THUMBNAIL_HOSTS.some(
    (h) => hostname === h || hostname.endsWith(`.${h}`)
  );
}

function fetchBuffer(targetUrl, headers) {
  return new Promise((resolve, reject) => {
    const follow = (u, depth) => {
      if (depth > 5) return reject(new Error("too many redirects"));
      const lib = u.startsWith("http://") ? require("http") : https;
      lib
        .get(u, { headers }, (res) => {
          if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
            return follow(new URL(res.headers.location, u).toString(), depth + 1);
          }
          if (res.statusCode !== 200) {
            return reject(new Error(`HTTP ${res.statusCode}`));
          }
          const chunks = [];
          res.on("data", (c) => chunks.push(c));
          res.on("end", () =>
            resolve({
              buffer: Buffer.concat(chunks),
              contentType: res.headers["content-type"] || "image/jpeg",
            })
          );
          res.on("error", reject);
        })
        .on("error", reject);
    };
    follow(targetUrl, 0);
  });
}

// IPC: fetch thumbnail bytes and return as a data URI so hotlink-blocking CDNs
// (Instagram, TikTok, FB) render correctly inside the renderer.
ipcMain.handle("get-thumbnail", async (_event, rawUrl) => {
  if (!rawUrl) return { dataUrl: "" };
  let parsed;
  try {
    parsed = new URL(rawUrl);
  } catch {
    return { dataUrl: rawUrl };
  }

  // YouTube thumbnails load fine from the renderer — skip the round-trip.
  if (!shouldProxyThumbnail(parsed.hostname)) {
    return { dataUrl: rawUrl };
  }

  try {
    const { buffer, contentType } = await fetchBuffer(rawUrl, {
      "User-Agent":
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
        "(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
      Referer: `${parsed.protocol}//${parsed.hostname}/`,
      Accept: "image/avif,image/webp,image/apng,image/*,*/*;q=0.8",
    });
    const dataUrl = `data:${contentType};base64,${buffer.toString("base64")}`;
    return { dataUrl };
  } catch (err) {
    console.error("get-thumbnail failed", err.message);
    return { dataUrl: "", error: err.message };
  }
});

// IPC: download media
ipcMain.handle("download", async (_event, opts) => {
  const downloadsDir = path.join(os.homedir(), "Downloads");
  const outputTemplate = path.join(downloadsDir, "%(title).80s.%(ext)s");

  if (opts.type === "thumbnail") {
    await runYtdlpWithCookieFallback(opts.url, [
      "--write-thumbnail", "--skip-download",
      "--convert-thumbnails", "jpg",
      "-o", path.join(downloadsDir, "%(title).80s"),
      "--no-playlist", opts.url,
    ]);
    return { success: true };
  }

  if (opts.type === "audio") {
    await runYtdlpWithCookieFallback(opts.url, [
      "-x", "--audio-format", "mp3", "--audio-quality", "0",
      "-o", outputTemplate, "--no-playlist", opts.url,
    ]);
  } else {
    const args = [];
    if (opts.quality === "best") {
      // 최고 화질 (4K·8K 포함 가용 최고)
      args.push("-f", "bestvideo+bestaudio/best");
    } else if (opts.quality) {
      const height = String(opts.quality).replace("p", "");
      // 지정 해상도 이하 + 오디오 병합, 실패 시 단일 best 파일로 fallback
      args.push(
        "-f",
        `bestvideo[height<=${height}]+bestaudio/best[height<=${height}]/best`
      );
    } else {
      // 기본: 1080p 상한
      args.push(
        "-f",
        "bestvideo[height<=1080]+bestaudio/best[height<=1080]/best"
      );
    }
    args.push(
      "--merge-output-format",
      "mp4",
      "-o",
      outputTemplate,
      "--no-playlist",
      opts.url
    );
    await runYtdlpWithCookieFallback(opts.url, args);
  }

  return { success: true };
});

// ── Window ──

async function createWindow() {
  mainWindow = new BrowserWindow({
    width: 900, height: 700,
    minWidth: 600, minHeight: 500,
    title: "SnapSave",
    icon: path.join(__dirname, "icon.ico"),
    autoHideMenuBar: true,
    backgroundColor: "#0c0c0f",
    show: false,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, "preload.js"),
    },
  });

  mainWindow.loadFile(path.join(__dirname, "index.html"));

  mainWindow.once("ready-to-show", () => {
    mainWindow.show();
    mainWindow.focus();
  });

  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url);
    return { action: "deny" };
  });

  mainWindow.on("closed", () => { mainWindow = null; });
}

function setupAutoUpdater() {
  if (isDev) return;
  autoUpdater.autoDownload = true;
  autoUpdater.autoInstallOnAppQuit = true;

  autoUpdater.on("update-available", (info) => {
    dialog.showMessageBox(mainWindow, {
      type: "info", title: "업데이트 발견",
      message: `새 버전 ${info.version}을 다운로드 중입니다.`,
    });
  });

  autoUpdater.on("update-downloaded", () => {
    dialog.showMessageBox(mainWindow, {
      type: "info", title: "업데이트 준비 완료",
      message: "업데이트가 다운로드되었습니다. 지금 재시작하시겠습니까?",
      buttons: ["지금 재시작", "나중에"],
    }).then((result) => {
      if (result.response === 0) autoUpdater.quitAndInstall();
    });
  });

  autoUpdater.on("error", () => {});
  autoUpdater.checkForUpdates();
}

app.whenReady().then(async () => {
  await createWindow();
  setupAutoUpdater();
});

app.on("window-all-closed", () => { app.quit(); });
