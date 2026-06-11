"use client";

import type { ReactNode } from "react";
import type { TerminalAuthorityState } from "@/lib/terminal/terminalStore";
import { terminalAuthorityLabel, terminalHasAuthority } from "@/lib/terminal/terminalAuthority";

type Props = {
  snapshot: TerminalAuthorityState;
  children: ReactNode;
};

export function TerminalAuthorityGuard({ snapshot, children }: Props) {
  if (snapshot.loading && !snapshot.hasAuthority) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center gap-3 font-mono text-sm text-zinc-500">
        <div className="h-5 w-5 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-400" />
        <p>LOADING</p>
        <p className="text-xs text-zinc-600">Connecting to backend authority...</p>
      </div>
    );
  }

  if (snapshot.restUnavailable || !terminalHasAuthority(snapshot)) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center gap-3 font-mono text-sm">
        <p className="text-rose-400 text-lg font-bold">{terminalAuthorityLabel(snapshot)}</p>
        <p className="text-zinc-500 text-xs">WebSocket disconnected · REST polling failed</p>
        <p className="text-zinc-600 text-xs">NO DATA AVAILABLE — retrying every 30s after circuit break</p>
      </div>
    );
  }

  return <>{children}</>;
}

export function TerminalNoData({ label = "NO DATA AVAILABLE" }: { label?: string }) {
  return (
    <div className="rounded-lg border border-dashed border-zinc-800 bg-zinc-950/40 px-4 py-8 text-center font-mono text-xs text-zinc-500">
      {label}
    </div>
  );
}
