// Auto-updater — checks GitHub Releases for newer SnapSave builds and
// performs a safe self-replace via a tiny .bat helper (Windows can't overwrite
// a running .exe directly, so the batch file does the swap after we exit).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Version is the running build version. Override at build time with:
//
//	go build -ldflags "-X main.Version=1.2.3"
var Version = "1.3.1"

// updateRepo is the GitHub "owner/name" the updater queries. To redirect
// updates to a different repo, override at build time:
//
//	go build -ldflags "-X main.updateRepo=other/repo"
var updateRepo = "theblackart001-bit/snapsave-releases"

// updateAssetPattern decides which asset in a release maps to our .exe.
// Match the actual portable build naming. Multiple matches → first wins.
var updateAssetPattern = regexp.MustCompile(`(?i)snapsave.*\.exe$`)

// ReleaseInfo is the subset of the GitHub Releases API response we use.
type ReleaseInfo struct {
	TagName string         `json:"tag_name"`
	Name    string         `json:"name"`
	Body    string         `json:"body"`
	HTMLURL string         `json:"html_url"`
	Assets  []ReleaseAsset `json:"assets"`
}

type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// updateStatus is what /api/check-update returns to the renderer.
type updateStatus struct {
	Current     string `json:"current"`
	Available   bool   `json:"available"`
	Latest      string `json:"latest,omitempty"`
	ReleaseURL  string `json:"releaseUrl,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
	Body        string `json:"body,omitempty"`
	Error       string `json:"error,omitempty"`
}

// updaterState caches the most recent release info so the UI doesn't have to
// hit GitHub every time the user expands the update banner.
var (
	updaterMu       sync.RWMutex
	cachedRelease   *ReleaseInfo
	lastCheckedAt   time.Time
	updateInProg    bool
)

// checkLatestRelease queries GitHub for the latest release. Returns nil and a
// nil error when the repo has no releases yet (404 response) so that the UI
// can show "you're on the latest" rather than an angry error.
func checkLatestRelease() (*ReleaseInfo, error) {
	if updateRepo == "" {
		return nil, fmt.Errorf("update repo not configured")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", updateRepo)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "SnapSave-Updater/"+Version)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Repo or releases don't exist yet — treat as "no update available"
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub HTTP %d", resp.StatusCode)
	}
	var info ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// compareVersions returns -1 if a<b, 0 if equal, 1 if a>b. Best-effort semver:
// splits on '.', compares numeric parts; non-numeric fragments fall back to
// lexical compare. Strips a leading 'v'/'V'.
func compareVersions(a, b string) int {
	a = strings.TrimPrefix(strings.TrimSpace(a), "v")
	a = strings.TrimPrefix(a, "V")
	b = strings.TrimPrefix(strings.TrimSpace(b), "v")
	b = strings.TrimPrefix(b, "V")
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var sa, sb string
		if i < len(pa) {
			sa = pa[i]
		}
		if i < len(pb) {
			sb = pb[i]
		}
		ia, ea := strconv.Atoi(sa)
		ib, eb := strconv.Atoi(sb)
		if ea == nil && eb == nil {
			if ia < ib {
				return -1
			}
			if ia > ib {
				return 1
			}
			continue
		}
		if sa < sb {
			return -1
		}
		if sa > sb {
			return 1
		}
	}
	return 0
}

// pickAsset finds the first release asset whose filename matches our pattern.
func pickAsset(rel *ReleaseInfo) *ReleaseAsset {
	if rel == nil {
		return nil
	}
	for i := range rel.Assets {
		if updateAssetPattern.MatchString(rel.Assets[i].Name) {
			return &rel.Assets[i]
		}
	}
	return nil
}

// computeStatus returns the current update status. Uses the cached release if
// it was checked within updateRecheckInterval, otherwise refreshes from GitHub.
const updateRecheckInterval = 30 * time.Minute

func computeStatus(forceRefresh bool) updateStatus {
	st := updateStatus{Current: Version}

	updaterMu.RLock()
	rel := cachedRelease
	last := lastCheckedAt
	updaterMu.RUnlock()

	if forceRefresh || rel == nil || time.Since(last) > updateRecheckInterval {
		fresh, err := checkLatestRelease()
		if err != nil {
			st.Error = err.Error()
			return st
		}
		updaterMu.Lock()
		cachedRelease = fresh
		lastCheckedAt = time.Now()
		updaterMu.Unlock()
		rel = fresh
	}

	if rel == nil {
		return st // no releases yet → not available
	}
	if compareVersions(rel.TagName, Version) <= 0 {
		return st
	}
	asset := pickAsset(rel)
	st.Available = true
	st.Latest = rel.TagName
	st.ReleaseURL = rel.HTMLURL
	st.Body = rel.Body
	if asset != nil {
		st.DownloadURL = asset.DownloadURL
	}
	return st
}

// downloadFileWithProgress downloads url to dst, calling onProgress with
// percent (0..100) at most ~10 times per second.
func downloadFileWithProgress(url, dst string, onProgress func(pct float64)) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 64*1024)
	lastPct := -1.0
	lastEmit := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if onProgress != nil && total > 0 && time.Since(lastEmit) > 100*time.Millisecond {
				pct := float64(written) / float64(total) * 100
				if pct-lastPct > 0.1 {
					onProgress(pct)
					lastPct = pct
					lastEmit = time.Now()
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if onProgress != nil {
		onProgress(100)
	}
	return nil
}

// writeUpdateBatch writes a self-deleting .bat that:
//  1. Waits for the running .exe to release the lock
//  2. Replaces the .exe with the freshly downloaded .new file
//  3. Re-launches it
//  4. Deletes itself
//
// We escape every path argument with %% so spaces (which the user's path has)
// don't break parsing.
func writeUpdateBatch(currentExe, newExe string) (string, error) {
	batPath := filepath.Join(filepath.Dir(currentExe), "snapsave-update.bat")
	script := `@echo off
setlocal
set "TARGET=` + currentExe + `"
set "SOURCE=` + newExe + `"

rem Wait until the old SnapSave process releases the file (max ~10s).
set /a TRIES=0
:waitloop
del /f /q "%TARGET%" >nul 2>&1
if not exist "%TARGET%" goto replace
set /a TRIES+=1
if %TRIES% GEQ 20 goto fail
ping -n 2 127.0.0.1 >nul
goto waitloop

:replace
move /y "%SOURCE%" "%TARGET%" >nul
if errorlevel 1 goto fail
start "" "%TARGET%"
del /f /q "%~f0"
exit /b 0

:fail
echo Update failed. The old SnapSave is locked.
pause
exit /b 1
`
	if err := os.WriteFile(batPath, []byte(script), 0755); err != nil {
		return "", err
	}
	return batPath, nil
}

// applyUpdate downloads the asset and arranges a self-replace + restart.
// Returns nil on a successful start of the helper batch (we exit shortly
// after); any error from this point is fatal for the update attempt.
func applyUpdate(downloadURL string, onProgress func(pct float64)) error {
	updaterMu.Lock()
	if updateInProg {
		updaterMu.Unlock()
		return fmt.Errorf("update already in progress")
	}
	updateInProg = true
	updaterMu.Unlock()
	defer func() {
		updaterMu.Lock()
		updateInProg = false
		updaterMu.Unlock()
	}()

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	newPath := exePath + ".new"

	if err := downloadFileWithProgress(downloadURL, newPath, onProgress); err != nil {
		_ = os.Remove(newPath)
		return err
	}

	batPath, err := writeUpdateBatch(exePath, newPath)
	if err != nil {
		_ = os.Remove(newPath)
		return err
	}

	// Detach the batch so it survives our exit.
	cmd := exec.Command("cmd", "/c", "start", "", "/min", batPath)
	cmd.SysProcAttr = hiddenWindowAttr()
	if err := cmd.Start(); err != nil {
		_ = os.Remove(newPath)
		_ = os.Remove(batPath)
		return err
	}

	// Give the helper a moment to spin up before we exit.
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

// startBackgroundUpdateCheck does a best-effort check on startup so the UI
// can surface a banner if a new release exists. Errors are silent — this is
// non-essential.
func startBackgroundUpdateCheck() {
	go func() {
		// Tiny delay so we don't compete with the WebView for network/CPU
		// during initial load.
		time.Sleep(3 * time.Second)
		_ = computeStatus(true)
	}()
}

// hasStartupUpdate runs a fast update check with a hard timeout so the
// startup path never hangs longer than `timeout`. Returns true only when a
// newer release has a downloadable asset attached. Always returns false on
// any error path so a flaky network just means "skip update".
func hasStartupUpdate(timeout time.Duration) bool {
	type result struct{ avail bool }
	ch := make(chan result, 1)
	go func() {
		st := computeStatus(true)
		ch <- result{avail: st.Available && st.DownloadURL != ""}
	}()
	select {
	case r := <-ch:
		return r.avail
	case <-time.After(timeout):
		return false
	}
}

// updateScreenHTML is the splash page shown when an update is being applied
// at startup. It immediately calls /api/update/apply and renders progress;
// the .bat helper takes over once download finishes and exits this process.
const updateScreenHTML = `<!DOCTYPE html>
<html lang="ko"><head><meta charset="UTF-8"><title>SnapSave 업데이트</title>
<style>
:root { --bg:#0c0c0f; --fg:#fff; --sub:#a0a0b0; --accent:#6c5ce7; --border:#2a2a35; }
* { box-sizing:border-box; margin:0; padding:0; }
body { background:var(--bg); color:var(--fg); font-family:'Segoe UI',system-ui,sans-serif;
       height:100vh; display:flex; flex-direction:column; align-items:center; justify-content:center;
       gap:24px; padding:32px; user-select:none; -webkit-font-smoothing:antialiased; }
h1 { font-size:32px; font-weight:700; letter-spacing:-1px;
     background:linear-gradient(135deg,#6c5ce7,#a29bfe,#74b9ff);
     -webkit-background-clip:text; -webkit-text-fill-color:transparent; }
.label { color:var(--sub); font-size:14px; }
.bar-wrap { width:360px; height:6px; background:#1e1e24; border-radius:3px; overflow:hidden; }
.bar { width:0%; height:100%; background:var(--accent); border-radius:3px;
       transition:width 0.25s ease-out; box-shadow:0 0 12px rgba(108,92,231,0.6); }
.pct { color:var(--fg); font-size:13px; font-variant-numeric:tabular-nums; }
.note { color:var(--sub); font-size:12px; max-width:360px; text-align:center; line-height:1.5; }
.err { color:#ff6b6b; font-size:13px; max-width:360px; text-align:center; }
</style></head>
<body>
  <h1>SnapSave</h1>
  <p id="msg" class="label">새 버전을 다운로드하고 있습니다...</p>
  <div class="bar-wrap"><div id="bar" class="bar"></div></div>
  <p id="pct" class="pct">0%</p>
  <p class="note">완료 후 자동으로 재시작됩니다.<br>창을 닫지 마세요.</p>
  <p id="err" class="err"></p>
<script>
(async () => {
  const bar = document.getElementById('bar');
  const pct = document.getElementById('pct');
  const msg = document.getElementById('msg');
  const err = document.getElementById('err');
  function set(p, label) {
    bar.style.width = p + '%';
    pct.textContent = Math.round(p) + '%';
    if (label) msg.textContent = label;
  }
  try {
    const res = await fetch('/api/update/apply', { method: 'POST' });
    if (!res.ok || !res.body) throw new Error('업데이트 시작 실패');
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const lines = buf.split('\n');
      buf = lines.pop() || '';
      for (const line of lines) {
        if (!line.trim()) continue;
        let evt; try { evt = JSON.parse(line); } catch (_) { continue; }
        if (evt.type === 'progress') set(evt.percent || 0, '새 버전 다운로드 중...');
        else if (evt.type === 'done') set(100, '적용 중... 곧 재시작됩니다');
        else if (evt.type === 'error') throw new Error(evt.message || '업데이트 실패');
      }
    }
  } catch (e) {
    err.textContent = e.message + ' — 5초 후 기존 버전으로 실행됩니다';
    setTimeout(() => { window.location.href = '/'; }, 5000);
  }
})();
</script></body></html>`

// HTTP handlers

func handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "1"
	st := computeStatus(force)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(st)
}

func handleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	emit := func(m map[string]interface{}) {
		_ = enc.Encode(m)
		if flusher != nil {
			flusher.Flush()
		}
	}

	st := computeStatus(false)
	if !st.Available || st.DownloadURL == "" {
		emit(map[string]interface{}{"type": "error", "message": "업데이트가 없거나 다운로드 자산을 찾을 수 없습니다"})
		return
	}

	emit(map[string]interface{}{"type": "progress", "percent": 0, "stage": "downloading"})
	err := applyUpdate(st.DownloadURL, func(pct float64) {
		emit(map[string]interface{}{"type": "progress", "percent": pct, "stage": "downloading"})
	})
	if err != nil {
		emit(map[string]interface{}{"type": "error", "message": err.Error()})
		return
	}
	emit(map[string]interface{}{"type": "done"})
}
