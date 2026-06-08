/**
 * Production alignment validation suite (P6).
 * Verifies one account, one engine, one MongoDB architecture.
 */

import { OWNER_ACCOUNT_KEY } from "./ownerAuth";
import {
  validateAccountKeyAlignment,
  FRONTEND_OWNER_ACCOUNT_KEY,
} from "./authoritativeAccountKey";
import { isEngineExecutionAuthority } from "./engineAuthority";
import { isMongoConfigured, pingMongo } from "./mongoTradesClient";
import {
  getClosedTradeStats,
  getPaperState,
  listOpenPositions,
  listPaperTrades,
  listPaperOrders,
  listStrategyHealth,
} from "./paperDeskClient";
import { getPortfolioAccountingSnapshot } from "./portfolioAccountingService";
import { runEnvChecks } from "./envCheck";

export type ValidationStatus = "PASS" | "WARNING" | "FAIL";

export type ValidationCheck = {
  id: string;
  status: ValidationStatus;
  message: string;
  detail?: string;
};

export type ProductionValidationReport = {
  overall: ValidationStatus;
  checked_at: string;
  account_key: string;
  checks: ValidationCheck[];
  go_no_go: "GO" | "NO-GO";
};

function worstStatus(checks: ValidationCheck[]): ValidationStatus {
  if (checks.some((c) => c.status === "FAIL")) return "FAIL";
  if (checks.some((c) => c.status === "WARNING")) return "WARNING";
  return "PASS";
}

