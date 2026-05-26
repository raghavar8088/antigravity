/**
 * Tests for the self-healing recommendation engine.
 * Pure-function tests — no MongoDB / no I/O.
 */
import { describe, it, expect } from "vitest";
import { recommendHealingActions, type DeskHealingActionType } from "./deskSelfHealing";
import type { AiAppTrackerSnapshot } from "./aiAppTracker/types";

// ── helpers ───────────────────────────────────────────────────────────────────

function healthySnap(overrides: Partial<AiAppTrackerSnapshot> = {}): AiAppTrackerSnapshot {
  const base: AiAppTrackerSnapshot = {
    createdAt: new Date().toISOString(),
    appBuildSha: "abc1234",
    module: "btc_future_trading",
    accountKeySuffix: "…test",
    worker: { enabled: true, stale: false, lastPollAt: Date.now(), ageSeconds: 5, owner: "vps" },
    entryFunnel: {
      ageSeconds: 3, dominantBlocker: "signal", recommendation: "All good.",
      activeStrategies: 20, evaluatedStrategies: 20, candidates: 0, opened: 0,
    },
    paperState: {
      balance: 1000, openPositions: 0, workerLastPollAt: Date.now(),
      balanceDriftUsd: 0, pauseEntries: false, clearedAt: null,
    },
    env: { profitMode: false, winnersOnly: false, researchMode: false, workerEnabled: true },
    warnings: [],
  };
  return { ...base, ...overrides };
}

function actionTypes(actions: ReturnType<typeof recommendHealingActions>): DeskHealingActionType[] {
  return actions.map((a) => a.type);
}

// ── No-op path ────────────────────────────────────────────────────────────────

describe("recommendHealingActions — healthy desk", () => {
  it("returns single NO_ACTION when everything is green and candidates > 0", () => {
    const snap = healthySnap({
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "signal", recommendation: "All good.",
        activeStrategies: 20, evaluatedStrategies: 20, candidates: 2, opened: 1,
      },
    });
    const actions = recommendHealingActions(snap);
    expect(actions).toHaveLength(1);
    expect(actions[0].type).toBe("NO_ACTION");
    expect(actions[0].severity).toBe("info");
    expect(actions[0].safeToAutomate).toBe(true);
  });

  it("never returns an empty array", () => {
    const actions = recommendHealingActions(healthySnap());
    expect(actions.length).toBeGreaterThan(0);
  });
});

// ── CHECK_DEPLOYMENT ──────────────────────────────────────────────────────────

describe("recommendHealingActions — deployment issues", () => {
  it("flags missing account key as DANGER", () => {
    const snap = healthySnap({
      accountKeySuffix: null,
      warnings: ["DESK_WORKER_ACCOUNT_KEY is not set — cannot read desk state."],
    });
    const actions = recommendHealingActions(snap);
    const dep = actions.find((a) => a.type === "CHECK_DEPLOYMENT")!;
    expect(dep).toBeDefined();
    expect(dep.severity).toBe("danger");
    expect(dep.safeToAutomate).toBe(false);
    expect(dep.title.toLowerCase()).toContain("account key");
  });

  it("flags missing MongoDB config as DANGER", () => {
    const snap = healthySnap({
      warnings: ["MongoDB is not configured (MONGODB_URI missing or invalid)."],
    });
    const actions = recommendHealingActions(snap);
    const dep = actions.find((a) => a.type === "CHECK_DEPLOYMENT")!;
    expect(dep).toBeDefined();
    expect(dep.severity).toBe("danger");
    expect(dep.title.toLowerCase()).toContain("mongodb");
  });
});

// ── REPAIR_STATE — missing paper_state ────────────────────────────────────────

describe("recommendHealingActions — paper_state missing", () => {
  it("emits REPAIR_STATE with copyable curl when paper_state missing", () => {
    const snap = healthySnap({
      warnings: ["No paper_state found in MongoDB for this account."],
    });
    const actions = recommendHealingActions(snap);
    const repair = actions.find((a) => a.type === "REPAIR_STATE")!;
    expect(repair).toBeDefined();
    expect(repair.severity).toBe("danger");
    expect(repair.copyableCommand).toContain("/api/paper-state/repair");
    expect(repair.copyableCommand).not.toContain("MONGODB_URI");
  });
});

