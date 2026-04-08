const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("api", {
  getInfo: (url) => ipcRenderer.invoke("get-info", url),
  download: (opts) => ipcRenderer.invoke("download", opts),
});
