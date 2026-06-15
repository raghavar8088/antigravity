/**
 * Kill Switch Status
 *
 * GET /api/killswitch/status — proxies engine state and auto-clears known
 * false-positive paper halts (balance equity drift) without requiring engine redeploy.
 */

import { NextResponse } from "next/server";
import {
  inferTriggeredBy,
  isKillSwitchArmedOnServer,
  shouldAutoClearHalt,
} from "@/lib/killswitch/killSwitchPolicy";
import { releaseEngineKillSwitch } from "@/lib/killswitch/releaseEngineKillSwitch";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function GET() {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/admin/ks/status`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
      headers: { "X-Service-Name": "vercel-ks-status" },
    });
    if (!res.ok) {
      return NextResponse.json({ ok: false, error: "Engine returned " + res.status }, { status: res.status });
    }

    const data = await res.json();
    const reason = typeof data.reason === "string" ? data.reason : null;
    const engineEnabled = data.enabled;
    const armed = isKillSwitchArmedOnServer(engineEnabled);
    let active = !!data.active;
    const autoClear = shouldAutoClearHalt(active, reason, engineEnabled);

    if (autoClear) {
      await releaseEngineKillSwitch();
      active = false;
    }

    return NextResponse.json({
      active,
      enabled: armed,
      reason: active ? reason : null,
      triggeredAt: active ? (data.triggeredAt ?? data.blockedAt ?? null) : null,
      triggeredBy: active ? inferTriggeredBy(reason) : null,
      activeStrategies: data.activeStrategies ?? 0,
      openOrders: data.openOrders ?? 0,
      autoCleared: autoClear,
    });
  } catch {
    return NextResponse.json(
      {
        active: false,
        enabled: false,
        triggeredAt: null,
        triggeredBy: null,
        activeStrategies: 0,
        openOrders: 0,
        error: "engine_offline",
      },
      { status: 200 },
    );
  }
}
