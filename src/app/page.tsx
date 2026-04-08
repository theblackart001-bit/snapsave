"use client";

import { useState, useCallback } from "react";

interface VideoInfo {
  id: string;
  title: string;
  thumbnail: string;
  duration: number;
  uploader: string;
  platform: string;
}

type DownloadType = "video" | "audio" | "thumbnail";

const PLATFORM_COLORS: Record<string, string> = {
  YouTube: "#ff0000",
  Instagram: "#e1306c",
  TikTok: "#00f2ea",
  Threads: "#ffffff",
  Facebook: "#1877f2",
};

const PLATFORM_ICONS: Record<string, string> = {
  YouTube: "▶",
  Instagram: "📷",
  TikTok: "♪",
  Threads: "@",
  Facebook: "f",
};

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0)
    return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  return `${m}:${String(s).padStart(2, "0")}`;
}


export default function Home() {
  const [url, setUrl] = useState("");
  const [videoInfo, setVideoInfo] = useState<VideoInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [error, setError] = useState("");

  const fetchInfo = useCallback(async () => {
    if (!url.trim()) return;

    setLoading(true);
    setError("");
    setVideoInfo(null);

    try {
      const res = await fetch("/api/info", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: url.trim() }),
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || "영상 정보를 가져올 수 없습니다");
        return;
      }

      setVideoInfo(data);
    } catch {
      setError("서버 연결에 실패했습니다");
    } finally {
      setLoading(false);
    }
  }, [url]);

  const handleDownload = useCallback(
    async (type: DownloadType) => {
      if (!url.trim()) return;

      setDownloading(type);

      try {
        const res = await fetch("/api/download", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            url: url.trim(),
            type,
            quality: type === "video" ? "1080p" : undefined,
          }),
        });

        if (!res.ok) {
          const data = await res.json();
          setError(data.error || "다운로드에 실패했습니다");
          return;
        }

        const blob = await res.blob();
        const disposition = res.headers.get("Content-Disposition");
        let filename = `download.${type === "audio" ? "mp3" : type === "thumbnail" ? "jpg" : "mp4"}`;
        if (disposition) {
          const match = disposition.match(/filename="?(.+?)"?$/);
          if (match) filename = decodeURIComponent(match[1]);
        }

        const a = document.createElement("a");
        a.href = URL.createObjectURL(blob);
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(a.href);
      } catch {
        setError("다운로드 중 오류가 발생했습니다");
      } finally {
        setDownloading(null);
      }
    },
    [url]
  );

  const platformColor = videoInfo
    ? PLATFORM_COLORS[videoInfo.platform] || "#6c5ce7"
    : "#6c5ce7";

  return (
    <main className="flex-1 flex flex-col items-center px-4 py-12">
      {/* Header */}
      <div className="text-center mb-10 animate-fade-up">
        <h1
          className="text-5xl font-bold tracking-tight mb-3"
          style={{
            background:
              "linear-gradient(135deg, #6c5ce7, #a29bfe, #74b9ff)",
            WebkitBackgroundClip: "text",
            WebkitTextFillColor: "transparent",
          }}
        >
          SnapSave
        </h1>
        <p style={{ color: "var(--text-secondary)" }} className="text-lg">
          YouTube, Instagram, TikTok, Threads, Facebook
        </p>
        <div className="flex gap-3 justify-center mt-4">
          {Object.entries(PLATFORM_ICONS).map(([name, icon]) => (
            <span
              key={name}
              className="w-9 h-9 rounded-full flex items-center justify-center text-sm font-bold"
              style={{
                background: `${PLATFORM_COLORS[name]}20`,
                color: PLATFORM_COLORS[name],
                border: `1px solid ${PLATFORM_COLORS[name]}30`,
              }}
              title={name}
            >
              {icon}
            </span>
          ))}
        </div>
      </div>

      {/* URL Input */}
      <div
        className="w-full max-w-2xl animate-fade-up"
        style={{ animationDelay: "0.1s" }}
      >
        <div
          className="flex gap-0 rounded-2xl overflow-hidden"
          style={{
            background: "var(--bg-secondary)",
            border: "1px solid var(--border)",
          }}
        >
          <input
            type="url"
            placeholder="영상 URL을 붙여넣으세요..."
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && fetchInfo()}
            className="flex-1 px-5 py-4 text-base outline-none"
            style={{
              background: "transparent",
              color: "var(--text-primary)",
            }}
          />
          <button
            onClick={fetchInfo}
            disabled={loading || !url.trim()}
            className="px-6 py-4 font-semibold text-white transition-all duration-200 cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            style={{
              background: loading
                ? "var(--bg-tertiary)"
                : "var(--accent)",
            }}
            onMouseEnter={(e) => {
              if (!loading)
                e.currentTarget.style.background = "var(--accent-hover)";
            }}
            onMouseLeave={(e) => {
              if (!loading)
                e.currentTarget.style.background = "var(--accent)";
            }}
          >
            {loading ? (
              <span className="flex items-center gap-2">
                <Spinner />
                분석 중...
              </span>
            ) : (
              "분석"
            )}
          </button>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div
          className="mt-4 px-5 py-3 rounded-xl text-sm max-w-2xl w-full animate-fade-up"
          style={{
            background: "rgba(255, 107, 107, 0.1)",
            border: "1px solid rgba(255, 107, 107, 0.2)",
            color: "var(--danger)",
          }}
        >
          {error}
        </div>
      )}

      {/* Loading skeleton */}
      {loading && (
        <div
          className="mt-8 w-full max-w-2xl rounded-2xl p-6"
          style={{
            background: "var(--bg-secondary)",
            border: "1px solid var(--border)",
          }}
        >
          <div className="flex gap-5">
            <div className="w-48 h-28 rounded-xl shimmer flex-shrink-0" />
            <div className="flex-1 space-y-3">
              <div className="h-5 w-3/4 rounded shimmer" />
              <div className="h-4 w-1/2 rounded shimmer" />
              <div className="h-4 w-1/3 rounded shimmer" />
            </div>
          </div>
        </div>
      )}

      {/* Video Info Card */}
      {videoInfo && !loading && (
        <div
          className="mt-8 w-full max-w-2xl rounded-2xl overflow-hidden animate-fade-up"
          style={{
            background: "var(--bg-secondary)",
            border: "1px solid var(--border)",
          }}
        >
          {/* Video preview */}
          <div className="flex gap-0">
            {videoInfo.thumbnail && (
              <div className="relative w-72 flex-shrink-0">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={videoInfo.thumbnail}
                  alt={videoInfo.title}
                  className="w-full h-full object-cover"
                  style={{ minHeight: 160 }}
                />
                {videoInfo.duration > 0 && (
                  <span
                    className="absolute bottom-2 right-2 px-2 py-0.5 rounded text-xs font-mono font-medium"
                    style={{
                      background: "rgba(0,0,0,0.8)",
                      color: "#fff",
                    }}
                  >
                    {formatDuration(videoInfo.duration)}
                  </span>
                )}
              </div>
            )}
            <div className="flex-1 p-5">
              <div className="flex items-center gap-2 mb-2">
                <span
                  className="px-2 py-0.5 rounded-full text-xs font-bold"
                  style={{
                    background: `${platformColor}20`,
                    color: platformColor,
                    border: `1px solid ${platformColor}30`,
                  }}
                >
                  {videoInfo.platform}
                </span>
              </div>
              <h2
                className="text-lg font-semibold leading-snug mb-2"
                style={{ color: "var(--text-primary)" }}
              >
                {videoInfo.title}
              </h2>
              <p
                className="text-sm"
                style={{ color: "var(--text-secondary)" }}
              >
                {videoInfo.uploader}
              </p>
            </div>
          </div>

          {/* Controls */}
          <div
            className="p-5 space-y-5"
            style={{ borderTop: "1px solid var(--border)" }}
          >
            {/* Download buttons */}
            <div className="grid grid-cols-3 gap-3">
              <DownloadButton
                label="영상 다운로드"
                sublabel="MP4 1080p"
                icon={
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                  >
                    <polygon points="5 3 19 12 5 21 5 3" />
                  </svg>
                }
                loading={downloading === "video"}
                onClick={() => handleDownload("video")}
                color="var(--accent)"
              />
              <DownloadButton
                label="음원 추출"
                sublabel="MP3"
                icon={
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                  >
                    <path d="M9 18V5l12-2v13" />
                    <circle cx="6" cy="18" r="3" />
                    <circle cx="18" cy="16" r="3" />
                  </svg>
                }
                loading={downloading === "audio"}
                onClick={() => handleDownload("audio")}
                color="#51cf66"
              />
              <DownloadButton
                label="썸네일"
                sublabel="JPG"
                icon={
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                  >
                    <rect x="3" y="3" width="18" height="18" rx="2" />
                    <circle cx="8.5" cy="8.5" r="1.5" />
                    <path d="M21 15l-5-5L5 21" />
                  </svg>
                }
                loading={downloading === "thumbnail"}
                onClick={() => handleDownload("thumbnail")}
                color="#74b9ff"
              />
            </div>
          </div>
        </div>
      )}

      {/* Footer */}
      <div
        className="mt-auto pt-16 pb-6 text-center text-xs"
        style={{ color: "var(--text-secondary)", opacity: 0.5 }}
      >
        SnapSave &mdash; 개인 사용 목적으로만 이용하세요
      </div>
    </main>
  );
}

