/**
 * GET /api/diagnostics/health
 *
 * Aggregated health for mock-trading backend executor, MongoDB, and market data.
 */
import { NextResponse } from "next/server";
import { buildDiagnosticsHealthReport } from "@/lib/mockTradingExecutor/diagnosticsHealth";
import { recommendExecutorHealing } from "@/lib/mockTradingExecutor/healingEngine";
import { loadExecutorState } from "@/lib/mockTradingExecutor/persistExecutorState";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const symbol = url.searchParams.get("symbol") ?? "BTCUSD";

  try {
    const report = await buildDiagnosticsHealthReport({ accountKey, symbol });
    const state = await loadExecutorState(accountKey);
    const healing = recommendExecutorHealing(state);

    return NextResponse.json(
      {
        ...report,
        healing,
      },
      { status: report.healthy ? 200 : 503 },
    );
  } catch (err) {
    const message = err instanceof Error ? err.message : "health check failed";
    return NextResponse.json({ ok: false, healthy: false, error: message }, { status: 500 });
  }
}
