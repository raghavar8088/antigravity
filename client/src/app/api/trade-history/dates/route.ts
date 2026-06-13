import { NextResponse } from "next/server";
import { listArchiveDays } from "@/lib/portfolio/tradeHistoryService";

export const dynamic = "force-dynamic";

export async function GET() {
  if (!process.env.DATABASE_URL) {
    return NextResponse.json({ ok: true, days: [], disabled: true, reason: "DATABASE_URL not set" });
  }
  try {
    const days = await listArchiveDays();
    return NextResponse.json({ ok: true, days });
  } catch (err) {
    console.error("[trade-history/dates]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}