export async function runProductionValidation(
  sessionAccountKey: string,
): Promise<ProductionValidationReport> {
  const checks: ValidationCheck[] = [];
  const accountKey = FRONTEND_OWNER_ACCOUNT_KEY;

  const keyAlign = validateAccountKeyAlignment();
  checks.push({
    id: "account_key_env",
    status: keyAlign.ok ? "PASS" : "FAIL",
    message: keyAlign.ok
      ? `Authoritative account key is ${accountKey}`
      : keyAlign.errors.join("; "),
    detail: keyAlign.warnings.join("; ") || undefined,
  });

  checks.push({
    id: "jwt_session_account",
    status: sessionAccountKey === accountKey ? "PASS" : "FAIL",
    message:
      sessionAccountKey === accountKey
        ? "JWT session userId matches owner account key"
        : `JWT userId "${sessionAccountKey}" != "${accountKey}"`,
  });

  const envReport = runEnvChecks();
  checks.push({
    id: "env_checks",
    status: envReport.ok ? "PASS" : "FAIL",
    message: envReport.ok
      ? "All required env vars pass"
      : envReport.checks.filter((c) => c.status === "FAIL").map((c) => c.key).join(", "),
  });

  checks.push({
    id: "engine_authority_mode",
    status: isEngineExecutionAuthority() ? "PASS" : "WARNING",
    message: isEngineExecutionAuthority()
      ? "Legacy TS worker disabled — Go engine is sole execution authority"
      : "ENGINE_EXECUTION_AUTHORITY=0 — legacy worker may dual-write MongoDB",
  });

  if (isMongoConfigured()) {
    const pingOk = await pingMongo();
    checks.push({
      id: "mongodb_ping",
      status: pingOk ? "PASS" : "FAIL",
      message: pingOk ? "MongoDB ping OK" : "MongoDB ping failed",
    });

    let paperState: Awaited<ReturnType<typeof getPaperState>> = null;
    try {
      paperState = await getPaperState(accountKey);
      checks.push({
        id: "paper_state",
        status: paperState ? "PASS" : "WARNING",
        message: paperState
          ? `paper_state found (balance=${paperState.balance ?? "?"})`
          : "No paper_state document for owner account — engine may not have started",
      });

      if (paperState && typeof paperState.balance === "number" && paperState.balance < 0) {
        checks.push({
          id: "balance_integrity",
          status: "FAIL",
          message: `Negative balance ${paperState.balance} USD in paper_state`,
        });
      } else {
        checks.push({
          id: "balance_integrity",
          status: "PASS",
          message: "Balance is non-negative",
        });
      }
    } catch (err) {
      checks.push({
        id: "paper_state",
        status: "FAIL",
        message: err instanceof Error ? err.message : "paper_state read failed",
      });
    }

    try {
      const positions = await listOpenPositions(accountKey);
      const missingProtection = positions.filter(
        (p) => !p.stop_loss || !p.take_profit,
      );
      checks.push({
        id: "open_positions",
        status: missingProtection.length > 0 ? "FAIL" : "PASS",
        message:
          missingProtection.length > 0
            ? `${missingProtection.length} open positions missing SL/TP`
            : `${positions.length} open positions all protected`,
      });
    } catch (err) {
      checks.push({
        id: "open_positions",
        status: "WARNING",
        message: err instanceof Error ? err.message : "paper_positions read failed",
      });
    }

    try {
      const trades = await listPaperTrades({ accountKey, limit: 1 });
      checks.push({
        id: "paper_trades",
        status: "PASS",
        message: `paper_trades readable (${trades.length >= 0 ? "ok" : "empty"})`,
      });
    } catch (err) {
      checks.push({
        id: "paper_trades",
        status: "WARNING",
        message: err instanceof Error ? err.message : "paper_trades read failed",
      });
    }

    try {
      await listPaperOrders({ accountKey, limit: 1 });
      checks.push({
        id: "paper_orders",
        status: "PASS",
        message: "paper_orders collection readable",
      });
    } catch (err) {
      checks.push({
        id: "paper_orders",
        status: "WARNING",
        message: err instanceof Error ? err.message : "paper_orders read failed",
      });
    }

    try {
      const health = await listStrategyHealth(accountKey);
      checks.push({
        id: "strategy_health",
        status: health.length > 0 ? "PASS" : "WARNING",
        message:
          health.length > 0
            ? `${health.length} strategy_health documents`
            : "strategy_health empty — wait for engine 15m cycle or check journal bootstrap",
      });
    } catch (err) {
      checks.push({
        id: "strategy_health",
        status: "WARNING",
        message: err instanceof Error ? err.message : "strategy_health read failed",
      });
    }

    try {
      const [closedStats, snapshot] = await Promise.all([
        getClosedTradeStats(accountKey),
        getPortfolioAccountingSnapshot(accountKey),
      ]);
      const pnlDrift = paperState
        ? Math.abs((paperState.realized_pnl ?? 0) - closedStats.realized_pnl)
        : 0;
      checks.push({
        id: "realized_pnl_mongo_authoritative",
        status:
          snapshot.realized_pnl === closedStats.realized_pnl ? "PASS" : "FAIL",
        message: `PortfolioAccountingService realized_pnl=${snapshot.realized_pnl}`,
        detail: `Mongo SUM(net_pnl)=${closedStats.realized_pnl}`,
      });
      checks.push({
        id: "paper_state_pnl_drift",
        status: !paperState || pnlDrift <= 50 ? "PASS" : "FAIL",
        message: paperState
          ? `paper_state vs SUM(net_pnl) drift=$${pnlDrift.toFixed(2)}`
          : "No paper_state — drift check skipped",
        detail: paperState
          ? `paper_state=${paperState.realized_pnl} mongo=${closedStats.realized_pnl}`
          : undefined,
      });
    } catch (err) {
      checks.push({
        id: "portfolio_accounting",
        status: "FAIL",
        message: err instanceof Error ? err.message : "Portfolio accounting check failed",
      });
    }
  } else {
    checks.push({
      id: "mongodb_ping",
      status: "FAIL",
      message: "MONGODB_URI not configured",
    });
  }

  const engineBase =
    process.env.INTERNAL_API_URL?.trim().replace(/\/$/, "") ??
    "http://localhost:8080";

  try {
    const res = await fetch(`${engineBase}/api/paper-desk/diagnostics`, {
      signal: AbortSignal.timeout(5_000),
    });
    if (!res.ok) {
      checks.push({
        id: "engine_running",
        status: "FAIL",
        message: `Engine diagnostics HTTP ${res.status}`,
      });
    } else {
      const diag = (await res.json()) as { account_key?: string; mongo_connected?: boolean };
      checks.push({
        id: "engine_running",
        status: "PASS",
        message: "Go engine reachable",
      });
      checks.push({
        id: "engine_account_key",
        status: diag.account_key === accountKey ? "PASS" : "FAIL",
        message:
          diag.account_key === accountKey
            ? "Engine account_key matches frontend"
            : `Engine account_key "${diag.account_key}" != "${accountKey}"`,
      });
      checks.push({
        id: "engine_mongodb",
        status: diag.mongo_connected ? "PASS" : "FAIL",
        message: diag.mongo_connected
          ? "Engine MongoDB connected"
          : "Engine reports MongoDB disconnected",
      });
    }
  } catch (err) {
    checks.push({
      id: "engine_running",
      status: "FAIL",
      message: err instanceof Error ? err.message : "Engine unreachable",
    });
  }

  const overall = worstStatus(checks);
  return {
    overall,
    checked_at: new Date().toISOString(),
    account_key: OWNER_ACCOUNT_KEY,
    checks,
    go_no_go: overall === "FAIL" ? "NO-GO" : "GO",
  };
}
