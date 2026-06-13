import { NextRequest, NextResponse } from "next/server";
import { buildWorkbookForDay, isIsoDate } from "@/lib/portfolio/tradeHistoryService";

export const dynamic = "force-dynamic";
export const maxDuration = 60;

export async function GET(req: NextRequest) {
  if (!process.env.DATABASE_URL) {
    return NextResponse.json({ ok: false, error: "DATABASE_URL not set" }, { status: 503 });
  }
  const day = req.nextUrl.searchParams.get("date");
  if (!day || !isIsoDate(day)) {
    return NextResponse.json({ ok: false, error: "Query date=YYYY-MM-DD is required" }, { status: 400 });
  }
  try {
    const body = await buildWorkbookForDay(day);
    return new NextResponse(new Uint8Array(body), {
      status: 200,
      headers: {
        "Content-Type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "Content-Disposition": `attachment; filename="trade-history-${day}.xlsx"`,
        "Cache-Control": "no-store",
      },
    });
  } catch (err) {
    console.error("[trade-history/download]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}
