/**
 * Tests for the self-healing executor.
 *
 * Mocks: none — the executor takes injected deps so tests pass in spies.
 *
 * Coverage:
 *   - Env flag off → every action skipped_disabled
 *   - REPAIR_STATE safe + executed → calls repair + writeWorkerEvent
 *   - REPAIR_STATE unsafe → skipped_unsafe
 *   - Unsupported types (RESTART_WORKER, REDUCE_ROSTER, ...) → skipped_unsupported
 *   - WAIT_FOR_MARKET / NO_ACTION → skipped_no_op (no exec, no event)
 *   - repairPaperState throws → status=failed, never re-throws
 *   - writeWorkerEvent throws → still returns executed (event is non-fatal)
 */
import { describe, it, expect, vi } from "vitest";
import {
  executeHealingActions,
  type HealingExecutionDeps,
} from "./deskSelfHealingExecutor";
import type { DeskHealingAction } from "./deskSelfHealing";

// ── helpers ───────────────────────────────────────────────────────────────────

function makeDeps(overrides: Partial<HealingExecutionDeps> = {}): {
  deps: HealingExecutionDeps;
  repairSpy: ReturnType<typeof vi.fn>;
  eventSpy: ReturnType<typeof vi.fn>;
} {
  const repairSpy = vi.fn().mockResolvedValue({
    repairId: "repair-abc",
    balance: 1000,
    clearedAt: 1700000000000,
  });
  const eventSpy = vi.fn().mockResolvedValue(undefined);

  const base: HealingExecutionDeps = {
    accountKey: "test-account",
    autoEnabled: true,
    repairPaperState: repairSpy,
    writeWorkerEvent: eventSpy,
    now: () => 1700000000000,
  };
  return { deps: { ...base, ...overrides }, repairSpy, eventSpy };
}

function action(
  type: DeskHealingAction["type"],
  safeToAutomate: boolean,
  overrides: Partial<DeskHealingAction> = {},
): DeskHealingAction {
  return {
    type,
    severity: "info",
    title: `${type} test`,
    reason: "test reason",
    operatorAction: "test action",
    safeToAutomate,
    ...overrides,
  };
}

// ── Env flag off ──────────────────────────────────────────────────────────────

describe("executeHealingActions — auto disabled", () => {
  it("records every action as skipped_disabled when autoEnabled=false", async () => {
    const { deps, repairSpy, eventSpy } = makeDeps({ autoEnabled: false });
    const actions = [action("REPAIR_STATE", true), action("WAIT_FOR_MARKET", true)];
    const results = await executeHealingActions(actions, deps);
    expect(results).toHaveLength(2);
    expect(results.every((r) => r.status === "skipped_disabled")).toBe(true);
    expect(repairSpy).not.toHaveBeenCalled();
    expect(eventSpy).not.toHaveBeenCalled();
  });
});

// ── REPAIR_STATE ──────────────────────────────────────────────────────────────

describe("executeHealingActions — REPAIR_STATE", () => {
  it("executes safe REPAIR_STATE and writes audit event", async () => {
    const { deps, repairSpy, eventSpy } = makeDeps();
    const results = await executeHealingActions(
      [action("REPAIR_STATE", true, { title: "Drift repair" })],
      deps,
    );
    expect(results).toHaveLength(1);
    expect(results[0].status).toBe("executed");
    expect(results[0].detail).toEqual({
      repairId: "repair-abc",
      balance: 1000,
      clearedAt: 1700000000000,
    });
    expect(repairSpy).toHaveBeenCalledOnce();
    expect(repairSpy).toHaveBeenCalledWith({ reason: "auto-heal: Drift repair" });
    expect(eventSpy).toHaveBeenCalledOnce();
    expect(eventSpy.mock.calls[0][0]).toMatchObject({
      type: "auto_heal_repair_state",
      severity: "info",
    });
  });

  it("skips unsafe REPAIR_STATE (safeToAutomate=false)", async () => {
    const { deps, repairSpy, eventSpy } = makeDeps();
    const results = await executeHealingActions(
      [action("REPAIR_STATE", false)],
      deps,
    );
    expect(results[0].status).toBe("skipped_unsafe");
    expect(repairSpy).not.toHaveBeenCalled();
    expect(eventSpy).not.toHaveBeenCalled();
  });
});

// ── Unsupported types ─────────────────────────────────────────────────────────

