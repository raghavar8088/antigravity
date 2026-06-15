"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export interface KillSwitchStatus {
  active: boolean;
  enabled: boolean;
  reason: string | null;
  triggeredAt: string | null;
  triggeredBy: string | null;
  activeStrategies: number;
  openOrders: number;
  /** "ok" | "engine_offline" | "error" */
  dataState: "ok" | "engine_offline" | "error";
}

const DEFAULT: KillSwitchStatus = {
  active: false,
  enabled: false,
  reason: null,
  triggeredAt: null,
  triggeredBy: null,
  activeStrategies: 0,
  openOrders: 0,
  dataState: "ok",
};

/** Polls /api/killswitch/status every 2 seconds with exponential backoff on error. */
export function useKillSwitch() {
  const [status, setStatus] = useState<KillSwitchStatus>(DEFAULT);
  const [loading, setLoading] = useState(false);
  const backoffRef = useRef(2000);

  const poll = useCallback(async () => {
    try {
      const res = await fetch("/api/killswitch/status", { cache: "no-store" });
      const data = await res.json();
      backoffRef.current = 2000;
      setStatus({
        active: !!data.active,
        enabled: data.enabled === true,
        reason: typeof data.reason === "string" ? data.reason : null,
        triggeredAt: data.triggeredAt ?? data.blockedAt ?? null,
        triggeredBy: data.triggeredBy ?? inferTriggeredBy(data.reason),
        activeStrategies: data.activeStrategies ?? 0,
        openOrders: data.openOrders ?? 0,
        dataState: data.error === "engine_offline" ? "engine_offline" : "ok",
      });
    } catch {
      backoffRef.current = Math.min(backoffRef.current * 2, 30_000);
      setStatus((s) => ({ ...s, dataState: "error" }));
    }
  }, []);

  useEffect(() => {
    poll();
    const id = setInterval(poll, 2000);
    return () => clearInterval(id);
  }, [poll]);

  const trigger = useCallback(async (reason?: string): Promise<boolean> => {
    setLoading(true);
    try {
      const res = await fetch("/api/killswitch/trigger", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reason }),
      });
      const data = await res.json();
      if (res.ok) await poll();
      return !!data.ok;
    } finally {
      setLoading(false);
    }
  }, [poll]);

  const resume = useCallback(async (): Promise<boolean> => {
    setLoading(true);
    try {
      const res = await fetch("/api/killswitch/resume", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });
      const data = await res.json();
      if (res.ok) await poll();
      return !!data.ok;
    } finally {
      setLoading(false);
    }
  }, [poll]);

  return { status, loading, trigger, resume };
}

function inferTriggeredBy(reason: unknown): string | null {
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
