/**
 * Kill switch policy for Vercel ↔ engine proxy.
 * Paper desk: disabled by default unless KILL_SWITCH_ENABLED=true.
 */

export function isKillSwitchArmedOnServer(engineEnabled?: unknown): boolean {
  const vercelFlag = process.env.KILL_SWITCH_ENABLED?.trim().toLowerCase();
  if (vercelFlag === "true" || vercelFlag === "1" || vercelFlag === "yes") {
    return true;
  }
  if (vercelFlag === "false" || vercelFlag === "0" || vercelFlag === "no") {
    return false;
  }
  // New engine exposes enabled; old engine omits it — treat omitted as disabled for paper.
  return engineEnabled === true;
}

/** Known false-positive halt from paper balance reconciliation lag. */
export function isBalanceDriftHalt(reason: unknown): boolean {
  if (typeof reason !== "string" || reason.trim() === "") return false;
  const r = reason.toLowerCase();
  if (!r.includes("reconciliation") && !r.includes("drift")) return false;
  return (
    r.includes("equity_drift") ||
    r.includes("equity drift") ||
    r.includes("available_margin_drift") ||
    r.includes("margin_used_drift") ||
    (r.includes("(balance)") && r.includes("balance"))
  );
}

export function shouldAutoClearHalt(active: boolean, reason: unknown, engineEnabled?: unknown): boolean {
  if (!active) return false;
  if (!isKillSwitchArmedOnServer(engineEnabled)) return true;
  return isBalanceDriftHalt(reason);
}

export function inferTriggeredBy(reason: unknown): string | null {
  if (typeof reason !== "string" || reason.trim() === "") return null;
  const r = reason.toLowerCase();
  if (r.includes("reconciliation") || r.includes("oms_desync") || r.includes("drift")) {
    return "reconciliation";
  }
  if (r.includes("manual operator") || r.includes("operator block") || r.includes("/api/admin/ks/block")) {
    return "operator";
  }
  return "engine";
}
