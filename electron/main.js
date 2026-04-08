const { app, BrowserWindow, shell, dialog } = require("electron");
const { spawn } = require("child_process");
const { autoUpdater } = require("electron-updater");
const path = require("path");
const net = require("net");

let mainWindow;
let serverProcess;

const isDev = !app.isPackaged;

function getResourcePath(filename) {
  if (isDev) {
    return path.join(__dirname, "..", "bin", filename);
  }
  return path.join(process.resourcesPath, "bin", filename);
}

function findAvailablePort(startPort) {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.listen(startPort, () => {
      const port = server.address().port;
      server.close(() => resolve(port));
    });
    server.on("error", () => resolve(findAvailablePort(startPort + 1)));
  });
}

async function startNextServer(port) {
  const nextPath = isDev
    ? path.join(__dirname, "..", "node_modules", ".bin", "next.cmd")
    : path.join(process.resourcesPath, "app", "node_modules", ".bin", "next.cmd");

  const appDir = isDev
    ? path.join(__dirname, "..")
    : path.join(process.resourcesPath, "app");

  const ytdlpPath = getResourcePath("yt-dlp.exe");
  const ffmpegDir = path.dirname(getResourcePath("ffmpeg.exe"));

  const env = {
    ...process.env,
    PORT: String(port),
    YTDLP_PATH: ytdlpPath,
    FFMPEG_DIR: ffmpegDir,
    NODE_ENV: "production",
  };

  return new Promise((resolve, reject) => {
    serverProcess = spawn(nextPath, ["start", "-p", String(port)], {
      cwd: appDir,
      env,
      stdio: "pipe",
      shell: true,
    });

    serverProcess.stdout.on("data", (data) => {
      const msg = data.toString();
      if (msg.includes("Ready") || msg.includes("started")) {
        resolve(port);
      }
    });

    serverProcess.stderr.on("data", (data) => {
      console.error("Server:", data.toString());
    });

    serverProcess.on("error", reject);

    setTimeout(() => resolve(port), 5000);
  });
}

async function createWindow(port) {
  mainWindow = new BrowserWindow({
    width: 900,
    height: 700,
    minWidth: 600,
    minHeight: 500,
    title: "SnapSave",
    icon: path.join(__dirname, "icon.ico"),
    autoHideMenuBar: true,
    backgroundColor: "#0c0c0f",
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
    },
  });

  mainWindow.loadURL(`http://localhost:${port}`);

  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url);
    return { action: "deny" };
  });

  mainWindow.on("closed", () => {
    mainWindow = null;
  });
}

// Auto-updater
function setupAutoUpdater() {
  if (isDev) return;

  autoUpdater.autoDownload = true;
  autoUpdater.autoInstallOnAppQuit = true;

  autoUpdater.on("update-available", (info) => {
    dialog.showMessageBox(mainWindow, {
      type: "info",
      title: "업데이트 발견",
      message: `새 버전 ${info.version}을 다운로드 중입니다. 완료되면 알려드릴게요.`,
    });
  });

  autoUpdater.on("update-downloaded", () => {
    dialog
      .showMessageBox(mainWindow, {
        type: "info",
        title: "업데이트 준비 완료",
        message: "업데이트가 다운로드되었습니다. 지금 재시작하시겠습니까?",
        buttons: ["지금 재시작", "나중에"],
      })
      .then((result) => {
        if (result.response === 0) {
          autoUpdater.quitAndInstall();
        }
      });
  });

  autoUpdater.on("error", () => {
    // Silent fail - don't bother user
  });

  autoUpdater.checkForUpdates();
}

app.whenReady().then(async () => {
  const port = await findAvailablePort(3456);
  await startNextServer(port);
  await createWindow(port);
  setupAutoUpdater();
});

app.on("window-all-closed", () => {
  if (serverProcess) {
    serverProcess.kill();
  }
  app.quit();
});

app.on("before-quit", () => {
  if (serverProcess) {
    serverProcess.kill();
  }
});
