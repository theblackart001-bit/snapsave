package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jchv/go-webview2"
)

var (
	binDir      string
	ytdlpExe    string
	ffmpegExe   string
	historyFile string
	thumbDir    string
)

func init() {
	exePath, _ := os.Executable()
	binDir = filepath.Join(filepath.Dir(exePath), "bin")
	ytdlpExe = filepath.Join(binDir, "yt-dlp.exe")
	ffmpegExe = filepath.Join(binDir, "ffmpeg.exe")
	historyFile = filepath.Join(filepath.Dir(exePath), "history.json")
	thumbDir = filepath.Join(filepath.Dir(exePath), "thumb-cache")
	os.MkdirAll(thumbDir, 0755)
	go cleanupOldThumbs()
}

// cleanupOldThumbs removes cached thumbnails older than 24h on startup so the
// folder doesn't grow unbounded over time.
func cleanupOldThumbs() {
	entries, err := os.ReadDir(thumbDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(thumbDir, e.Name()))
		}
	}
}

// safeID strips path separators so IDs from yt-dlp can't escape thumbDir.
var safeIDRe = regexp.MustCompile(`[^A-Za-z0-9_\-]`)

func safeID(id string) string {
	return safeIDRe.ReplaceAllString(id, "_")
}

// findCachedThumb returns the path of a cached thumbnail file for this ID, if any.
// yt-dlp may write .jpg, .webp, .png depending on source; check common extensions.
func findCachedThumb(id string) string {
	id = safeID(id)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		p := filepath.Join(thumbDir, id+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// writeThumbnailViaYtdlp asks yt-dlp to fetch the thumbnail using the same
// authenticated session that just succeeded for metadata, and write it to the
// local cache. Runs best-effort — failure is non-fatal and we fall back to the
// CDN proxy. Tries the same browser cookies in the same order as runYtdlp.
func writeThumbnailViaYtdlp(rawURL, id string) {
	if id == "" {
		return
	}
	id = safeID(id)
	// If we already have it cached, skip.
	if findCachedThumb(id) != "" {
		return
	}

	out := filepath.Join(thumbDir, id) // yt-dlp adds the right extension
	args := []string{
		"--write-thumbnail",
		"--skip-download",
		"--convert-thumbnails", "jpg",
		"-o", out,
		"--no-playlist",
		rawURL,
	}

	var order []string
	if needsCookies(rawURL) {
		order = append(order, cookieBrowsers...)
		order = append(order, "")
	} else {
		order = append(order, "")
		order = append(order, cookieBrowsers...)
	}
	for _, browser := range order {
		_, err := runYtdlpRaw(browser, args...)
		if err == nil {
			return
		}
		// Don't loop forever on errors that won't resolve via different cookies.
		if !shouldRetryWithDifferentCookies(err.Error()) {
			return
		}
	}
}

// ── Download history ──

type HistoryEntry struct {
	Title     string `json:"title"`
	Platform  string `json:"platform"`
	Type      string `json:"type"`
	Thumbnail string `json:"thumbnail"`
	URL       string `json:"url"`
	Date      string `json:"date"`
}

func loadHistory() []HistoryEntry {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return []HistoryEntry{}
	}
	var entries []HistoryEntry
	json.Unmarshal(data, &entries)
	return entries
}

func saveHistoryEntry(entry HistoryEntry) {
	entries := loadHistory()
	entries = append([]HistoryEntry{entry}, entries...)
	if len(entries) > 100 {
		entries = entries[:100]
	}
	data, _ := json.Marshal(entries)
	os.WriteFile(historyFile, data, 0644)
}

// ── Download helpers ──

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func extractFFmpegFromZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if filepath.Base(f.Name) == "ffmpeg.exe" && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, rc)
			return err
		}
	}
	return fmt.Errorf("ffmpeg.exe not found in zip")
}

func ensureTools() error {
	os.MkdirAll(binDir, 0755)

	if _, err := os.Stat(ytdlpExe); os.IsNotExist(err) {
		if err := downloadFile("https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe", ytdlpExe); err != nil {
			return fmt.Errorf("yt-dlp 다운로드 실패: %w", err)
		}
	}

	if _, err := os.Stat(ffmpegExe); os.IsNotExist(err) {
		zipPath := filepath.Join(binDir, "ffmpeg.zip")
		if err := downloadFile("https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip", zipPath); err != nil {
			return fmt.Errorf("ffmpeg 다운로드 실패: %w", err)
		}
		if err := extractFFmpegFromZip(zipPath, ffmpegExe); err != nil {
			os.Remove(zipPath)
			return fmt.Errorf("ffmpeg 압축해제 실패: %w", err)
		}
		os.Remove(zipPath)
	}

	return nil
}

