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
      <div className="google-empty-state min-h-[60vh]">
        <div className="mx-auto mb-3 h-8 w-8 animate-spin rounded-full border-2 border-blue-100 border-t-blue-500" />
        <p className="m3-empty-state__title">Loading Trade Engine data</p>
        <p className="m3-empty-state__subtitle">Connecting to the backend authority.</p>
      </div>
    );
  }

  const degraded = snapshot.restUnavailable || !terminalHasAuthority(snapshot);

  return (
    <>
      {degraded && !dismissed && (
        <div className="m3-banner m3-banner--error mb-3 flex items-center justify-between gap-3 px-4 py-3 text-sm" role="status">
          <span>
            <span className="font-bold">Trade Engine data is updating</span>
            {" — "}
            {snapshot.restUnavailable
              ? "REST circuit is retrying every 30s"
              : "Live stream is reconnecting while polling continues."}
          </span>
          <button
            type="button"
            onClick={() => setDismissed(true)}
            className="text-rose-600 transition-colors hover:text-rose-700"
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

function humanizeEmptyLabel(label: string) {
  const cleaned = label
    .replace(/\bNO\b/g, "No")
    .replace(/\bDATA\b/g, "data")
    .replace(/\bAVAILABLE\b/g, "available")
    .replace(/\bLOADING\.\.\./g, "Loading")
    .replace(/\bMOCK[-_ ]TRADING\b/gi, "Trade Engine")
    .replace(/\bPAPER DESK\b/gi, "Trade Engine")
    .replace(/\s+—\s+/g, " - ")
    .trim();
  return cleaned.charAt(0).toUpperCase() + cleaned.slice(1).toLowerCase().replace(/\bbtc\b/g, "BTC").replace(/\bws\b/g, "WS");
}

export function TerminalNoData({ label = "No data available" }: { label?: string }) {
  return (
    <div className="google-empty-state google-empty-state-inline" role="status">
      <p className="m3-empty-state__title">{humanizeEmptyLabel(label)}</p>
      <p className="m3-empty-state__subtitle">This section will update automatically when Trade Engine data is available.</p>
    </div>
  );
}
