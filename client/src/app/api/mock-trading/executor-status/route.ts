/**
 * GET /api/mock-trading/executor-status
 *
 * Read-only health + last-cycle telemetry for the backend mock-trading executor.
 */

import { NextResponse } from "next/server";
import { buildExecutorDiagnosis } from "@/lib/mockTradingExecutor/diagnoseExecutor";
import { getExecutorConfigView } from "@/lib/mockTradingExecutor/executorConfig";
import { recommendExecutorHealing } from "@/lib/mockTradingExecutor/healingEngine";
import {
  executorStateHealth,
  loadExecutorState,
} from "@/lib/mockTradingExecutor/persistExecutorState";
import type { MockExecutorHealth } from "@/lib/mockTradingExecutor/types";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";
import {
  getLatestMockAccountSnapshot,
  listMockTrades,
} from "@/lib/trading/mockTradingMongo";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;

  try {
    const [state, accountSnap, recentTrades, thresholdConfig] = await Promise.all([
      loadExecutorState(accountKey),
      getLatestMockAccountSnapshot(accountKey),
      listMockTrades({
        account_key: accountKey,
        page: 1,
        limit: 10,
        sort: "newest",
      }),
      getExecutorConfigView(accountKey),
    ]);

    const healthFlags = executorStateHealth(state);
    const diagnosis = buildExecutorDiagnosis(state);
    const healing = recommendExecutorHealing(state);

    const executor: MockExecutorHealth = {
      healthy: healthFlags.healthy,
      stale: healthFlags.stale,
      ageSeconds: healthFlags.ageSeconds,
      lastTickAt: state?.last_tick_at ?? null,
      lastMode: state?.last_mode ?? null,
      dominantBlocker: state?.last_dominant_blocker ?? state?.entry_funnel_snapshot?.dominantBlocker ?? null,
      candidateCount: state?.last_candidate_count ?? null,
      openedLastCycle: state?.last_opened ?? null,
      closedLastCycle: state?.last_closed ?? null,
      errors: state?.last_error ? [state.last_error] : [],
    };

    return NextResponse.json({
      ok: true,
      account_key: accountKey,
      executor,
      funnel: state?.entry_funnel_snapshot ?? null,
      account: accountSnap.account,
      config: accountSnap.config,
      threshold_config: thresholdConfig,
      recent_trades: recentTrades.trades,
      no_trade_diagnosis: diagnosis,
      healing,
      server_time: new Date().toISOString(),
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "executor status failed";
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}
