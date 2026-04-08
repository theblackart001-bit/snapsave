package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jchv/go-webview2"
)

var (
	binDir    string
	ytdlpExe  string
	ffmpegExe string
)

func init() {
	// bin/ is next to the exe
	exePath, _ := os.Executable()
	binDir = filepath.Join(filepath.Dir(exePath), "bin")
	ytdlpExe = filepath.Join(binDir, "yt-dlp.exe")
	ffmpegExe = filepath.Join(binDir, "ffmpeg.exe")
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

func runYtdlp(args ...string) (string, error) {
	allArgs := append([]string{"--ffmpeg-location", binDir, "--no-warnings"}, args...)
	cmd := exec.Command(ytdlpExe, allArgs...)
	cmd.SysProcAttr = hiddenWindowAttr()

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Filter noise from stderr
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

func detectPlatform(url string) string {
	u := strings.ToLower(url)
	switch {
	case strings.Contains(u, "youtube.com") || strings.Contains(u, "youtu.be"):
		return "YouTube"
	case strings.Contains(u, "instagram.com"):
		return "Instagram"
	case strings.Contains(u, "tiktok.com"):
		return "TikTok"
	case strings.Contains(u, "threads.net"):
		return "Threads"
	case strings.Contains(u, "facebook.com") || strings.Contains(u, "fb.watch"):
		return "Facebook"
	}
	return "Unknown"
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

	raw, err := runYtdlp("-j", "--no-playlist", req.URL)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	var data map[string]interface{}
	json.Unmarshal([]byte(raw), &data)

	info := VideoInfo{
		ID:        getString(data, "id"),
		Title:     getStringOr(data, "title", "Untitled"),
		Thumbnail: getString(data, "thumbnail"),
		Duration:  getInt(data, "duration"),
		Uploader:  getStringOr(data, "uploader", getStringOr(data, "channel", "Unknown")),
		Platform:  detectPlatform(req.URL),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL     string `json:"url"`
		Type    string `json:"type"`
		Quality string `json:"quality"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	home, _ := os.UserHomeDir()
	dlDir := filepath.Join(home, "Downloads")
	outTpl := filepath.Join(dlDir, "%(title).80s.%(ext)s")

	var err error
	switch req.Type {
	case "thumbnail":
		_, err = runYtdlp("--write-thumbnail", "--skip-download", "--convert-thumbnails", "jpg",
			"-o", filepath.Join(dlDir, "%(title).80s"), "--no-playlist", req.URL)
	case "audio":
		_, err = runYtdlp("-x", "--audio-format", "mp3", "--audio-quality", "0",
			"-o", outTpl, "--no-playlist", req.URL)
	default:
		args := []string{}
		if req.Quality != "" {
			h := strings.Replace(req.Quality, "p", "", 1)
			args = append(args, "-f", fmt.Sprintf("bestvideo[height<=%s]+bestaudio/best[height<=%s]/best", h, h))
		} else {
			args = append(args, "-f", "bestvideo+bestaudio/best")
		}
		args = append(args, "--merge-output-format", "mp4", "-o", outTpl, "--no-playlist", req.URL)
		_, err = runYtdlp(args...)
	}

	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
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
	w.Navigate(fmt.Sprintf("http://%s", addr))

	// Set taskbar/window icon after a short delay for window to be created
	go func() {
		time.Sleep(500 * time.Millisecond)
		setWindowIcon()
	}()

	w.Run()
}
