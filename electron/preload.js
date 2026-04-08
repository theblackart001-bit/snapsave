const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("api", {
  checkSetup: () => ipcRenderer.invoke("check-setup"),
  runSetup: () => ipcRenderer.invoke("run-setup"),
  onSetupProgress: (cb) => ipcRenderer.on("setup-progress", (_e, data) => cb(data)),
  getInfo: (url) => ipcRenderer.invoke("get-info", url),
  download: (opts) => ipcRenderer.invoke("download", opts),
});
