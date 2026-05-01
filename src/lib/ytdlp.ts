import { execFile } from "child_process";
import path from "path";
import fs from "fs";
import os from "os";

function getYtdlpPath(): string {
  if (process.env.YTDLP_PATH && fs.existsSync(process.env.YTDLP_PATH)) {
    return process.env.YTDLP_PATH;
  }
  // Check project bin directory
  const binPath = path.join(process.cwd(), "bin", "yt-dlp.exe");
  if (fs.existsSync(binPath)) return binPath;
  return "yt-dlp";
}

function getFfmpegDir(): string {
  if (process.env.FFMPEG_DIR && fs.existsSync(process.env.FFMPEG_DIR)) {
    return process.env.FFMPEG_DIR;
  }
  // Check project bin directory
  const binDir = path.join(process.cwd(), "bin");
  if (fs.existsSync(path.join(binDir, "ffmpeg.exe"))) return binDir;
  // Try ffmpeg-static package
  try {
    const ffmpegStatic = require("ffmpeg-static") as string;
    if (ffmpegStatic && fs.existsSync(ffmpegStatic)) {
      return path.dirname(ffmpegStatic);
    }
  } catch {}
  if (process.platform !== "win32") return "/usr/bin";
  return "";
}

function getDownloadsDir(): string {
  return path.join(os.homedir(), "Downloads");
}

export interface VideoInfo {
  id: string;
  title: string;
  thumbnail: string;
  duration: number;
  uploader: string;
  platform: string;
  formats: FormatInfo[];
}

export interface FormatInfo {
  formatId: string;
  ext: string;
  resolution: string;
  filesize: number | null;
  vcodec: string;
  acodec: string;
  label: string;
}

function detectPlatform(url: string): string {
  if (/youtube\.com|youtu\.be/i.test(url)) return "YouTube";
  if (/instagram\.com/i.test(url)) return "Instagram";
  if (/tiktok\.com/i.test(url)) return "TikTok";
  if (/threads\.net/i.test(url)) return "Threads";
  if (/facebook\.com|fb\.watch/i.test(url)) return "Facebook";
  return "Unknown";
}

// Platforms that frequently need login cookies (private/age-gated/region-locked content)
function needsCookies(url: string): boolean {
  return /instagram\.com|threads\.net|facebook\.com|fb\.watch/i.test(url);
}

const COOKIE_BROWSERS = ["chrome", "edge", "firefox", "brave"] as const;

