/**
 * Self-healing recommendation engine for the BTC Future Trading paper desk.
 *
 * Pure function — no I/O. Maps an AiAppTrackerSnapshot to a prioritized list
 * of structured DeskHealingAction objects (vs the free-form recommendation
 * strings produced by summarizeTrackerSnapshot).
 *
 * Hard invariants — PR-SELF-HEALING-DESK constraints:
 *   - Paper only. No live Delta order placement is ever recommended.
 *   - Never lowers thresholds, bypasses fee/ATR/regime gates, or alters PnL.
 *   - `safeToAutomate: true` means an automation layer COULD trigger this
 *     action without operator review — it does NOT mean it will. All actual
 *     execution stays gated behind explicit env flags + operator approval.
 *   - copyableCommand strings never embed secrets. Account keys and URIs
 *     come from the operator's shell environment (`$DESK_WORKER_ACCOUNT_KEY`,
 *     `$APP_URL`, …) when the command is run.
 */
import type { AiAppTrackerSnapshot } from "./aiAppTracker/types";

// ─── Public types ────────────────────────────────────────────────────────────

export type DeskHealingActionType =
  | "REPAIR_STATE"
  | "RESTART_WORKER"
  | "WAIT_FOR_MARKET"
  | "REDUCE_ROSTER"
  | "DISABLE_BAD_STRATEGIES"
  | "COLLECT_DATA"
  | "CHECK_DEPLOYMENT"
  | "NO_ACTION";

export interface DeskHealingAction {
  type: DeskHealingActionType;
  severity: "info" | "warning" | "danger";
  title: string;
  reason: string;
  operatorAction: string;
  safeToAutomate: boolean;
  copyableCommand?: string;
  relatedFiles?: string[];
}

// ─── Thresholds ──────────────────────────────────────────────────────────────

const LARGE_DRIFT_USD = 100;
const MILD_DRIFT_USD = 50;
const LOW_ACTIVE_STRATEGIES = 4;

const SEVERITY_RANK: Record<DeskHealingAction["severity"], number> = {
  danger: 0,
  warning: 1,
  info: 2,
};

// ─── Individual rule builders (each returns one action or null) ──────────────

function checkDeployment(snap: AiAppTrackerSnapshot): DeskHealingAction | null {
  if (!snap.accountKeySuffix) {
    return {
      type: "CHECK_DEPLOYMENT",
      severity: "danger",
      title: "Account key missing",
      reason:
        "DESK_WORKER_ACCOUNT_KEY env var is unset; the tracker cannot read paper_state from MongoDB.",
      operatorAction:
        "Set DESK_WORKER_ACCOUNT_KEY on the Vercel project AND the VPS .env (same value), then redeploy.",
      safeToAutomate: false,
      relatedFiles: [
        "client/scripts/btc-ft-paper-worker.ts",
        "client/AI_README.md",
      ],
    };
  }
  if (snap.warnings.some((w) => w.includes("MongoDB is not configured"))) {
    return {
      type: "CHECK_DEPLOYMENT",
      severity: "danger",
      title: "MongoDB not configured",
      reason:
        "MONGODB_URI is missing or invalid. All desk state lives in Atlas — without it the worker cannot persist or read state.",
      operatorAction:
        "Set MONGODB_URI to the Atlas M0 connection string in Vercel env and on the VPS .env. Redeploy.",
      safeToAutomate: false,
      relatedFiles: ["client/src/lib/mongoTradesClient.ts"],
    };
  }
  return null;
}

function checkPaperState(snap: AiAppTrackerSnapshot): DeskHealingAction | null {
  if (snap.warnings.some((w) => w.includes("No paper_state found"))) {
    return {
      type: "REPAIR_STATE",
      severity: "danger",
      title: "No paper_state document",
      reason:
        "Account key resolves but no paper_state document exists in MongoDB. Worker has never run or the key is wrong.",
      operatorAction:
        "Either start the VPS worker (`pm2 start btc-ft-paper-worker`) or initialize state via the repair endpoint.",
      safeToAutomate: false,
      copyableCommand:
        'curl -X POST "$APP_URL/api/paper-state/repair" -H "Content-Type: application/json" -d \'{"accountKey":"\'$DESK_WORKER_ACCOUNT_KEY\'","initialBalance":1000,"reason":"initialize"}\'',
      relatedFiles: ["client/src/app/api/paper-state/repair/route.ts"],
    };
  }
  return null;
}

