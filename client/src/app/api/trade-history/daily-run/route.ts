import { NextResponse } from "next/server";
import {
  authorizeCronRequest,
  defaultArchiveDateUtc,
  readArchiveDateFromRequest,
  runDailyArchive,
} from "@/lib/tradeHistoryService";

export const dynamic = "force-dynamic";
export const maxDuration = 60;

async function handle(req: Request) {
  if (!process.env.DATABASE_URL) {
    return NextResponse.json({ ok: false, error: "DATABASE_URL not set" }, { status: 503 });
  }
  if (!authorizeCronRequest(req)) {
    return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
  }
  const date = (await readArchiveDateFromRequest(req)) ?? defaultArchiveDateUtc();
  try {
    const { inserted, scanned } = await runDailyArchive(date);
    return NextResponse.json({ ok: true, date, inserted, scanned });
  } catch (err) {
    console.error("[trade-history/daily-run]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}

/** Vercel Cron invokes GET with Authorization: Bearer CRON_SECRET */
export async function GET(req: Request) {
  return handle(req);
}

export async function POST(req: Request) {
  return handle(req);
}