function Spinner() {
  return (
    <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none">
      <circle
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="3"
        className="opacity-25"
      />
      <path
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
        className="opacity-75"
      />
    </svg>
  );
}

interface DownloadButtonProps {
  label: string;
  sublabel: string;
  icon: React.ReactNode;
  loading: boolean;
  onClick: () => void;
  color: string;
}

function DownloadButton({
  label,
  sublabel,
  icon,
  loading,
  onClick,
  color,
}: DownloadButtonProps) {
  return (
    <button
      onClick={onClick}
      disabled={loading}
      className="flex flex-col items-center gap-1.5 py-4 px-3 rounded-xl transition-all duration-200 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
      style={{
        background: "var(--bg-tertiary)",
        border: "1px solid var(--border)",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = color;
        e.currentTarget.style.background = `${color}10`;
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = "var(--border)";
        e.currentTarget.style.background = "var(--bg-tertiary)";
      }}
    >
      {loading ? (
        <Spinner />
      ) : (
        <span style={{ color }}>{icon}</span>
      )}
      <span
        className="text-sm font-medium"
        style={{ color: "var(--text-primary)" }}
      >
        {label}
      </span>
      <span className="text-xs" style={{ color: "var(--text-secondary)" }}>
        {sublabel}
      </span>
    </button>
  );
}
