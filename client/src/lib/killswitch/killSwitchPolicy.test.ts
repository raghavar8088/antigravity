import { describe, expect, it } from "vitest";
import {
  isBalanceDriftHalt,
  isKillSwitchArmedOnServer,
  shouldAutoClearHalt,
} from "@/lib/killswitch/killSwitchPolicy";

describe("killSwitchPolicy", () => {
  it("treats balance equity drift as auto-clearable", () => {
    const reason = "reconciliation critical drift (balance): balance equity_drift — equity drift 2%";
    expect(isBalanceDriftHalt(reason)).toBe(true);
    expect(shouldAutoClearHalt(true, reason, undefined)).toBe(true);
  });

  it("defaults to disarmed when engine omits enabled flag", () => {
    expect(isKillSwitchArmedOnServer(undefined)).toBe(false);
    expect(isKillSwitchArmedOnServer(false)).toBe(false);
    expect(isKillSwitchArmedOnServer(true)).toBe(true);
  });

  it("does not auto-clear legitimate operator halts when armed", () => {
    const prev = process.env.KILL_SWITCH_ENABLED;
    process.env.KILL_SWITCH_ENABLED = "true";
    const reason = "manual operator block via /api/admin/ks/block";
    expect(shouldAutoClearHalt(true, reason, true)).toBe(false);
    process.env.KILL_SWITCH_ENABLED = prev;
  });
});
