import { NextRequest, NextResponse } from "next/server";
import { getVideoInfo } from "@/lib/ytdlp";

export async function POST(request: NextRequest) {
  try {
    const { url } = await request.json();

    if (!url || typeof url !== "string") {
      return NextResponse.json({ error: "URL is required" }, { status: 400 });
    }

    const urlPattern = /^https?:\/\/.+/i;
    if (!urlPattern.test(url)) {
      return NextResponse.json({ error: "Invalid URL format" }, { status: 400 });
    }

    const info = await getVideoInfo(url);
    return NextResponse.json(info);
  } catch (error) {
    const message = error instanceof Error ? error.message : "Failed to fetch video info";
    return NextResponse.json({ error: message }, { status: 500 });
  }
}