// ── RESTART_WORKER ────────────────────────────────────────────────────────────

describe("recommendHealingActions — worker", () => {
  it("emits RESTART_WORKER (danger) when heartbeat stale", () => {
    const snap = healthySnap({
      worker: { enabled: true, stale: true, lastPollAt: Date.now() - 200_000, ageSeconds: 200, owner: "vps" },
      warnings: ["Worker heartbeat is stale (200s old)."],
    });
    const actions = recommendHealingActions(snap);
    const restart = actions.find((a) => a.type === "RESTART_WORKER")!;
    expect(restart).toBeDefined();
    expect(restart.severity).toBe("danger");
    expect(restart.copyableCommand).toContain("pm2 restart");
  });

  it("emits RESTART_WORKER (warning) on stale funnel even when heartbeat fresh", () => {
    const snap = healthySnap({
      warnings: ["Entry funnel snapshot is stale (180s old)."],
    });
    const actions = recommendHealingActions(snap);
    const restart = actions.find((a) => a.type === "RESTART_WORKER")!;
    expect(restart).toBeDefined();
    expect(restart.severity).toBe("warning");
  });
});

// ── REPAIR_STATE — balance drift ──────────────────────────────────────────────

describe("recommendHealingActions — balance drift", () => {
  it("large drift + 0 open positions → REPAIR_STATE safeToAutomate=true", () => {
    const snap = healthySnap({
      paperState: {
        balance: 850, openPositions: 0,
        workerLastPollAt: Date.now(), balanceDriftUsd: -150,
        pauseEntries: false, clearedAt: null,
      },
      warnings: ["Balance drifted -$150.00 from day start."],
    });
    const actions = recommendHealingActions(snap);
    const repair = actions.find((a) => a.type === "REPAIR_STATE")!;
    expect(repair).toBeDefined();
    expect(repair.severity).toBe("danger");
    expect(repair.safeToAutomate).toBe(true);
  });

  it("large drift + open positions → REPAIR_STATE safeToAutomate=false", () => {
    const snap = healthySnap({
      paperState: {
        balance: 850, openPositions: 2,
        workerLastPollAt: Date.now(), balanceDriftUsd: -150,
        pauseEntries: false, clearedAt: null,
      },
      warnings: ["Balance drifted -$150.00 from day start."],
    });
    const actions = recommendHealingActions(snap);
    const repair = actions.find((a) => a.type === "REPAIR_STATE")!;
    expect(repair.safeToAutomate).toBe(false);
  });

  it("mild drift ($50-$100) → COLLECT_DATA warning, NOT auto-repair", () => {
    const snap = healthySnap({
      paperState: {
        balance: 925, openPositions: 0,
        workerLastPollAt: Date.now(), balanceDriftUsd: -75,
        pauseEntries: false, clearedAt: null,
      },
      warnings: ["Balance drifted -$75.00 from day start."],
    });
    const actions = recommendHealingActions(snap);
    const types = actionTypes(actions);
    expect(types).toContain("COLLECT_DATA");
    expect(types).not.toContain("REPAIR_STATE");
  });

  it("small drift (< $50) → no drift action", () => {
    const snap = healthySnap({
      paperState: {
        balance: 1010, openPositions: 0,
        workerLastPollAt: Date.now(), balanceDriftUsd: 10,
        pauseEntries: false, clearedAt: null,
      },
    });
    const types = actionTypes(recommendHealingActions(snap));
    expect(types).not.toContain("REPAIR_STATE");
    expect(types).not.toContain("COLLECT_DATA");
  });
});

// ── REPAIR_STATE — pauseEntries ───────────────────────────────────────────────

