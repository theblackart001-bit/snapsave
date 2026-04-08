import { execFile } from "child_process";
import path from "path";
import fs from "fs";

const YTDLP_PATH = process.env.YTDLP_PATH || "yt-dlp";

function getFfmpegDir(): string {
  try {
    const ffmpegStatic = require("ffmpeg-static") as string;
    return path.dirname(ffmpegStatic);
  } catch {
    return "/usr/bin";
  }
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

function runYtdlp(args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    const ffmpegDir = getFfmpegDir();
    execFile(
      YTDLP_PATH,
      ["--ffmpeg-location", ffmpegDir, ...args],
      { maxBuffer: 10 * 1024 * 1024, timeout: 60000 },
      (error, stdout, stderr) => {
        if (error) {
          reject(new Error(stderr || error.message));
        } else {
          resolve(stdout);
        }
      }
    );
  });
}

export async function getVideoInfo(url: string): Promise<VideoInfo> {
  const raw = await runYtdlp(["-j", "--no-playlist", url]);
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
): Promise<{ filePath: string; filename: string }> {
  const tmpDir = path.join(process.cwd(), "tmp-downloads");
  if (!fs.existsSync(tmpDir)) {
    fs.mkdirSync(tmpDir, { recursive: true });
  }

  const outputTemplate = path.join(tmpDir, "%(title).80s-%(id)s.%(ext)s");

  if (options.type === "thumbnail") {
    const infoRaw = await runYtdlp(["-j", "--no-playlist", options.url]);
    const info = JSON.parse(infoRaw);
    const thumbUrl = info.thumbnail;
    if (!thumbUrl) throw new Error("No thumbnail available");

    const thumbPath = path.join(tmpDir, `thumb-${info.id}.jpg`);
    await runYtdlp([
      "--write-thumbnail",
      "--skip-download",
      "--convert-thumbnails",
      "jpg",
      "-o",
      path.join(tmpDir, `thumb-%(id)s`),
      "--no-playlist",
      options.url,
    ]);

    // Find the downloaded thumbnail
    const files = fs.readdirSync(tmpDir).filter((f) => f.startsWith(`thumb-${info.id}`));
    if (files.length === 0) throw new Error("Thumbnail download failed");

    const filePath = path.join(tmpDir, files[0]);
    return { filePath, filename: `${info.title || info.id}-thumbnail.jpg` };
  }

  if (options.type === "audio") {
    const args = [
      "-x",
      "--audio-format",
      "mp3",
      "--audio-quality",
      "0",
      "-o",
      outputTemplate,
      "--no-playlist",
      options.url,
    ];
    await runYtdlp(args);
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
    await runYtdlp(args);
  }

  // Find the most recent file in tmp dir
  const files = fs.readdirSync(tmpDir)
    .map((f) => ({ name: f, time: fs.statSync(path.join(tmpDir, f)).mtimeMs }))
    .sort((a, b) => b.time - a.time);

  if (files.length === 0) throw new Error("Download failed");

  const filePath = path.join(tmpDir, files[0].name);
  return { filePath, filename: files[0].name };
}
