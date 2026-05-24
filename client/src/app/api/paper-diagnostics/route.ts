/**
 * GET /api/paper-diagnostics?account_key=xxx&window=20
 *
 * Returns per-strategy diagnostics and rolling health check.
 * Read-only. No writes.
 */
import { NextResponse } from "next/server";
import {
  computeRollingHealthCheck,
  computeStrategyDiagnostics,
} from "@/lib/futuresStrategyDiagnostics";
import { getTradesCollection, isMongoConfigured } from "@/lib/mongoTradesClient";
import type { PaperTradeDbRow } from "@/lib/paperTradesTypes";

export const dynamic = "force-dynamic";

const DIAGNOSTICS_PROJECTION = {
  strategy_id: 1,
  strategy_name: 1,
  template_family: 1,
  net_pnl: 1,
  gross_pnl: 1,
  fees: 1,
  exit_reason: 1,
  opened_at: 1,
  closed_at: 1,
} as const;

function parseWindow(raw: string | null): number {
  const n = parseInt(raw ?? "20", 10);
  if (!Number.isFinite(n)) return 20;
  return Math.min(100, Math.max(1, n));
}

export async function GET(req: Request) {
  const { searchParams } = new URL(req.url);
  const accountKey = searchParams.get("account_key")?.trim() ?? "";
  const windowN = parseWindow(searchParams.get("window"));

  if (!accountKey) {
    return NextResponse.json(
      { ok: false, error: "account_key is required" },
      { status: 400 },
    );
  }

  if (!isMongoConfigured()) {
    return NextResponse.json(
      { ok: false, error: "No MongoDB configured" },
      { status: 503 },
    );
  }

  try {
    const col = await getTradesCollection();
    const trades = await col
      .find(
        { account_key: accountKey, closed_at: { $exists: true } },
        { projection: DIAGNOSTICS_PROJECTION },
      )
      .sort({ closed_at: -1 })
      .limit(500)
      .toArray();

    const rows = trades as PaperTradeDbRow[];
    const diagnostics = computeStrategyDiagnostics(rows);
    const healthCheck = computeRollingHealthCheck(rows, windowN);

    return NextResponse.json({
      ok: true,
      diagnostics,
      healthCheck,
      tradeCount: rows.length,
      accountKey,
      window: windowN,
      source: "mongo",
    });
  } catch (err) {
    console.error("[paper-diagnostics]", err);
    return NextResponse.json(
      {
        ok: false,
        error: "internal_server_error",
        message: err instanceof Error ? err.message : "Mongo read failed",
      },
      { status: 500 },
    );
  }
}
