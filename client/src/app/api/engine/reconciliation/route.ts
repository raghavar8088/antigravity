import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getPaperState } from "@/lib/paperDeskClient";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

type ReconEvent = {
  ts: string;
  drift_amount: number;
  action: string;
  kill_switch_triggered: boolean;
  domain?: string;
};

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  const engineBase = process.env.INTERNAL_API_URL?.trim().replace(/\/$/, "") ?? "http://localhost:8080";
  const events: ReconEvent[] = [];
  let overall: "GREEN" | "AMBER" | "RED" = "GREEN";

  if (isMongoConfigured()) {
    try {
      const state = await getPaperState(auth.ctx.userId);
      if (state?.snapped_at) {
        const ageMs = Date.now() - new Date(state.snapped_at).getTime();
        const stale = ageMs > 120_000;
        overall = stale ? "AMBER" : "GREEN";
        events.push({
          ts: state.snapped_at,
          drift_amount: Math.abs(state.current_drawdown ?? 0),
          action: stale ? "STATE_STALE" : "STATE_OK",
          kill_switch_triggered: false,
          domain: "paper_state",
        });
      } else {
        overall = "AMBER";
        events.push({
          ts: new Date().toISOString(),
          drift_amount: 0,
          action: "NO_STATE",
          kill_switch_triggered: false,
        });
      }
    } catch {
      overall = "RED";
    }
  }

  try {
    const r = await fetch(`${engineBase}/api/reconciliation/status`, {
      signal: AbortSignal.timeout(2_000),
    });
    if (r.ok) {
      const data = await r.json();
      if (Array.isArray(data.events)) {
        for (const e of data.events) {
          events.push({
            ts: String(e.ts ?? e.completed_at ?? new Date().toISOString()),
            drift_amount: Number(e.drift_amount ?? e.drift_score ?? 0),
            action: String(e.action ?? e.status ?? "RECON_CYCLE"),
            kill_switch_triggered: Boolean(e.kill_switch_triggered),
            domain: String(e.domain ?? "engine"),
          });
        }
      }
      if (data.overall === "RED" || data.overall === "AMBER") overall = data.overall;
    }
  } catch {
    // engine endpoint optional — Mongo freshness is primary UI signal
  }

  const cutoff = Date.now() - 24 * 60 * 60 * 1000;
  const filtered = events
    .filter((e) => new Date(e.ts).getTime() >= cutoff)
    .sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime());

  return NextResponse.json({
    ok: true,
    overall,
    events: filtered,
    server_time: new Date().toISOString(),
  });
}
