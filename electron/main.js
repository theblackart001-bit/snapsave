const { app, BrowserWindow, shell, dialog, ipcMain } = require("electron");
const { execFile } = require("child_process");
const { autoUpdater } = require("electron-updater");
const path = require("path");
const fs = require("fs");
const os = require("os");

app.disableHardwareAcceleration();
app.commandLine.appendSwitch("disable-gpu");

let mainWindow;
const isDev = !app.isPackaged;

function getResourcePath(filename) {
  if (isDev) return path.join(__dirname, "..", "bin", filename);
  return path.join(process.resourcesPath, "bin", filename);
}

function getYtdlpPath() {
  return getResourcePath("yt-dlp.exe");
}

function getFfmpegDir() {
  return path.dirname(getResourcePath("ffmpeg.exe"));
}

function runYtdlp(args) {
  return new Promise((resolve, reject) => {
    const ffmpegDir = getFfmpegDir();
    const ffmpegArgs = ffmpegDir ? ["--ffmpeg-location", ffmpegDir] : [];
    execFile(
      getYtdlpPath(),
      [...ffmpegArgs, ...args],
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
  const raw = await runYtdlp(["-j", "--no-playlist", url]);
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

// IPC: download media
ipcMain.handle("download", async (_event, opts) => {
  const downloadsDir = path.join(os.homedir(), "Downloads");
  const outputTemplate = path.join(downloadsDir, "%(title).80s.%(ext)s");

  if (opts.type === "thumbnail") {
    await runYtdlp([
      "--write-thumbnail",
      "--skip-download",
      "--convert-thumbnails", "jpg",
      "-o", path.join(downloadsDir, "%(title).80s"),
      "--no-playlist",
      opts.url,
    ]);
    return { success: true };
  }

  if (opts.type === "audio") {
    await runYtdlp([
      "-x", "--audio-format", "mp3", "--audio-quality", "0",
      "-o", outputTemplate, "--no-playlist", opts.url,
    ]);
  } else {
    const args = [];
    if (opts.quality) {
      const height = opts.quality.replace("p", "");
      args.push("-f", `bestvideo[height<=${height}]+bestaudio/best[height<=${height}]/best`);
    } else {
      args.push("-f", "bestvideo+bestaudio/best");
    }
    args.push("--merge-output-format", "mp4", "-o", outputTemplate, "--no-playlist", opts.url);
    await runYtdlp(args);
  }

  return { success: true };
});

async function createWindow() {
  mainWindow = new BrowserWindow({
    width: 900,
    height: 700,
    minWidth: 600,
    minHeight: 500,
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
      type: "info",
      title: "업데이트 발견",
      message: `새 버전 ${info.version}을 다운로드 중입니다.`,
    });
  });

  autoUpdater.on("update-downloaded", () => {
    dialog.showMessageBox(mainWindow, {
      type: "info",
      title: "업데이트 준비 완료",
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