describe("recommendHealingActions — pauseEntries", () => {
  it("emits REPAIR_STATE warning when pauseEntries=true", () => {
    const snap = healthySnap({
      paperState: {
        balance: 1000, openPositions: 0,
        workerLastPollAt: Date.now(), balanceDriftUsd: 0,
        pauseEntries: true, clearedAt: null,
      },
    });
    const actions = recommendHealingActions(snap);
    const repair = actions.find((a) => a.type === "REPAIR_STATE")!;
    expect(repair).toBeDefined();
    expect(repair.severity).toBe("warning");
    expect(repair.safeToAutomate).toBe(false);
  });
});

// ── REDUCE_ROSTER ─────────────────────────────────────────────────────────────

describe("recommendHealingActions — roster", () => {
  it("emits REDUCE_ROSTER when activeStrategies < 4", () => {
    const snap = healthySnap({
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "signal", recommendation: "",
        activeStrategies: 2, evaluatedStrategies: 2, candidates: 0, opened: 0,
      },
    });
    const actions = recommendHealingActions(snap);
    const reduce = actions.find((a) => a.type === "REDUCE_ROSTER")!;
    expect(reduce).toBeDefined();
    expect(reduce.severity).toBe("warning");
  });

  it("does NOT emit REDUCE_ROSTER when activeStrategies >= 4", () => {
    const snap = healthySnap({
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "signal", recommendation: "",
        activeStrategies: 10, evaluatedStrategies: 10, candidates: 0, opened: 0,
      },
    });
    expect(actionTypes(recommendHealingActions(snap))).not.toContain("REDUCE_ROSTER");
  });
});

// ── Blocker mapping ───────────────────────────────────────────────────────────

describe("recommendHealingActions — blocker mapping", () => {
  it("signal blocker → WAIT_FOR_MARKET info (gate working)", () => {
    const snap = healthySnap({
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "signal", recommendation: "",
        activeStrategies: 20, evaluatedStrategies: 20, candidates: 0, opened: 0,
      },
    });
    const wait = recommendHealingActions(snap).find((a) => a.type === "WAIT_FOR_MARKET")!;
    expect(wait).toBeDefined();
    expect(wait.severity).toBe("info");
    expect(wait.safeToAutomate).toBe(true);
  });

  it("rotation blocker → WAIT_FOR_MARKET (cooldown)", () => {
    const snap = healthySnap({
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "rotation", recommendation: "",
        activeStrategies: 20, evaluatedStrategies: 20, candidates: 0, opened: 0,
      },
    });
    const types = actionTypes(recommendHealingActions(snap));
    expect(types).toContain("WAIT_FOR_MARKET");
  });

  it("noStrategies blocker → DISABLE_BAD_STRATEGIES warning (roster too tight)", () => {
    const snap = healthySnap({
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "noStrategies", recommendation: "",
        activeStrategies: 0, evaluatedStrategies: 0, candidates: 0, opened: 0,
      },
    });
    const action = recommendHealingActions(snap).find(
      (a) => a.type === "DISABLE_BAD_STRATEGIES",
    )!;
    expect(action).toBeDefined();
    expect(action.severity).toBe("warning");
  });

  it("category blocker → DISABLE_BAD_STRATEGIES", () => {
    const snap = healthySnap({
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "category", recommendation: "",
        activeStrategies: 8, evaluatedStrategies: 8, candidates: 0, opened: 0,
      },
    });
    expect(actionTypes(recommendHealingActions(snap))).toContain("DISABLE_BAD_STRATEGIES");
  });

  it("unknown blocker → COLLECT_DATA fallback", () => {
    const snap = healthySnap({
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "newBlockerNotYetMapped", recommendation: "",
        activeStrategies: 20, evaluatedStrategies: 20, candidates: 0, opened: 0,
      },
    });
    expect(actionTypes(recommendHealingActions(snap))).toContain("COLLECT_DATA");
  });

  it("blocker emits NOTHING when opened > 0 (trades flowing)", () => {
    const snap = healthySnap({
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "signal", recommendation: "",
        activeStrategies: 20, evaluatedStrategies: 20, candidates: 2, opened: 1,
      },
    });
    const types = actionTypes(recommendHealingActions(snap));
    expect(types).not.toContain("WAIT_FOR_MARKET");
    expect(types).toEqual(["NO_ACTION"]);
  });
});