function runYtdlp(args: string[], opts: { cookieBrowser?: string } = {}): Promise<string> {
  return new Promise((resolve, reject) => {
    const ffmpegDir = getFfmpegDir();
    const ffmpegArgs = ffmpegDir ? ["--ffmpeg-location", ffmpegDir] : [];
    const cookieArgs = opts.cookieBrowser
      ? ["--cookies-from-browser", opts.cookieBrowser]
      : [];
    execFile(
      getYtdlpPath(),
      [...ffmpegArgs, ...cookieArgs, ...args],
      { maxBuffer: 10 * 1024 * 1024, timeout: 120000 },
      (error, stdout, stderr) => {
        if (error) {
          // Filter out WARNING lines, keep only actual errors
          const errorLines = (stderr || error.message)
            .split("\n")
            .filter((l: string) => !l.startsWith("WARNING:"))
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

// Run yt-dlp with cookie fallback: try without cookies first, then each supported
// browser if the error indicates a login/visibility issue. This way YouTube etc.
// stay fast and Instagram/Threads automatically pick up the user's Chrome session.
async function runYtdlpWithCookieFallback(
  url: string,
  args: string[]
): Promise<string> {
  // For platforms that often need login, try Chrome cookies first to avoid
  // the obvious "login required" failure, then fall back to other browsers.
  const browsers = needsCookies(url)
    ? [...COOKIE_BROWSERS, undefined]
    : [undefined, ...COOKIE_BROWSERS];

  let lastError: Error | null = null;
  for (const browser of browsers) {
    try {
      return await runYtdlp(args, { cookieBrowser: browser });
    } catch (e) {
      const err = e instanceof Error ? e : new Error(String(e));
      const msg = err.message.toLowerCase();
      lastError = err;

      // Cookie extraction failed (browser locked, no profile) — try next browser
      if (
        msg.includes("could not copy") ||
        msg.includes("could not find") ||
        msg.includes("unable to read") ||
        msg.includes("permission denied") ||
        msg.includes("no such file")
      ) {
        continue;
      }

      // Login/visibility errors — try next browser (a different login may work)
      if (
        msg.includes("login required") ||
        msg.includes("rate-limit") ||
        msg.includes("not available to everyone") ||
        msg.includes("can't be seen") ||
        msg.includes("private")
      ) {
        continue;
      }

      // Any other error: stop and surface it
      throw err;
    }
  }
  throw lastError ?? new Error("다운로드 중 오류가 발생했습니다");
}

export async function getVideoInfo(url: string): Promise<VideoInfo> {
  const raw = await runYtdlpWithCookieFallback(url, ["-j", "--no-playlist", url]);
  const data = JSON.parse(raw);

  const formats: FormatInfo[] = (data.formats || [])
    .filter(
      (f: Record<string, unknown>) =>
        f.vcodec !== "none" || f.acodec !== "none"
    )
    .map((f: Record<string, unknown>) => {
      const height = f.height as number | undefined;
      const vcodec = (f.vcodec as string) || "none";
      const acodec = (f.acodec as string) || "none";
      let label = "";
      if (vcodec !== "none" && acodec !== "none") {
        label = `${height || "?"}p (video+audio)`;
      } else if (vcodec !== "none") {
        label = `${height || "?"}p (video only)`;
      } else {
        label = `audio only (${f.ext})`;
      }

      return {
        formatId: f.format_id as string,
        ext: f.ext as string,
        resolution: height ? `${height}p` : "audio",
        filesize: (f.filesize as number) || null,
        vcodec,
        acodec,
        label,
      };
    });

  // Deduplicate: keep best format per resolution
  const bestFormats = new Map<string, FormatInfo>();
  for (const f of formats) {
    const key = f.label;
    const existing = bestFormats.get(key);
    if (!existing || (f.filesize && existing.filesize && f.filesize > existing.filesize)) {
      bestFormats.set(key, f);
    }
  }

  return {
    id: data.id,
    title: data.title || "Untitled",
    thumbnail: data.thumbnail || "",
    duration: data.duration || 0,
    uploader: data.uploader || data.channel || "Unknown",
    platform: detectPlatform(url),
    formats: Array.from(bestFormats.values()),
  };
}

export interface DownloadOptions {
  url: string;
  type: "video" | "audio" | "thumbnail";
  formatId?: string;
  quality?: string;
}

export async function downloadMedia(
  options: DownloadOptions
): Promise<{ filename: string }> {
  const downloadsDir = getDownloadsDir();
  const outputTemplate = path.join(downloadsDir, "%(title).80s.%(ext)s");

  if (options.type === "thumbnail") {
    const infoRaw = await runYtdlpWithCookieFallback(options.url, [
      "-j",
      "--no-playlist",
      options.url,
    ]);
    const info = JSON.parse(infoRaw);
    if (!info.thumbnail) throw new Error("No thumbnail available");

    await runYtdlpWithCookieFallback(options.url, [
      "--write-thumbnail",
      "--skip-download",
      "--convert-thumbnails",
      "jpg",
      "-o",
      path.join(downloadsDir, "%(title).80s"),
      "--no-playlist",
      options.url,
    ]);

    return { filename: `${(info.title || info.id).slice(0, 80)}.jpg` };
  }

  if (options.type === "audio") {
    await runYtdlpWithCookieFallback(options.url, [
      "-x",
      "--audio-format",
      "mp3",
      "--audio-quality",
      "0",
      "-o",
      outputTemplate,
      "--no-playlist",
      options.url,
    ]);
  } else {
    const args: string[] = [];
    if (options.formatId) {
      args.push("-f", `${options.formatId}+bestaudio/best`);
    } else if (options.quality) {
      const height = options.quality.replace("p", "");
      args.push("-f", `bestvideo[height<=${height}]+bestaudio/best[height<=${height}]/best`);
    } else {
      args.push("-f", "bestvideo+bestaudio/best");
    }
    args.push("--merge-output-format", "mp4", "-o", outputTemplate, "--no-playlist", options.url);
    await runYtdlpWithCookieFallback(options.url, args);
  }

  return { filename: "다운로드 폴더에 저장됨" };
}