// ── yt-dlp runner ──

// runYtdlpRaw runs yt-dlp once with the given args (plus optional cookie browser).
func runYtdlpRaw(cookieBrowser string, args ...string) (string, error) {
	all := []string{"--ffmpeg-location", binDir, "--no-warnings"}
	if cookieBrowser != "" {
		all = append(all, "--cookies-from-browser", cookieBrowser)
	}
	all = append(all, args...)
	cmd := exec.Command(ytdlpExe, all...)
	cmd.SysProcAttr = hiddenWindowAttr()

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errText := stderr.String()
		var lines []string
		for _, l := range strings.Split(errText, "\n") {
			t := strings.TrimSpace(l)
			if t == "" || strings.HasPrefix(t, "WARNING:") || strings.HasPrefix(t, "[download]") || strings.HasPrefix(t, "[info]") {
				continue
			}
			lines = append(lines, t)
		}
		msg := strings.Join(lines, "\n")
		if msg == "" {
			msg = "다운로드 중 오류가 발생했습니다"
		}
		return "", fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

// progressLineRe matches yt-dlp progress lines like "[download]  12.3% of ..."
// when invoked with --newline --progress.
var progressLineRe = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)

// progressEvent is the schema the streaming download endpoint emits as NDJSON.
type progressEvent struct {
	Type      string  `json:"type"`              // progress | log | error | done
	Percent   float64 `json:"percent,omitempty"` // 0..100
	Stage     string  `json:"stage,omitempty"`   // downloading | processing | completed
	StageText string  `json:"stageText,omitempty"`
	Message   string  `json:"message,omitempty"`
}

// runYtdlpStream runs yt-dlp once and forwards live progress to onEvent. yt-dlp
// is invoked with --newline so each progress update arrives on its own line.
// Returns nil on exit code 0; otherwise an error built from filtered stderr.
func runYtdlpStream(cookieBrowser string, args []string, onEvent func(progressEvent)) error {
	all := []string{"--ffmpeg-location", binDir, "--no-warnings", "--newline", "--progress"}
	if cookieBrowser != "" {
		all = append(all, "--cookies-from-browser", cookieBrowser)
	}
	all = append(all, args...)

	cmd := exec.Command(ytdlpExe, all...)
	cmd.SysProcAttr = hiddenWindowAttr()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	stage := "downloading"
	var stderrBuf strings.Builder

	// Drain stderr in a goroutine so the buffer never fills up; capture for
	// the final error message but also surface processing-stage transitions.
	stderrDone := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			stderrBuf.WriteString(line + "\n")
		}
		close(stderrDone)
	}()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()

		// Progress update — map raw 0..100% to stage-aware 0..50% (download)
		// then 50..100% (processing) so the bar never goes backward.
		if m := progressLineRe.FindStringSubmatch(line); m != nil {
			pct, _ := strconv.ParseFloat(m[1], 64)
			displayPct := pct
			if stage == "downloading" {
				displayPct = pct * 0.5
			}
			onEvent(progressEvent{Type: "progress", Percent: displayPct, Stage: stage, StageText: stageLabel(stage)})
			continue
		}

		// Stage transitions — Merger/ffmpeg/ExtractAudio means we crossed into
		// post-processing.
		if stage == "downloading" &&
			(strings.Contains(line, "[Merger]") ||
				strings.Contains(line, "[ffmpeg]") ||
				strings.Contains(line, "[ExtractAudio]") ||
				strings.Contains(line, "[VideoConvertor]")) {
			stage = "processing"
			onEvent(progressEvent{Type: "progress", Percent: 50, Stage: stage, StageText: stageLabel(stage)})
		}
	}

	<-stderrDone
	err = cmd.Wait()
	if err != nil {
		errText := stderrBuf.String()
		var lines []string
		for _, l := range strings.Split(errText, "\n") {
			t := strings.TrimSpace(l)
			if t == "" || strings.HasPrefix(t, "WARNING:") || strings.HasPrefix(t, "[download]") || strings.HasPrefix(t, "[info]") {
				continue
			}
			lines = append(lines, t)
		}
		msg := strings.Join(lines, "\n")
		if msg == "" {
			msg = "다운로드 중 오류가 발생했습니다"
		}
		return fmt.Errorf("%s", msg)
	}
	onEvent(progressEvent{Type: "progress", Percent: 100, Stage: "completed", StageText: stageLabel("completed")})
	return nil
}

func stageLabel(stage string) string {
	switch stage {
	case "downloading":
		return "다운로드 중..."
	case "processing":
		return "병합·인코딩 중..."
	case "completed":
		return "완료"
	}
	return "처리 중..."
}

