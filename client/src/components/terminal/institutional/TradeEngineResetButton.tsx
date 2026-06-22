"use client";

import { useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { NavIcon } from "./NavIcons";
import { TERMINAL_ROUTES } from "@/lib/utils/navRoutes";

/**
 * Header control that resets the Trade Engine paper account — clears all
 * persisted trades, snapshots, analytics, and logs for the owner account so
 * the desk starts fresh at the configured starting balance. Only rendered on
 * the Trade Engine route; authorized server-side by the signed owner session.
 */
export function TradeEngineResetButton() {
  const pathname = usePathname();
  const router = useRouter();
  const [loading, setLoading] = useState(false);

  if (pathname !== TERMINAL_ROUTES["trade-engine"]) return null;

  async function onReset() {
    if (loading) return;
    const confirmed =
      typeof window === "undefined" ||
      window.confirm(
        "Reset the Trade Engine? This permanently deletes all persisted trades, logs, analytics, and account snapshots, and restarts the desk from the configured starting balance.",
      );
    if (!confirmed) return;

    setLoading(true);
    try {
      const res = await fetch("/api/mock-trading/reset", { method: "POST" });
      if (!res.ok) {
        const json = (await res.json().catch(() => null)) as { error?: string } | null;
        if (typeof window !== "undefined") {
          window.alert(`Reset failed: ${json?.error ?? `HTTP ${res.status}`}`);
        }
        return;
      }
      router.refresh();
      if (typeof window !== "undefined") window.location.reload();
    } catch (err) {
      if (typeof window !== "undefined") {
        window.alert(`Reset failed: ${(err as Error).message}`);
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <button
      type="button"
      className="m3-icon-btn"
      onClick={onReset}
      disabled={loading}
      aria-label="Reset Trade Engine"
      title="Reset Trade Engine"
    >
      <NavIcon name="reset" />
    </button>
  );
}
