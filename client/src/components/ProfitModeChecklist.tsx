"use client";

import { useEffect, useState } from "react";
import { DeskCard, DeskButton } from "@/components/desk/ui";

type ProfitModeChecklistProps = {
  storageNamespace: string;
  /** Number of closed trades recorded so far. */
  totalTrades: number;
};

const CHECKLIST_ITEMS = [
  {
    key: "profit_mode",
    label: "Profit mode is ON",
    detail: "Set NEXT_PUBLIC_BTC_FT_PROFIT_MODE=1 in your .env.local or Vercel dashboard.",
  },
  {
    key: "threshold",
    label: "Signal threshold ≥ 26",
    detail: "Default when profit mode is enabled. Override via NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD.",
  },
  {
    key: "first_trade",
    label: "First trade recorded",
    detail: "Keep this tab open — the engine polls every ~4 s and opens entries when signal qualifies.",
  },
  {
    key: "ten_trades",
    label: "10 closed trades for health check",
    detail: "Rolling health check (grade A/B/C/F) and diagnostics unlock after 10 trades.",
  },
] as const;

/**
 * First-run operator checklist shown in compact/profit mode until 10 trades are recorded.
 * Dismissed via localStorage; does not re-appear after dismissal even if reloaded.
 */
export function ProfitModeChecklist({ storageNamespace, totalTrades }: ProfitModeChecklistProps) {
  const dismissKey = `${storageNamespace}_checklist_dismissed`;
  const [dismissed, setDismissed] = useState<boolean>(() => {
    if (typeof localStorage === "undefined") return false;
    return localStorage.getItem(dismissKey) === "1";
  });

  useEffect(() => {
    if (typeof localStorage === "undefined") return;
    setDismissed(localStorage.getItem(dismissKey) === "1");
  }, [dismissKey]);

  if (dismissed) return null;

  function handleDismiss() {
    if (typeof localStorage !== "undefined") localStorage.setItem(dismissKey, "1");
    setDismissed(true);
  }

  const checks = [
    true,                    // profit_mode — always true when this renders
    true,                    // threshold — always true in profit mode
    totalTrades >= 1,        // first_trade
    totalTrades >= 10,       // ten_trades
  ] as const;

  const allDone = checks.every(Boolean);

  return (
    <DeskCard>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 10 }}>
        <span className="desk-label-md" style={{ fontWeight: 600, fontSize: "0.8rem" }}>
          Operator setup checklist {allDone ? "✓" : `(${checks.filter(Boolean).length}/${checks.length})`}
        </span>
        <DeskButton variant="text" onClick={handleDismiss}>
          Dismiss
        </DeskButton>
      </div>
      <ul style={{ margin: 0, padding: 0, listStyle: "none", display: "flex", flexDirection: "column", gap: 6 }}>
        {CHECKLIST_ITEMS.map((item, i) => (
          <li key={item.key} style={{ display: "flex", gap: 8, alignItems: "flex-start" }}>
            <span
              style={{
                flexShrink: 0,
                width: 16,
                height: 16,
                borderRadius: "50%",
                background: checks[i] ? "var(--desk-success)" : "var(--desk-outline)",
                marginTop: 2,
              }}
              aria-label={checks[i] ? "done" : "pending"}
            />
            <span>
              <span className="desk-label-md" style={{ fontWeight: checks[i] ? 500 : 400 }}>
                {item.label}
              </span>
              {!checks[i] && (
                <span
                  className="desk-label-md"
                  style={{ display: "block", color: "var(--desk-on-surface-variant)", fontSize: "0.72rem" }}
                >
                  {item.detail}
                </span>
              )}
            </span>
          </li>
        ))}
      </ul>
    </DeskCard>
  );
}