// runYtdlpStreamWithCookieFallback applies the same cookie strategy as
// runYtdlpWithCookieFallback but for streaming downloads. Earlier attempts
// that fail before any progress is emitted are retried silently; once
// progress starts flowing, errors propagate.
func runYtdlpStreamWithCookieFallback(rawURL string, args []string, onEvent func(progressEvent)) error {
	var order []string
	if needsCookies(rawURL) {
		order = append(order, cookieBrowsers...)
		order = append(order, "")
	} else {
		order = append(order, "")
		order = append(order, cookieBrowsers...)
	}

	var lastErr error
	for _, browser := range order {
		var anyProgress bool
		err := runYtdlpStream(browser, args, func(e progressEvent) {
			if e.Type == "progress" && e.Percent > 0 {
				anyProgress = true
			}
			onEvent(e)
		})
		if err == nil {
			return nil
		}
		lastErr = err
		// If progress already started flowing, don't silently retry — the user
		// would see the bar reset to 0% which is confusing.
		if anyProgress {
			return friendlyYtdlpError(err)
		}
		if !shouldRetryWithDifferentCookies(err.Error()) {
			return friendlyYtdlpError(err)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("다운로드 중 오류가 발생했습니다")
	}
	return friendlyYtdlpError(lastErr)
}

// Platforms whose private/age-gated/region-locked content typically requires login cookies.
func needsCookies(rawURL string) bool {
	u := strings.ToLower(rawURL)
	return strings.Contains(u, "instagram.com") ||
		strings.Contains(u, "threads.net") ||
		strings.Contains(u, "threads.com") ||
		strings.Contains(u, "facebook.com") ||
		strings.Contains(u, "fb.watch")
}

// Browsers tried for cookie extraction, in order of preference on Windows.
var cookieBrowsers = []string{"chrome", "edge", "brave", "firefox"}

// shouldRetryWithDifferentCookies returns true when the error suggests the failure
// could be solved by trying a different cookie source (or no cookies).
func shouldRetryWithDifferentCookies(errMsg string) bool {
	m := strings.ToLower(errMsg)
	return strings.Contains(m, "could not copy") ||
		strings.Contains(m, "could not find") ||
		strings.Contains(m, "unable to read") ||
		strings.Contains(m, "permission denied") ||
		strings.Contains(m, "no such file") ||
		strings.Contains(m, "login required") ||
		strings.Contains(m, "rate-limit") ||
		strings.Contains(m, "not available to everyone") ||
		strings.Contains(m, "can't be seen") ||
		strings.Contains(m, "private") ||
		strings.Contains(m, "requires authentication")
}

// runYtdlp runs yt-dlp with platform-aware cookie fallback. The first arg is
// the URL (used to decide cookie strategy). Args remain in the original order.
func runYtdlp(args ...string) (string, error) {
	// Find the URL among the args (last positional, by yt-dlp convention).
	var url string
	for i := len(args) - 1; i >= 0; i-- {
		a := args[i]
		if strings.HasPrefix(a, "http://") || strings.HasPrefix(a, "https://") {
			url = a
			break
		}
	}

	// Strategy: for cookie-needy platforms try Chrome first, then fall back through
	// other browsers, then no-cookies. For everything else go cookie-free first.
	var order []string
	if needsCookies(url) {
		order = append(order, cookieBrowsers...)
		order = append(order, "")
	} else {
		order = append(order, "")
		order = append(order, cookieBrowsers...)
	}

	var lastErr error
	for _, browser := range order {
		out, err := runYtdlpRaw(browser, args...)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !shouldRetryWithDifferentCookies(err.Error()) {
			return "", friendlyYtdlpError(err)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("다운로드 중 오류가 발생했습니다")
	}
	return "", friendlyYtdlpError(lastErr)
}

// friendlyYtdlpError replaces raw yt-dlp stderr with a Korean explanation when
// the failure is caused by an audience/login/private/rate-limit restriction
// the tool cannot work around on its own. Other errors pass through unchanged.
func friendlyYtdlpError(err error) error {
	if err == nil {
		return nil
	}
	m := strings.ToLower(err.Error())
	// Cookie DB was locked AND every browser fallback failed — this is the most
	// common silent-failure root cause when the user has Chrome open. Tell them.
	if strings.Contains(m, "could not copy") ||
		(strings.Contains(m, "could not find") && strings.Contains(m, "cookies")) {
		return fmt.Errorf(
			"브라우저 쿠키를 읽지 못했습니다. Chrome·Edge 등 모든 창을 닫은 뒤 SnapSave를 다시 실행해 주세요. " +
				"(Instagram·Facebook 콘텐츠는 로그인 쿠키가 필요한데, 브라우저가 켜져 있으면 쿠키 DB가 잠겨 접근할 수 없습니다.)",
		)
	}
	switch {
	case strings.Contains(m, "not available to everyone"),
		strings.Contains(m, "can't be seen"),
		strings.Contains(m, "cannot be seen"):
		return fmt.Errorf(
			"이 게시물은 작성자가 일부 대상에게만 공개했습니다 (예: 친한 친구 전용·지역 제한·연령 제한). " +
				"해당 계정을 팔로우 중이거나 대상에 포함된 계정으로 Chrome에 로그인한 뒤 Chrome 창을 모두 닫고 다시 시도하세요.",
		)
	case strings.Contains(m, "login required"),
		strings.Contains(m, "requires authentication"):
		return fmt.Errorf(
			"로그인이 필요한 콘텐츠입니다. Chrome에 인스타그램·페이스북 계정으로 로그인한 뒤 Chrome 창을 모두 닫고 다시 시도하세요.",
		)
	case strings.Contains(m, "rate-limit"), strings.Contains(m, "rate limit"):
		return fmt.Errorf("플랫폼에서 일시적으로 요청을 제한했습니다. 잠시 후 다시 시도하세요.")
	case strings.Contains(m, "private"):
		return fmt.Errorf("비공개 게시물입니다. 접근 권한이 있는 계정으로 로그인된 브라우저가 필요합니다.")
	case strings.Contains(m, "video unavailable"),
		strings.Contains(m, "this video is unavailable"):
		return fmt.Errorf("영상을 사용할 수 없습니다. 삭제되었거나 지역 차단된 콘텐츠일 수 있습니다.")
	}
	return err
}

func detectPlatform(url string) string {
	u := strings.ToLower(url)
	switch {
	case strings.Contains(u, "youtube.com") || strings.Contains(u, "youtu.be"):
		return "YouTube"
	case strings.Contains(u, "instagram.com"):
		return "Instagram"
	case strings.Contains(u, "tiktok.com"):
		return "TikTok"
	case strings.Contains(u, "threads.net") || strings.Contains(u, "threads.com"):
		return "Threads"
	case strings.Contains(u, "facebook.com") || strings.Contains(u, "fb.watch"):
		return "Facebook"
	case strings.Contains(u, "xiaohongshu.com") || strings.Contains(u, "xhslink.com"):
		return "Xiaohongshu"
	case strings.Contains(u, "douyin.com"):
		return "Douyin"
	}
	return "Unknown"
}

// normalizeURL cleans up platform-specific URL quirks before passing to yt-dlp.
func normalizeURL(rawURL string) string {
	// Threads: threads.com → threads.net, remove /video-... suffix
	if strings.Contains(rawURL, "threads.com") || strings.Contains(rawURL, "threads.net") {
		rawURL = strings.Replace(rawURL, "threads.com", "threads.net", 1)
		// /post/ID/video-... → /post/ID
		if idx := strings.Index(rawURL, "/video-"); idx != -1 {
			rawURL = rawURL[:idx]
		}
	}
	return rawURL
}

// instagramShortcodeRe matches the shortcode in /reel/<id>/, /reels/<id>/,
// /p/<id>/, and /tv/<id>/ paths.
var instagramShortcodeRe = regexp.MustCompile(`/(?:reel|reels|p|tv)/([A-Za-z0-9_-]+)`)

func instagramShortcode(rawURL string) string {
	m := instagramShortcodeRe.FindStringSubmatch(rawURL)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// instagramEmbedThumbnail fetches the public embed page for an Instagram post
// with an iPhone User-Agent (which returns the rich embed HTML containing the
// EmbeddedMediaImage <img>) and extracts the largest srcset URL. This works
// without any login or cookies — it's the same approach sssinstagram uses for
// public posts.
//
// Returns "" silently on any failure so the caller can fall back gracefully.
var embedImgRe = regexp.MustCompile(`(?s)<img class="EmbeddedMediaImage"[^>]*?\bsrc="([^"]+)"`)

func instagramEmbedThumbnail(shortcode string) string {
	if shortcode == "" {
		return ""
	}
	embedURL := fmt.Sprintf("https://www.instagram.com/p/%s/embed/captioned/", shortcode)
	client := &http.Client{Timeout: 12 * time.Second}
	req, _ := http.NewRequest("GET", embedURL, nil)
	// iPhone UA reliably returns the populated embed HTML with the media image.
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	m := embedImgRe.FindSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return strings.ReplaceAll(string(m[1]), "&amp;", "&")
}

// downloadEmbedThumbnail saves the embed-page thumbnail to local cache so the
// renderer fetches it without exposing the session-signed CDN URL. Best-effort.
func downloadEmbedThumbnail(thumbURL, id string) bool {
	if thumbURL == "" || id == "" {
		return false
	}
	id = safeID(id)
	parsed, err := url.Parse(thumbURL)
	if err != nil {
		return false
	}
	body, _, err := fetchThumbnail(thumbURL, parsed)
	if err != nil {
		return false
	}
	dst := filepath.Join(thumbDir, id+".jpg")
	if err := os.WriteFile(dst, body, 0644); err != nil {
		return false
	}
	return true
}

// threadsShortcode extracts the shortcode from a Threads URL.
var threadsPostRe = regexp.MustCompile(`/post/([A-Za-z0-9_-]+)`)

func threadsShortcode(rawURL string) string {
	m := threadsPostRe.FindStringSubmatch(rawURL)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// threadsVideoURL fetches the mp4 URL via Threads embed page.
func threadsVideoURL(shortcode string) (videoURL string, title string, thumbnail string, err error) {
	embedURL := fmt.Sprintf("https://www.threads.net/t/%s/embed/", shortcode)
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", embedURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("threads embed request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Extract mp4 URL (HTML contains &amp; instead of &)
	mp4Re := regexp.MustCompile(`https://scontent[^"]*\.mp4[^"]*`)
	mp4Match := mp4Re.FindString(html)
	if mp4Match == "" {
		return "", "", "", fmt.Errorf("no video found in Threads post")
	}
	// Unescape HTML entities
	mp4Match = strings.ReplaceAll(mp4Match, "&amp;", "&")

	// Extract title
	title = "threads_video"
	titleRe := regexp.MustCompile(`og:title" content="([^"]*)"`)
	if tm := titleRe.FindStringSubmatch(html); len(tm) >= 2 {
		title = tm[1]
	}

	// Extract thumbnail from embed page
	thumbnail = ""
	posterRe := regexp.MustCompile(`poster="([^"]*)"`)
	if pm := posterRe.FindStringSubmatch(html); len(pm) >= 2 {
		thumbnail = strings.ReplaceAll(pm[1], "&amp;", "&")
	}
	// Fallback: fetch from main post page (has og:image)
	if thumbnail == "" {
		mainURL := fmt.Sprintf("https://www.threads.net/t/%s/", shortcode)
		mainReq, _ := http.NewRequest("GET", mainURL, nil)
		mainReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		if mainResp, mErr := client.Do(mainReq); mErr == nil {
			mainBody, _ := io.ReadAll(mainResp.Body)
			mainResp.Body.Close()
			ogRe := regexp.MustCompile(`og:image" content="([^"]*)"`)
			if om := ogRe.FindStringSubmatch(string(mainBody)); len(om) >= 2 {
				thumbnail = strings.ReplaceAll(om[1], "&amp;", "&")
			}
		}
	}

	return mp4Match, title, thumbnail, nil
}

// threadsDownload downloads a Threads video directly.
func threadsDownload(videoURL, title, dlDir string) (string, error) {
	// Sanitize filename
	safe := regexp.MustCompile(`[<>:"/\\|?*]`).ReplaceAllString(title, "_")
	if len(safe) > 80 {
		safe = safe[:80]
	}
	outPath := filepath.Join(dlDir, safe+".mp4")

	client := &http.Client{Timeout: 120 * time.Second}
	parsed, _ := url.Parse(videoURL)
	req, _ := http.NewRequest("GET", parsed.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://www.threads.net")
	req.Header.Set("Referer", "https://www.threads.net/")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("file create failed: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return "", fmt.Errorf("file write failed: %w", err)
	}
	if written < 1000 {
		os.Remove(outPath)
		return "", fmt.Errorf("download failed: file too small (%d bytes), CDN rejected request", written)
	}
	return outPath, nil
}

// ── API handlers ──

type VideoInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
	Duration  int    `json:"duration"`
	Uploader  string `json:"uploader"`
	Platform  string `json:"platform"`
}

func handleGetInfo(w http.ResponseWriter, r *http.Request) {
	var req struct{ URL string `json:"url"` }
	json.NewDecoder(r.Body).Decode(&req)

	reqURL := normalizeURL(req.URL)
	platform := detectPlatform(reqURL)

	// Threads 전용 처리 (yt-dlp 미지원)
	if platform == "Threads" {
		sc := threadsShortcode(reqURL)
		if sc == "" {
			jsonError(w, "Invalid Threads URL", 400)
			return
		}
		_, title, thumb, err := threadsVideoURL(sc)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		info := VideoInfo{
			ID:        sc,
			Title:     title,
			Thumbnail: thumb,
			Duration:  0,
			Uploader:  "Threads",
			Platform:  "Threads",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
		return
	}

	raw, err := runYtdlp("-j", "--no-playlist", reqURL)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	var data map[string]interface{}
	json.Unmarshal([]byte(raw), &data)

	id := getString(data, "id")
	thumb := getString(data, "thumbnail")

	// For platforms whose CDN signs thumbnail URLs to the originating session
	// (Instagram, FB, TikTok), the URL we got is unusable for any other client.
	// Try in order:
	//   1. Instagram public embed page (no cookies needed) — works for any
	//      public post even when yt-dlp succeeded only via session cookies.
	//   2. yt-dlp --write-thumbnail (uses same authenticated session that
	//      succeeded for metadata).
	// First success wins; the renderer just hits /api/local-thumb.
	if id != "" && thumb != "" && needsLocalThumbCache(thumb) {
		cached := false
		if platform == "Instagram" {
			if shortcode := instagramShortcode(reqURL); shortcode != "" {
				if embedThumb := instagramEmbedThumbnail(shortcode); embedThumb != "" {
					cached = downloadEmbedThumbnail(embedThumb, id)
				}
			}
		}
		if !cached {
			writeThumbnailViaYtdlp(reqURL, id)
		}
		if findCachedThumb(id) != "" {
			thumb = "/api/local-thumb?id=" + safeID(id)
		}
	}

	info := VideoInfo{
		ID:        id,
		Title:     getStringOr(data, "title", "Untitled"),
		Thumbnail: thumb,
		Duration:  getInt(data, "duration"),
		Uploader:  getStringOr(data, "uploader", getStringOr(data, "channel", "Unknown")),
		Platform:  platform,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// needsLocalThumbCache returns true when the thumbnail URL points at a CDN that
// signs URLs to the requesting session — the only reliable way to render those
// is to refetch via yt-dlp's authenticated session and serve from local cache.
func needsLocalThumbCache(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return proxyHostAllowed(parsed.Host)
}

func handleLocalThumb(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writePngPlaceholder(w)
		return
	}
	p := findCachedThumb(safeID(id))
	if p == "" {
		writePngPlaceholder(w)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		writePngPlaceholder(w)
		return
	}
	defer f.Close()
	ext := strings.ToLower(filepath.Ext(p))
	ct := "image/jpeg"
	switch ext {
	case ".png":
		ct = "image/png"
	case ".webp":
		ct = "image/webp"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	io.Copy(w, f)
}

// buildVideoArgs builds yt-dlp args for a video download. Mirrors Youtuboost's
// approach: re-encode to MP4 with libx264+AAC and faststart so the file plays
// reliably everywhere and starts streaming before the full download is buffered.
func buildVideoArgs(quality, outTpl, url string) []string {
	args := []string{}
	if quality != "" && quality != "best" {
		h := strings.Replace(quality, "p", "", 1)
		args = append(args, "-f", fmt.Sprintf("bestvideo[height<=%s]+bestaudio/best[height<=%s]/best", h, h))
	} else {
		args = append(args, "-f", "bestvideo*+bestaudio/best")
	}
	args = append(args,
		"--recode-video", "mp4",
		"--postprocessor-args", "VideoConvertor+ffmpeg:-movflags +faststart -preset fast -crf 22 -c:v libx264 -c:a aac",
		"-o", outTpl,
		"--no-playlist",
		url,
	)
	return args
}

// handleDownloadStream is the streaming variant of /api/download. It writes
// NDJSON progress events as yt-dlp runs, so the renderer can show a live
// progress bar instead of a spinner.
func handleDownloadStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL       string `json:"url"`
		Type      string `json:"type"`
		Quality   string `json:"quality"`
		Title     string `json:"title"`
		Platform  string `json:"platform"`
		Thumbnail string `json:"thumbnail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", 400)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	emit := func(e progressEvent) {
		_ = enc.Encode(e)
		if flusher != nil {
			flusher.Flush()
		}
	}

	dlURL := normalizeURL(req.URL)
	platform := detectPlatform(dlURL)
	home, _ := os.UserHomeDir()
	dlDir := filepath.Join(home, "Downloads")

	// Threads has no yt-dlp support — fall back to the non-streaming direct
	// download path, but still emit start/done events so the UI bar moves.
	if platform == "Threads" {
		emit(progressEvent{Type: "progress", Percent: 5, Stage: "downloading", StageText: stageLabel("downloading")})
		sc := threadsShortcode(dlURL)
		if sc == "" {
			emit(progressEvent{Type: "error", Message: "Invalid Threads URL"})
			return
		}
		videoURL, title, _, err := threadsVideoURL(sc)
		if err != nil {
			emit(progressEvent{Type: "error", Message: err.Error()})
			return
		}
		_, err = threadsDownload(videoURL, title, dlDir)
		if err != nil {
			emit(progressEvent{Type: "error", Message: err.Error()})
			return
		}
		saveHistoryEntry(HistoryEntry{
			Title: title, Platform: "Threads", Type: "영상",
			URL: req.URL, Date: time.Now().Format("2006-01-02 15:04"),
		})
		emit(progressEvent{Type: "progress", Percent: 100, Stage: "completed", StageText: stageLabel("completed")})
		emit(progressEvent{Type: "done"})
		return
	}

	outTpl := filepath.Join(dlDir, "%(title).80s.%(ext)s")
	var args []string
	switch req.Type {
	case "thumbnail":
		args = []string{"--write-thumbnail", "--skip-download", "--convert-thumbnails", "jpg",
			"-o", filepath.Join(dlDir, "%(title).80s"), "--no-playlist", dlURL}
	case "audio":
		args = []string{"-x", "--audio-format", "mp3", "--audio-quality", "0",
			"-o", outTpl, "--no-playlist", dlURL}
	default:
		args = buildVideoArgs(req.Quality, outTpl, dlURL)
	}

	if err := runYtdlpStreamWithCookieFallback(dlURL, args, emit); err != nil {
		emit(progressEvent{Type: "error", Message: err.Error()})
		return
	}

	typeLabel := "영상"
	if req.Type == "audio" {
		typeLabel = "음원"
	} else if req.Type == "thumbnail" {
		typeLabel = "썸네일"
	}
	saveHistoryEntry(HistoryEntry{
		Title:     req.Title,
		Platform:  req.Platform,
		Type:      typeLabel,
		Thumbnail: req.Thumbnail,
		URL:       req.URL,
		Date:      time.Now().Format("2006-01-02 15:04"),
	})
	emit(progressEvent{Type: "done"})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL       string `json:"url"`
		Type      string `json:"type"`
		Quality   string `json:"quality"`
		Title     string `json:"title"`
		Platform  string `json:"platform"`
		Thumbnail string `json:"thumbnail"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	dlURL := normalizeURL(req.URL)
	platform := detectPlatform(dlURL)

	home, _ := os.UserHomeDir()
	dlDir := filepath.Join(home, "Downloads")

	// Threads 전용 다운로드
	if platform == "Threads" {
		sc := threadsShortcode(dlURL)
		if sc == "" {
			jsonError(w, "Invalid Threads URL", 400)
			return
		}
		videoURL, title, _, err := threadsVideoURL(sc)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		outPath, err := threadsDownload(videoURL, title, dlDir)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		saveHistoryEntry(HistoryEntry{
			Title:     title,
			Platform:  "Threads",
			Type:      "영상",
			Thumbnail: "",
			URL:       req.URL,
			Date:      time.Now().Format("2006-01-02 15:04"),
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "path": outPath})
		return
	}

	outTpl := filepath.Join(dlDir, "%(title).80s.%(ext)s")

	var err error
	switch req.Type {
	case "thumbnail":
		_, err = runYtdlp("--write-thumbnail", "--skip-download", "--convert-thumbnails", "jpg",
			"-o", filepath.Join(dlDir, "%(title).80s"), "--no-playlist", dlURL)
	case "audio":
		_, err = runYtdlp("-x", "--audio-format", "mp3", "--audio-quality", "0",
			"-o", outTpl, "--no-playlist", dlURL)
	default:
		_, err = runYtdlp(buildVideoArgs(req.Quality, outTpl, dlURL)...)
	}

	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	typeLabel := "영상"
	if req.Type == "audio" {
		typeLabel = "음원"
	} else if req.Type == "thumbnail" {
		typeLabel = "썸네일"
	}
	saveHistoryEntry(HistoryEntry{
		Title:     req.Title,
		Platform:  req.Platform,
		Type:      typeLabel,
		Thumbnail: req.Thumbnail,
		URL:       req.URL,
		Date:      time.Now().Format("2006-01-02 15:04"),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loadHistory())
}

func handleClearHistory(w http.ResponseWriter, r *http.Request) {
	os.Remove(historyFile)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleSetup(w http.ResponseWriter, r *http.Request) {
	err := ensureTools()
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"ready": true})
}

func handleCheckSetup(w http.ResponseWriter, r *http.Request) {
	_, e1 := os.Stat(ytdlpExe)
	_, e2 := os.Stat(ffmpegExe)
	ready := e1 == nil && e2 == nil
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ready": ready})
}

// ── Helpers ──

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

func getStringOr(m map[string]interface{}, key, fallback string) string {
	s := getString(m, key)
	if s == "" {
		return fallback
	}
	return s
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		}
	}
	return 0
}

// proxyAllowedHosts lists the CDN suffixes the thumbnail proxy will fetch from.
// Anything else is rejected to prevent the proxy being used for SSRF.
var proxyAllowedHosts = []string{
	"cdninstagram.com",
	"fbcdn.net",
	"fbsbx.com",
	"tiktokcdn.com",
	"tiktokcdn-us.com",
	"muscdn.com",
	"ttwstatic.com",
	"xhscdn.com",
	"douyinpic.com",
	"douyinstatic.com",
	"ytimg.com",
}

func proxyHostAllowed(host string) bool {
	host = strings.ToLower(host)
	for _, h := range proxyAllowedHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

// referrerFor picks a referer header that the upstream CDN expects so it
// won't 403 the request as a hotlink.
func referrerFor(host string) string {
	host = strings.ToLower(host)
	switch {
	case strings.Contains(host, "cdninstagram.com"):
		return "https://www.instagram.com/"
	case strings.Contains(host, "fbcdn.net"), strings.Contains(host, "fbsbx.com"):
		return "https://www.facebook.com/"
	case strings.Contains(host, "tiktokcdn"), strings.Contains(host, "muscdn"), strings.Contains(host, "ttwstatic"):
		return "https://www.tiktok.com/"
	case strings.Contains(host, "xhscdn"):
		return "https://www.xiaohongshu.com/"
	case strings.Contains(host, "douyin"):
		return "https://www.douyin.com/"
	case strings.Contains(host, "ytimg"):
		return "https://www.youtube.com/"
	}
	return ""
}

// transparent1x1Png is a minimal valid PNG used when the upstream thumbnail
// fetch fails — keeps the card layout stable instead of showing a broken-image icon.
var transparent1x1Png = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

func writePngPlaceholder(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	w.Write(transparent1x1Png)
}

// fetchThumbnail tries to fetch the upstream image and returns its bytes, content
// type, and an error. Builds the request with the host's expected Referer.
func fetchThumbnail(imgURL string, parsed *url.URL) ([]byte, string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", imgURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	if ref := referrerFor(parsed.Host); ref != "" {
		req.Header.Set("Referer", ref)
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("upstream %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "image/") {
		ct = "image/jpeg"
	}
	return body, ct, nil
}

func handleThumbProxy(w http.ResponseWriter, r *http.Request) {
	imgURL := r.URL.Query().Get("url")
	if imgURL == "" {
		writePngPlaceholder(w)
		return
	}

	parsed, err := url.Parse(imgURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		writePngPlaceholder(w)
		return
	}
	if !proxyHostAllowed(parsed.Host) {
		writePngPlaceholder(w)
		return
	}

	body, ct, err := fetchThumbnail(imgURL, parsed)
	if err != nil {
		// Soft-fail: serve a 1x1 PNG so the UI shows an empty thumbnail tile
		// instead of a broken-image icon.
		writePngPlaceholder(w)
		return
	}

	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(body)
}

func findPort() int {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func main() {
	runtime.LockOSThread()

	port := findPort()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/check-setup", handleCheckSetup)
	mux.HandleFunc("/api/setup", handleSetup)
	mux.HandleFunc("/api/info", handleGetInfo)
	mux.HandleFunc("/api/download", handleDownload)
	mux.HandleFunc("/api/download-stream", handleDownloadStream)
	mux.HandleFunc("/api/history", handleHistory)
	mux.HandleFunc("/api/history/clear", handleClearHistory)
	mux.HandleFunc("/api/thumb", handleThumbProxy)
	mux.HandleFunc("/api/local-thumb", handleLocalThumb)
	mux.HandleFunc("/api/update/status", handleUpdateStatus)
	mux.HandleFunc("/api/update/apply", handleApplyUpdate)
	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(updateScreenHTML))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexHTML))
	})

	go http.ListenAndServe(addr, mux)

	// Wait for server
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Synchronous startup update check — bounded by a 5s hard timeout so a
	// flaky network never makes the launch hang. If a newer build with a
	// downloadable asset is available we route the WebView straight to the
	// /update splash, which auto-applies and restarts. Otherwise launch the
	// main UI normally and keep the lazy background check for later.
	updateAvailable := hasStartupUpdate(5 * time.Second)
	if !updateAvailable {
		startBackgroundUpdateCheck()
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "SnapSave",
			Width:  900,
			Height: 700,
		},
	})
	if w == nil {
		fmt.Println("WebView2를 초기화할 수 없습니다. Microsoft Edge WebView2 Runtime을 설치해주세요.")
		os.Exit(1)
	}
	defer w.Destroy()

	w.SetSize(900, 700, webview2.HintNone)
	if updateAvailable {
		w.Navigate(fmt.Sprintf("http://%s/update", addr))
	} else {
		w.Navigate(fmt.Sprintf("http://%s", addr))
	}

	// Set taskbar/window icon after a short delay for window to be created
	go func() {
		time.Sleep(500 * time.Millisecond)
		setWindowIcon()
	}()

	w.Run()
}