// ── Severity ordering ─────────────────────────────────────────────────────────

describe("recommendHealingActions — severity ordering", () => {
  it("sorts danger before warning before info when multiple issues exist", () => {
    const snap = healthySnap({
      worker: { enabled: true, stale: true, lastPollAt: Date.now() - 200_000, ageSeconds: 200, owner: "vps" },
      paperState: {
        balance: 1000, openPositions: 0, workerLastPollAt: Date.now() - 200_000,
        balanceDriftUsd: 0, pauseEntries: true, clearedAt: null,
      },
      entryFunnel: {
        ageSeconds: 3, dominantBlocker: "signal", recommendation: "",
        activeStrategies: 2, evaluatedStrategies: 2, candidates: 0, opened: 0,
      },
      warnings: [
        "Worker heartbeat is stale (200s old).",
        "Entries are paused (pauseEntries=true).",
      ],
    });
    const actions = recommendHealingActions(snap);
    for (let i = 1; i < actions.length; i++) {
      const prev = actions[i - 1].severity;
      const cur = actions[i].severity;
      const order = { danger: 0, warning: 1, info: 2 } as const;
      expect(order[cur]).toBeGreaterThanOrEqual(order[prev]);
    }
  });
});

// ── Hard invariants ───────────────────────────────────────────────────────────

describe("recommendHealingActions — hard invariants", () => {
  it("never recommends lowering thresholds in any action text", () => {
    const samples: AiAppTrackerSnapshot[] = [
      healthySnap(),
      healthySnap({ warnings: ["Worker heartbeat is stale (200s)."] }),
      healthySnap({
        paperState: {
          balance: 800, openPositions: 0, workerLastPollAt: Date.now(),
          balanceDriftUsd: -200, pauseEntries: false, clearedAt: null,
        },
        warnings: ["Balance drifted -$200.00 from day start."],
      }),
      healthySnap({
        entryFunnel: {
          ageSeconds: 3, dominantBlocker: "signal", recommendation: "",
          activeStrategies: 20, evaluatedStrategies: 20, candidates: 0, opened: 0,
        },
      }),
    ];
    for (const snap of samples) {
      const actions = recommendHealingActions(snap);
      const all = actions.map((a) => `${a.title} ${a.reason} ${a.operatorAction}`).join(" ").toLowerCase();
      expect(all).not.toContain("lower threshold");
      expect(all).not.toContain("lower the threshold");
      expect(all).not.toContain("reduce threshold");
      expect(all).not.toContain("decrease threshold");
      expect(all).not.toContain("signal threshold to");
      expect(all).not.toContain("bypass gate");
      expect(all).not.toContain("disable gate");
    }
  });

  it("never embeds raw secrets in copyableCommand", () => {
    const snap = healthySnap({
      warnings: ["No paper_state found in MongoDB for this account."],
    });
    const actions = recommendHealingActions(snap);
    const cmds = actions.map((a) => a.copyableCommand ?? "").join(" ");
    expect(cmds).not.toContain("mongodb+srv://");
    expect(cmds).not.toContain("mongodb://");
    // Account key references must use $-style env var indirection, not literal keys
    expect(cmds).not.toMatch(/"accountKey"\s*:\s*"[A-Za-z0-9-_]{8,}/);
  });

  it("every action has non-empty title, reason, operatorAction", () => {
    const samples = [
      healthySnap(),
      healthySnap({ accountKeySuffix: null }),
      healthySnap({ worker: { enabled: true, stale: true, lastPollAt: null, ageSeconds: null, owner: null } }),
    ];
    for (const snap of samples) {
      for (const action of recommendHealingActions(snap)) {
        expect(action.title.length).toBeGreaterThan(0);
        expect(action.reason.length).toBeGreaterThan(0);
        expect(action.operatorAction.length).toBeGreaterThan(0);
      }
    }
  });
});