describe("executeHealingActions — unsupported types", () => {
  it.each([
    "RESTART_WORKER",
    "REDUCE_ROSTER",
    "DISABLE_BAD_STRATEGIES",
    "COLLECT_DATA",
    "CHECK_DEPLOYMENT",
  ] as const)("never executes %s even when safeToAutomate=true", async (type) => {
    const { deps, repairSpy, eventSpy } = makeDeps();
    const results = await executeHealingActions([action(type, true)], deps);
    expect(results[0].status).toBe("skipped_unsupported");
    expect(repairSpy).not.toHaveBeenCalled();
    expect(eventSpy).not.toHaveBeenCalled();
  });
});

// ── No-op family ──────────────────────────────────────────────────────────────

describe("executeHealingActions — no-op family", () => {
  it("WAIT_FOR_MARKET → skipped_no_op, no side effects", async () => {
    const { deps, repairSpy, eventSpy } = makeDeps();
    const results = await executeHealingActions([action("WAIT_FOR_MARKET", true)], deps);
    expect(results[0].status).toBe("skipped_no_op");
    expect(repairSpy).not.toHaveBeenCalled();
    expect(eventSpy).not.toHaveBeenCalled();
  });

  it("NO_ACTION → skipped_no_op, no side effects", async () => {
    const { deps, repairSpy, eventSpy } = makeDeps();
    const results = await executeHealingActions([action("NO_ACTION", true)], deps);
    expect(results[0].status).toBe("skipped_no_op");
    expect(repairSpy).not.toHaveBeenCalled();
    expect(eventSpy).not.toHaveBeenCalled();
  });
});

// ── Failure handling ──────────────────────────────────────────────────────────

describe("executeHealingActions — failure handling", () => {
  it("repairPaperState throws → status=failed, event still emitted, never re-throws", async () => {
    const { deps, eventSpy } = makeDeps({
      repairPaperState: vi.fn().mockRejectedValue(new Error("mongo down")),
    });
    const results = await executeHealingActions([action("REPAIR_STATE", true)], deps);
    expect(results[0].status).toBe("failed");
    expect(results[0].reason).toContain("mongo down");
    expect(eventSpy).toHaveBeenCalledOnce();
    expect(eventSpy.mock.calls[0][0].type).toBe("auto_heal_failed");
  });

  it("writeWorkerEvent throws → executor still returns executed (event is non-fatal)", async () => {
    const { deps } = makeDeps({
      writeWorkerEvent: vi.fn().mockRejectedValue(new Error("event write failed")),
    });
    const results = await executeHealingActions([action("REPAIR_STATE", true)], deps);
    expect(results[0].status).toBe("executed");
  });
});

// ── Multiple actions ──────────────────────────────────────────────────────────

describe("executeHealingActions — mixed batch", () => {
  it("returns one result per action, in input order", async () => {
    const { deps } = makeDeps();
    const actions = [
      action("REPAIR_STATE", true, { title: "repair-1" }),
      action("RESTART_WORKER", false),
      action("WAIT_FOR_MARKET", true),
      action("REPAIR_STATE", false),
    ];
    const results = await executeHealingActions(actions, deps);
    expect(results.map((r) => r.status)).toEqual([
      "executed",
      "skipped_unsupported",
      "skipped_no_op",
      "skipped_unsafe",
    ]);
  });
});

// ── Hard invariants ───────────────────────────────────────────────────────────

describe("executeHealingActions — hard invariants", () => {
  it("never throws — even when every dep throws", async () => {
    const deps: HealingExecutionDeps = {
      accountKey: "test",
      autoEnabled: true,
      repairPaperState: vi.fn().mockRejectedValue(new Error("a")),
      writeWorkerEvent: vi.fn().mockRejectedValue(new Error("b")),
      now: () => 0,
    };
    await expect(
      executeHealingActions([action("REPAIR_STATE", true)], deps),
    ).resolves.toBeDefined();
  });

  it("never executes anything that is not safeToAutomate", async () => {
    const { deps, repairSpy } = makeDeps();
    const allActions: DeskHealingAction[] = [
      action("REPAIR_STATE", false),
      action("RESTART_WORKER", false),
      action("REDUCE_ROSTER", false),
      action("DISABLE_BAD_STRATEGIES", false),
      action("COLLECT_DATA", false),
      action("CHECK_DEPLOYMENT", false),
    ];
    await executeHealingActions(allActions, deps);
    expect(repairSpy).not.toHaveBeenCalled();
  });

  it("populates durationMs for executed actions", async () => {
    let t = 1000;
    const { deps } = makeDeps({ now: () => (t += 50) });
    const results = await executeHealingActions([action("REPAIR_STATE", true)], deps);
    expect(results[0].durationMs).toBeGreaterThan(0);
  });
});
