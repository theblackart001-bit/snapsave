import { NextRequest, NextResponse } from "next/server";
import { downloadMedia } from "@/lib/ytdlp";
import fs from "fs";

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

    const { filePath, filename } = await downloadMedia({
      url,
      type,
      formatId,
      quality,
    });

    const fileBuffer = fs.readFileSync(filePath);

    // Clean up after reading
    fs.unlinkSync(filePath);

    const contentType =
      type === "audio"
        ? "audio/mpeg"
        : type === "thumbnail"
          ? "image/jpeg"
          : "video/mp4";

    return new NextResponse(fileBuffer, {
      headers: {
        "Content-Type": contentType,
        "Content-Disposition": `attachment; filename="${encodeURIComponent(filename)}"`,
        "Content-Length": fileBuffer.length.toString(),
      },
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "Download failed";
    return NextResponse.json({ error: message }, { status: 500 });
  }
}