function checkWorker(snap: AiAppTrackerSnapshot): DeskHealingAction | null {
  if (snap.worker.stale) {
    return {
      type: "RESTART_WORKER",
      severity: "danger",
      title: "Worker heartbeat stale",
      reason: `Worker last polled ${snap.worker.ageSeconds ?? "?"}s ago (threshold 90s). VPS pm2 process may have crashed; only the Vercel cron fallback is firing.`,
      operatorAction:
        "SSH to the AWS Lightsail VPS and restart the worker. Vercel cron continues to fire every 60s while you investigate.",
      safeToAutomate: false,
      copyableCommand:
        "pm2 restart btc-ft-paper-worker && pm2 logs btc-ft-paper-worker --lines 50",
      relatedFiles: ["client/scripts/btc-ft-paper-worker.ts"],
    };
  }
  if (snap.warnings.some((w) => w.includes("funnel snapshot is stale"))) {
    return {
      type: "RESTART_WORKER",
      severity: "warning",
      title: "Entry funnel snapshot stale",
      reason:
        "Worker heartbeat is fresh but the funnel snapshot has not updated. Worker may be looping without persisting (MongoDB write failure, symbol fetch loop).",
      operatorAction:
        "Restart the VPS worker and tail logs for MongoDB write or fetch errors.",
      safeToAutomate: false,
      copyableCommand:
        "pm2 restart btc-ft-paper-worker && pm2 logs btc-ft-paper-worker --lines 100",
      relatedFiles: [
        "client/scripts/btc-ft-paper-worker.ts",
        "client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts",
      ],
    };
  }
  return null;
}

function checkBalanceDrift(snap: AiAppTrackerSnapshot): DeskHealingAction | null {
  const drift = snap.paperState.balanceDriftUsd;
  if (drift == null) return null;

  const abs = Math.abs(drift);
  const sign = drift >= 0 ? "+" : "";

  if (abs > LARGE_DRIFT_USD) {
    const safe = snap.paperState.openPositions === 0;
    return {
      type: "REPAIR_STATE",
      severity: "danger",
      title: `Large balance drift (${sign}$${drift.toFixed(2)})`,
      reason:
        "Drift > $100 since day start. With no open positions this almost always means probe/bootstrap-trade contamination. Repair preserves history but clears positions and resets balance.",
      operatorAction: safe
        ? "Safe to repair — no open positions. POST /api/paper-state/repair to reset balance."
        : "Wait for open positions to close, or close them manually in the UI, before repairing.",
      safeToAutomate: safe,
      copyableCommand:
        'curl -X POST "$APP_URL/api/paper-state/repair" -H "Content-Type: application/json" -d \'{"accountKey":"\'$DESK_WORKER_ACCOUNT_KEY\'","initialBalance":1000,"reason":"drift-repair"}\'',
      relatedFiles: [
        "client/src/app/api/paper-state/repair/route.ts",
        "client/src/lib/futuresDeskPnLTracker.ts",
      ],
    };
  }
  if (abs >= MILD_DRIFT_USD) {
    return {
      type: "COLLECT_DATA",
      severity: "warning",
      title: `Mild balance drift (${sign}$${drift.toFixed(2)})`,
      reason:
        "Drift $50–$100. Could be real session PnL or probe contamination — verify before repairing.",
      operatorAction:
        "Run `npm run ai:summary` and inspect the last 10 trades. Repair only if drift looks synthetic (no matching closed-trade PnL).",
      safeToAutomate: false,
      relatedFiles: ["client/src/lib/futuresDeskPnLTracker.ts"],
    };
  }
  return null;
}

function checkPauseEntries(snap: AiAppTrackerSnapshot): DeskHealingAction | null {
  if (!snap.paperState.pauseEntries) return null;
  return {
    type: "REPAIR_STATE",
    severity: "warning",
    title: "Entries paused",
    reason: "paper_state.pause_entries=true — no new positions will open.",
    operatorAction:
      "If pause was intentional, leave it. Otherwise resume via the UI toggle or POST /api/paper-state/repair (which also clears the pause flag).",
    safeToAutomate: false,
    relatedFiles: ["client/src/app/api/paper-state/repair/route.ts"],
  };
}

function checkRoster(snap: AiAppTrackerSnapshot): DeskHealingAction | null {
  const active = snap.entryFunnel.activeStrategies;
  if (active == null || active <= 0 || active >= LOW_ACTIVE_STRATEGIES) return null;
  return {
    type: "REDUCE_ROSTER",
    severity: "warning",
    title: `Only ${active} active strateg${active === 1 ? "y" : "ies"}`,
    reason:
      "Active roster is below 4 strategies. Few candidates means few opens even on otherwise tradeable bars.",
    operatorAction:
      "Check NEXT_PUBLIC_BTC_FT_STRATEGY_IDS on Vercel — clear it to fall back to the CORE roster (44 strategies: 20 winners + 24 premiums).",
    safeToAutomate: false,
    relatedFiles: ["client/src/lib/btcFtRoster.ts"],
  };
}

