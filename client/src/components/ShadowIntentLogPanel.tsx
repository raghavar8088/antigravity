"use client";

import { useCallback, useState } from "react";
import type { ShadowIntentListItem } from "@/lib/shadowTradeIntentTypes";

const SHADOW_LOG_LIMIT = 20;

export function ShadowIntentLogPanel({
  enabled,
  signedIn,
}: {
  enabled: boolean;
  signedIn: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [intents, setIntents] = useState<ShadowIntentListItem[] | null>(null);

  const load = useCallback(async () => {
    if (!signedIn) return;
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`/api/shadow-trade-intents?limit=${SHADOW_LOG_LIMIT}`, {
        credentials: "include",
        cache: "no-store",
      });
      const body = (await res.json()) as {
        ok?: boolean;
        error?: string;
        intents?: ShadowIntentListItem[];
      };
      if (!res.ok || !body.ok) {
        setError(body.error ?? `HTTP ${res.status}`);
        setIntents(null);
        return;
      }
      setIntents(body.intents ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Load failed");
      setIntents(null);
    } finally {
      setLoading(false);
    }
  }, [signedIn]);

  if (!enabled) return null;

  return (
    <div className="mt-3 border-t border-zinc-100 pt-3">
      <button
        type="button"
        onClick={() => {
          const next = !open;
          setOpen(next);
          if (next && intents === null) void load();
        }}
        className="text-left text-[10px] font-semibold uppercase tracking-wide text-zinc-500 hover:text-zinc-700"
      >
        Shadow log (last {SHADOW_LOG_LIMIT}) {open ? "▾" : "▸"}
      </button>
      {open ? (
        <div className="mt-2">
          {!signedIn ? (
            <p className="text-[10px] text-zinc-500">Sign in to record and view shadow intents.</p>
          ) : (
            <>
              <div className="mb-2 flex gap-2">
                <button
                  type="button"
                  disabled={loading}
                  onClick={() => void load()}
                  className="rounded border border-zinc-200 bg-white px-2 py-0.5 text-[10px] font-medium text-zinc-600 hover:bg-zinc-50 disabled:opacity-50"
                >
                  {loading ? "Loading…" : "Refresh"}
                </button>
                <span className="text-[10px] text-zinc-400">Paper only — no testnet orders</span>
              </div>
              {error ? <p className="text-[10px] text-rose-600">{error}</p> : null}
              {!error && intents?.length === 0 ? (
                <p className="text-[10px] text-zinc-400">No shadow intents yet.</p>
              ) : null}
              {intents && intents.length > 0 ? (
                <ul className="max-h-48 space-y-1 overflow-y-auto text-[10px] font-mono text-zinc-700">
                  {intents.map((row) => (
                    <li key={row.id} className="border-b border-zinc-50 pb-1 last:border-0">
                      <span className="text-zinc-400">{row.createdAt.slice(11, 19)}</span>{" "}
                      <span className="font-semibold text-zinc-800">{row.intentKind}</span>{" "}
                      {row.symbol} {row.side} ${row.notional.toFixed(0)}{" "}
                      {row.intentKind === "close" ? (
                        <>
                          {row.entryPrice.toFixed(0)}→{row.exitPrice?.toFixed(0)} {row.exitReason}
                        </>
                      ) : (
                        <>@ {row.entryPrice.toFixed(0)}</>
                      )}{" "}
                      #{row.strategyId}{" "}
                      {row.wouldPlaceTestnet ? (
                        <span className="text-violet-700">testnet✓</span>
                      ) : (
                        <span className="text-zinc-400">testnet✗</span>
                      )}
                    </li>
                  ))}
                </ul>
              ) : null}
            </>
          )}
        </div>
      ) : null}
    </div>
  );
}
