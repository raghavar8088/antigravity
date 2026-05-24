/**
 * GET /api/paper-replay-compare?account_key=xxx&date=YYYY-MM-DD
 * Read-only live vs replay sign-flip comparison for a UTC day.
 */
import { NextResponse } from "next/server";
import { compareUtcDayReplay } from "@/lib/futuresReplayCompare";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const { searchParams } = new URL(req.url);
  const accountKey = searchParams.get("account_key")?.trim() ?? "";
  const date =
    searchParams.get("date")?.trim() ?? new Date().toISOString().slice(0, 10);

  if (!accountKey) {
    return NextResponse.json(
      { ok: false, error: "account_key is required" },
      { status: 400 },
    );
  }

  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) {
    return NextResponse.json(
      { ok: false, error: "date must be YYYY-MM-DD" },
      { status: 400 },
    );
  }

  try {
    const result = await compareUtcDayReplay(accountKey, date);
    if ("ok" in result && result.ok === false) {
      return NextResponse.json({ ok: false, error: result.error }, { status: 503 });
    }

    return NextResponse.json({
      ok: true,
      ...result,
    });
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    return NextResponse.json({ ok: false, error: msg }, { status: 500 });
  }
}
