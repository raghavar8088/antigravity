import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

// POST is disabled — account snapshots are now written exclusively by the Go engine
// to paper_state every 10 seconds. Read account state from /api/paper-desk/state.
export async function POST() {
  return NextResponse.json(
    { ok: false, code: "DEPRECATED", error: "Browser account snapshots are disabled. The Go engine writes account state to paper_state every 10s. Read from /api/paper-desk/state." },
    { status: 410 },
  );
}
