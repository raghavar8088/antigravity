"use client";

import { TerminalCard } from "@/components/terminal/institutional/TerminalCard";

export default function SettingsPage() {
  return (
    <TerminalCard title="Command Center Settings" subtitle="Operator preferences · session">
      <div className="space-y-3 text-xs text-zinc-400">
        <p>Terminal authority is read-only. Execution parameters are controlled by the Go engine and environment configuration.</p>
        <p className="font-mono text-[10px] text-zinc-500">
          NEXT_PUBLIC_TERMINAL_WS_URL · INTERNAL_API_URL · MONGODB_URI · AUTH_JWT_SECRET
        </p>
      </div>
    </TerminalCard>
  );
}
