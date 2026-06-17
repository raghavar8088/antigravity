/**
 * GET /api/mock-trading/signal-trace
 *
 * Dry-run diagnostics: evaluates the mock-trading roster without opening trades.
 */
import { NextResponse } from "next/server";
import { collectMockAccountRiskContext, collectMockSignalTraceDiagnostic } from "@/lib/mockTradingExecutor/signalTraceDiagnostic";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";
export const maxDuration = 15;

export async function GET(req: Request): Promise<NextResponse> {
  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const symbol = url.searchParams.get("symbol") ?? "BTCUSD";

  try {
    const [trace, accountCtx] = await Promise.all([
      collectMockSignalTraceDiagnostic({ accountKey, symbol }),
      collectMockAccountRiskContext(accountKey),
    ]);

    return NextResponse.json({
      ok: trace.ok,
      timestamp: trace.timestamp,
      account_key: accountKey,
      market: {
        symbol: trace.symbol,
        price: trace.markPrice,
        regime: trace.regime,
        bars: trace.bars,
        dataFresh: trace.dataFresh,
      },
      account: {
        equity: accountCtx.account.equity,
        openCount: accountCtx.openCount,
        availableBalance: accountCtx.account.availableBalance,
      },
      candidateCount: trace.candidateCount,
      executableCount: trace.executableCount,
      blockerSummary: trace.blockerSummary,
      dominantBlocker: trace.dominantBlocker,
      summary: trace.summary,
      funnel: trace.funnel,
      rows: trace.rows,
      error: trace.marketError,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "signal trace failed";
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}
