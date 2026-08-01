"use client";

/**
 * DeskAdminControls — shared reset / clear-trades panel for the paper desks
 * (Crypto Scalp, Crypto Options Buying, Crypto Options Selling).
 *
 * Two deliberately different actions, matching the engine's own split:
 *   - Clear trades: forgets the trade record and the statistics derived from it,
 *     leaving open positions and the current balance alone.
 *   - Reset account: restarts the desk from a chosen starting capital.
 *
 * Both are destructive and irreversible, so each requires a second click to
 * confirm rather than firing on the first press. Every desk wired to this is
 * paper-only — no order routing and no real money behind these endpoints.
 */

import { useState } from "react";
import { DeskButton, DeskCard, DeskSectionHeader } from "@/components/desk/ui";

type Props = {
  /** POST endpoint that resets the account, e.g. "/api/options-buying/reset". */
  resetPath: string;
  /** POST endpoint that clears trade history. */
  clearPath: string;
  /** What the capital field means on this desk. */
  capitalLabel?: string;
  /** Placeholder shown when the field is empty (the desk's current default). */
  capitalPlaceholder?: string;
  /** Called after a successful action so the page can refetch. */
  onDone?: () => void;
};

type Pending = "reset" | "clear" | null;

export function DeskAdminControls({
  resetPath,
  clearPath,
  capitalLabel = "Starting capital (USD)",
  capitalPlaceholder = "engine default",
  onDone,
}: Props) {
  const [capital, setCapital] = useState("");
  const [confirming, setConfirming] = useState<Pending>(null);
  const [busy, setBusy] = useState<Pending>(null);
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);

  const parsedCapital = (() => {
    const n = Number(capital.trim());
    return capital.trim() !== "" && Number.isFinite(n) && n > 0 ? n : null;
  })();
  const capitalInvalid = capital.trim() !== "" && parsedCapital === null;

  async function run(kind: Exclude<Pending, null>) {
    setBusy(kind);
    setMessage(null);
    try {
      const path = kind === "reset" ? resetPath : clearPath;
      // Capital only applies to a reset; clearing history never re-bases the account.
      const body =
        kind === "reset" && parsedCapital !== null
          ? JSON.stringify({ initialCapital: parsedCapital })
          : "{}";
      const res = await fetch(path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
        cache: "no-store",
      });
      const text = await res.text();
      if (!res.ok) {
        setMessage({ ok: false, text: `Failed (${res.status}): ${text.slice(0, 180)}` });
        return;
      }
      setMessage({
        ok: true,
        text:
          kind === "reset"
            ? `Account reset${parsedCapital !== null ? ` to $${parsedCapital.toLocaleString()}` : ""}.`
            : "Trade history cleared.",
      });
      if (kind === "reset") setCapital("");
      onDone?.();
    } catch (e) {
      setMessage({ ok: false, text: e instanceof Error ? e.message : "Request failed" });
    } finally {
      setBusy(null);
      setConfirming(null);
    }
  }

  function press(kind: Exclude<Pending, null>) {
    if (confirming === kind) {
      void run(kind);
      return;
    }
    setMessage(null);
    setConfirming(kind);
  }

  return (
    <DeskCard variant="outlined" padding="lg">
      <DeskSectionHeader title="Desk Management" />
      <p
        className="desk-label-md"
        style={{ color: "var(--desk-on-surface-variant)", margin: "0 0 16px" }}
      >
        Paper desk only. Clearing history forgets past trades and their statistics;
        resetting restarts the account from a chosen starting capital.
      </p>

      <div style={{ display: "flex", flexWrap: "wrap", gap: 16, alignItems: "flex-end" }}>
        <label style={{ display: "flex", flexDirection: "column", gap: 6, minWidth: 220 }}>
          <span className="desk-label-md" style={{ color: "var(--desk-on-surface-variant)" }}>
            {capitalLabel}
          </span>
          <input
            type="number"
            min="1"
            step="any"
            inputMode="decimal"
            value={capital}
            onChange={(e) => {
              setCapital(e.target.value);
              setConfirming(null);
            }}
            placeholder={capitalPlaceholder}
            className="desk-mono"
            style={{
              padding: "10px 12px",
              borderRadius: 8,
              border: `1px solid ${capitalInvalid ? "var(--desk-error)" : "var(--desk-outline)"}`,
              background: "var(--desk-surface)",
              color: "var(--desk-on-surface)",
              fontSize: 14,
            }}
          />
          <span className="desk-label-sm" style={{ color: "var(--desk-on-surface-variant)" }}>
            {capitalInvalid
              ? "Enter a positive number."
              : "Applies to Reset only. Leave blank to keep the engine default."}
          </span>
        </label>

        <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
          <DeskButton
            variant="outlined"
            disabled={busy !== null}
            onClick={() => press("clear")}
          >
            {busy === "clear"
              ? "Clearing…"
              : confirming === "clear"
                ? "Click again to confirm"
                : "Clear trades"}
          </DeskButton>
          <DeskButton
            variant="filled"
            disabled={busy !== null || capitalInvalid}
            onClick={() => press("reset")}
          >
            {busy === "reset"
              ? "Resetting…"
              : confirming === "reset"
                ? "Click again to confirm"
                : "Reset account"}
          </DeskButton>
        </div>
      </div>

      {confirming !== null && busy === null && (
        <p
          className="desk-label-md"
          style={{ color: "var(--desk-error)", margin: "14px 0 0" }}
        >
          {confirming === "reset"
            ? "This wipes all trades, statistics and open positions, then restarts the account. It cannot be undone."
            : "This wipes the trade record and the statistics derived from it. It cannot be undone."}{" "}
          <button
            type="button"
            onClick={() => setConfirming(null)}
            style={{
              background: "none",
              border: "none",
              padding: 0,
              color: "var(--desk-primary)",
              cursor: "pointer",
              font: "inherit",
              textDecoration: "underline",
            }}
          >
            Cancel
          </button>
        </p>
      )}

      {message && (
        <p
          className="desk-label-md"
          style={{
            color: message.ok ? "var(--desk-success, var(--desk-primary))" : "var(--desk-error)",
            margin: "14px 0 0",
          }}
        >
          {message.text}
        </p>
      )}
    </DeskCard>
  );
}

export default DeskAdminControls;
