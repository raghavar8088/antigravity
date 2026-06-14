"use client";

import { useState, type ReactNode } from "react";
import type { TerminalAuthorityState } from "@/lib/terminal/terminalStore";
import { terminalHasAuthority } from "@/lib/terminal/terminalAuthority";

type Props = {
  snapshot: TerminalAuthorityState;
  /**
   * When true (default) the guard blocks with a full-page spinner during the
   * very first load before any data arrives. Pages that own their own data
   * sources (non-authoritative review layers) should pass authorityRequired={false} so
   * they render immediately — the degraded banner still shows when the paper
   * desk is unreachable.
   */
  authorityRequired?: boolean;
  children: ReactNode;
};

export function TerminalAuthorityGuard({
  snapshot,
  authorityRequired = true,
  children,
}: Props) {
  const [dismissed, setDismissed] = useState(false);

  if (authorityRequired && snapshot.loading && !snapshot.hasAuthority) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center gap-3 font-mono text-sm text-zinc-500">
        <div className="h-5 w-5 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-400" />
        <p>LOADING</p>
        <p className="text-xs text-zinc-600">Connecting to backend authority...</p>
      </div>
    );
  }

  const degraded = snapshot.restUnavailable || !terminalHasAuthority(snapshot);

  return (
    <>
      {degraded && !dismissed && (
        <div className="flex items-center justify-between gap-3 rounded-md border border-rose-900/60 bg-rose-950/30 px-4 py-2 font-mono text-xs text-rose-400 mb-3">
          <span>
            <span className="font-bold">PAPER DESK OFFLINE</span>
            {" — "}
            {snapshot.restUnavailable
              ? "REST circuit open · retrying every 30 s"
              : "WebSocket disconnected · polling…"}
          </span>
          <button
            type="button"
            onClick={() => setDismissed(true)}
            className="text-rose-600 hover:text-rose-300 transition-colors"
            aria-label="Dismiss"
          >
            ✕
          </button>
        </div>
      )}
      {children}
    </>
  );
}

export function TerminalNoData({ label = "NO DATA AVAILABLE" }: { label?: string }) {
  return (
    <div className="rounded-lg border border-dashed border-zinc-800 bg-zinc-950/40 px-4 py-8 text-center font-mono text-xs text-zinc-500">
      {label}
    </div>
  );
}