function checkBlocker(snap: AiAppTrackerSnapshot): DeskHealingAction | null {
  const { dominantBlocker, candidates, opened } = snap.entryFunnel;
  if (dominantBlocker == null || candidates == null || opened == null) return null;
  if (candidates > 0 || opened > 0) return null;

  switch (dominantBlocker.toLowerCase()) {
    case "signal":
    case "confirm":
    case "regime":
    case "spread":
    case "session":
    case "atrfees":
      return {
        type: "WAIT_FOR_MARKET",
        severity: "info",
        title: `Blocker: ${dominantBlocker} — gate is working as designed`,
        reason:
          "Strategy gate rejected every candidate this tick. This is the system protecting capital, not a fault.",
        operatorAction:
          "No action. If you want trades anyway, change the strategy roster — never lower the gate.",
        safeToAutomate: true,
        relatedFiles: ["client/src/lib/deskEntryFunnelSnapshot.ts"],
      };
    case "rotation":
    case "cooldown":
    case "suspended":
      return {
        type: "WAIT_FOR_MARKET",
        severity: "info",
        title: `Blocker: ${dominantBlocker}`,
        reason: "Rotation/cooldown is suppressing strategies to prevent overtrading after losses.",
        operatorAction: "No action. Rotation clears automatically when the cooldown window expires.",
        safeToAutomate: true,
        relatedFiles: ["client/src/lib/futuresStrategyRotation.ts"],
      };
    case "nostrategies":
    case "category":
      return {
        type: "DISABLE_BAD_STRATEGIES",
        severity: "warning",
        title: `Blocker: ${dominantBlocker} — roster filter too tight`,
        reason:
          "The active strategy pool is empty after roster gating. Category filter, winners-only, or env allowlist is excluding everything.",
        operatorAction:
          "Inspect NEXT_PUBLIC_BTC_FT_CATEGORY and NEXT_PUBLIC_BTC_FT_WINNERS_ONLY on Vercel; broaden the filter or expand the winners list.",
        safeToAutomate: false,
        relatedFiles: [
          "client/src/lib/btcFtRoster.ts",
          "client/src/lib/futuresDeskPolicy.ts",
        ],
      };
    case "margin":
    case "maxopen":
    case "sameside":
      return {
        type: "WAIT_FOR_MARKET",
        severity: "info",
        title: `Blocker: ${dominantBlocker}`,
        reason: "Position-management gate: existing positions saturate the slot or margin budget.",
        operatorAction: "No action — gate clears when an existing position closes.",
        safeToAutomate: true,
      };
    default:
      return {
        type: "COLLECT_DATA",
        severity: "info",
        title: `Blocker: ${dominantBlocker} — unmapped`,
        reason: "Blocker label not yet mapped by the self-healing engine.",
        operatorAction:
          "Add this blocker to deskSelfHealing.ts checkBlocker(). Inspect deskEntryFunnelSnapshot.ts for the canonical enum.",
        safeToAutomate: false,
        relatedFiles: ["client/src/lib/deskSelfHealing.ts"],
      };
  }
}

// ─── Main export ─────────────────────────────────────────────────────────────

/**
 * Recommend a prioritized list of healing actions from a tracker snapshot.
 * Sorted danger → warning → info; never empty (returns NO_ACTION when healthy).
 */
export function recommendHealingActions(
  snapshot: AiAppTrackerSnapshot,
): DeskHealingAction[] {
  const rules: Array<(s: AiAppTrackerSnapshot) => DeskHealingAction | null> = [
    checkDeployment,
    checkPaperState,
    checkWorker,
    checkBalanceDrift,
    checkPauseEntries,
    checkRoster,
    checkBlocker,
  ];

  const actions: DeskHealingAction[] = [];
  for (const rule of rules) {
    const action = rule(snapshot);
    if (action) actions.push(action);
  }

  if (actions.length === 0) {
    actions.push({
      type: "NO_ACTION",
      severity: "info",
      title: "Desk healthy",
      reason: "All diagnostics green. Worker live, no drift, candidates flowing.",
      operatorAction:
        "Monitor for the next dominant blocker. Run `npm run ai:summary` if behavior changes.",
      safeToAutomate: true,
    });
  }

  return actions.sort(
    (a, b) => SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity],
  );
}
