import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

// POST is disabled — trade close execution is now owned exclusively by the Go engine.
// The engine closes positions on SL/TP and writes to paper_trades.
// Read closed trades from /api/paper-desk/trades.
export async function POST() {
  return NextResponse.json(
    { ok: false, code: "DEPRECATED", error: "Browser trade close is disabled. The Go engine owns SL/TP execution. Read closed trades from /api/paper-desk/trades." },
    { status: 410 },
  );
}
