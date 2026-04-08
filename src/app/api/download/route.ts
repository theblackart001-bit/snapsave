import { NextRequest, NextResponse } from "next/server";
import { downloadMedia } from "@/lib/ytdlp";

export async function POST(request: NextRequest) {
  try {
    const { url, type, formatId, quality } = await request.json();

    if (!url || typeof url !== "string") {
      return NextResponse.json({ error: "URL is required" }, { status: 400 });
    }

    const urlPattern = /^https?:\/\/.+/i;
    if (!urlPattern.test(url)) {
      return NextResponse.json({ error: "Invalid URL format" }, { status: 400 });
    }

    const validTypes = ["video", "audio", "thumbnail"];
    if (!validTypes.includes(type)) {
      return NextResponse.json({ error: "Invalid type" }, { status: 400 });
    }

    const { filename } = await downloadMedia({
      url,
      type,
      formatId,
      quality,
    });

    return NextResponse.json({ success: true, filename });
  } catch (error) {
    const message = error instanceof Error ? error.message : "Download failed";
    return NextResponse.json({ error: message }, { status: 500 });
  }
}
