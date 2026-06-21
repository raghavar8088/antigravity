"use client";

/**
 * Master Strictness Dial — one 0-100 control that proportionally scales every
 * signal-quality-relevant threshold at once (engine/internal/config/strictness.go
 * is the source of truth for the LOOSE/CURRENT/STRICT values and the
 * interpolation formula; this component mirrors that formula client-side
 * purely for the live preview, and never applies anything without an
 * explicit confirmation, same safety bar as individual threshold saves).
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { TerminalCard } from "./terminal/institutional/TerminalCard";
import type { StrictnessProfile, ThresholdConfigDTO, ThresholdDelta } from "@/lib/thresholdConfig/types";
import { formatThresholdValue, interpolateStrictnessValue } from "@/lib/thresholdConfig/types";

type DialStatus = "idle" | "applying" | "applied" | "error";

function labelFor(key: string, thresholds: ThresholdConfigDTO[]): ThresholdConfigDTO | undefined {
  return thresholds.find((t) => t.key === key);
}

function formatProfileValue(key: string, value: number, thresholds: ThresholdConfigDTO[]): string {
  const t = labelFor(key, thresholds);
  if (!t) return String(value);
  return formatThresholdValue(t, value);
}

function dialZoneLabel(d: number): string {
  if (d < 35) return "Loose-leaning";
  if (d > 65) return "Strict-leaning";
  return "Near current baseline";
}

// ── batch confirmation modal ────────────────────────────────────────────────

function StrictnessConfirmModal({
  dialPosition,
  deltas,
  thresholds,
  drifted,
  onCancel,
  onConfirm,
  busy,
}: {
  dialPosition: number;
  deltas: { key: string; oldValue: number; newValue: number }[];
  thresholds: ThresholdConfigDTO[];
  drifted: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  busy: boolean;
}) {
  const [ack, setAck] = useState(false);
  const canConfirm = drifted ? ack : true;
  const zone = dialZoneLabel(dialPosition);

  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(0,0,0,0.55)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
      }}
      role="dialog"
      aria-modal="true"
    >
      <div
        style={{
          width: "min(640px, 94vw)",
          maxHeight: "86vh",
          overflowY: "auto",
          borderRadius: 12,
          border: "1px solid var(--border)",
          background: "var(--surface)",
          padding: 20,
          display: "grid",
          gap: 14,
        }}
      >
        <div>
          <h3 style={{ margin: 0, fontSize: 15, fontWeight: 800 }}>
            Apply Strictness Level {dialPosition.toFixed(0)} ({zone})?
          </h3>
          <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--text-muted)" }}>
            This will adjust {deltas.length} threshold{deltas.length === 1 ? "" : "s"} in a single batch.
          </p>
        </div>

        {drifted ? (
          <div
            style={{
              padding: "10px 12px",
              borderRadius: 8,
              border: "1px solid var(--red)",
              background: "color-mix(in srgb, var(--red) 10%, transparent)",
              fontSize: 12,
            }}
          >
            Custom configuration detected — one or more of these thresholds were individually tuned and no longer
            match a single dial position. Applying this dial will OVERWRITE those individual customizations.
          </div>
        ) : null}

        <div style={{ display: "grid", gap: 6, maxHeight: 280, overflowY: "auto" }}>
          {deltas.map((d) => {
            const t = labelFor(d.key, thresholds);
            const increased = d.newValue > d.oldValue;
            return (
              <div
                key={d.key}
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  padding: "8px 10px",
                  borderRadius: 8,
                  background: "var(--surface-2)",
                  fontSize: 12,
                }}
              >
                <span style={{ fontWeight: 700 }}>{t?.label ?? d.key}</span>
                <span style={{ fontFamily: "var(--font-mono)" }}>
                  <span style={{ color: "var(--text-muted)" }}>{formatProfileValue(d.key, d.oldValue, thresholds)}</span>
                  {"  →  "}
                  <span style={{ color: increased ? "var(--green)" : "var(--red)", fontWeight: 800 }}>
                    {formatProfileValue(d.key, d.newValue, thresholds)}
                  </span>
                </span>
              </div>
            );
          })}
        </div>

        <p style={{ margin: 0, fontSize: 12 }}>
          This takes effect immediately for thresholds marked &ldquo;live,&rdquo; and on next restart for thresholds
          marked &ldquo;requires restart.&rdquo;
        </p>

        {drifted ? (
          <label style={{ display: "flex", gap: 8, alignItems: "flex-start", fontSize: 12, cursor: "pointer" }}>
            <input type="checkbox" checked={ack} onChange={(e) => setAck(e.target.checked)} style={{ marginTop: 2 }} />
            <span>I understand this will overwrite my individually-tuned values</span>
          </label>
        ) : null}

        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
          <button
            type="button"
            onClick={onCancel}
            disabled={busy}
            style={{
              padding: "8px 14px",
              borderRadius: 8,
              border: "1px solid var(--border)",
              background: "var(--surface-2)",
              color: "var(--text-primary)",
              fontWeight: 700,
              cursor: busy ? "not-allowed" : "pointer",
            }}
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={busy || !canConfirm}
            style={{
              padding: "8px 14px",
              borderRadius: 8,
              border: "none",
              background: canConfirm ? "var(--green)" : "var(--border)",
              color: canConfirm ? "#111" : "var(--text-muted)",
              fontWeight: 800,
              cursor: busy || !canConfirm ? "not-allowed" : "pointer",
            }}
          >
            {busy ? "Applying…" : "Confirm Strictness Change"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── main dial component ─────────────────────────────────────────────────────

export function StrictnessDial({
  thresholds,
  onApplied,
}: {
  thresholds: ThresholdConfigDTO[];
  /** Called with the updated threshold keys so the parent can refetch/merge. */
  onApplied: () => void;
}) {
  const [profiles, setProfiles] = useState<StrictnessProfile[]>([]);
  const [serverDialPosition, setServerDialPosition] = useState<number | null>(50);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState("");
  const [draftDial, setDraftDial] = useState(50);
  const [showAffected, setShowAffected] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [status, setStatus] = useState<DialStatus>("idle");
  const [applyError, setApplyError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/engine/config/strictness");
      const json = (await res.json()) as {
        ok?: boolean;
        currentDialPosition: number | null;
        profiles?: StrictnessProfile[];
        error?: string;
      };
      if (!res.ok || !json.ok) throw new Error(json.error ?? "Failed to load strictness profile");
      setProfiles(json.profiles ?? []);
      setServerDialPosition(json.currentDialPosition);
      setDraftDial(json.currentDialPosition ?? 50);
      setErrorMsg("");
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : "Failed to load strictness profile");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const drifted = serverDialPosition === null;

  const previewValues = useMemo(() => {
    const out = new Map<string, number>();
    for (const p of profiles) out.set(p.thresholdKey, interpolateStrictnessValue(p, draftDial));
    return out;
  }, [profiles, draftDial]);

  const estimatedImpactPct = useMemo(() => {
    if (profiles.length === 0) return 0;
    let sum = 0;
    for (const p of profiles) {
      const span = p.strictValue - p.looseValue;
      if (span === 0) continue;
      const now = previewValues.get(p.thresholdKey) ?? p.currentValue;
      let frac = (now - p.currentValue) / span;
      if (p.direction === "loosens") frac = -frac;
      sum += frac;
    }
    return (sum / profiles.length) * 100;
  }, [profiles, previewValues]);

  const deltasAtDraft = useMemo<ThresholdDelta[]>(
    () =>
      profiles.map((p) => ({
        key: p.thresholdKey,
        oldValue: p.currentValue,
        newValue: previewValues.get(p.thresholdKey) ?? p.currentValue,
      })),
    [profiles, previewValues],
  );

  const dirty = Math.round(draftDial) !== (serverDialPosition === null ? -1 : Math.round(serverDialPosition));

  const doApply = useCallback(async () => {
    setStatus("applying");
    setApplyError("");
    try {
      const res = await fetch("/api/engine/config/strictness", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ dialPosition: draftDial }),
      });
      const json = (await res.json()) as { ok?: boolean; error?: string };
      if (!res.ok || !json.ok) throw new Error(json.error ?? "Apply failed");
      setStatus("applied");
      setShowConfirm(false);
      onApplied();
      await load();
      window.setTimeout(() => setStatus("idle"), 2500);
    } catch (err) {
      setStatus("error");
      setApplyError(err instanceof Error ? err.message : "Apply failed");
    }
  }, [draftDial, onApplied, load]);

  if (loading) {
    return (
      <TerminalCard title="Overall Signal Strictness" subtitle="Loading…">
        <p style={{ margin: 0, fontSize: 12, color: "var(--text-muted)" }}>Loading strictness profile…</p>
      </TerminalCard>
    );
  }
  if (errorMsg && profiles.length === 0) {
    return (
      <TerminalCard title="Overall Signal Strictness" subtitle="Unavailable">
        <p style={{ margin: 0, fontSize: 12, color: "var(--red)" }}>{errorMsg}</p>
      </TerminalCard>
    );
  }

  return (
    <div
      style={{
        borderRadius: 14,
        border: "1px solid var(--amber)",
        background: "color-mix(in srgb, var(--amber) 7%, var(--surface))",
        padding: 20,
        display: "grid",
        gap: 14,
      }}
    >
      <div>
        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          <h2 style={{ margin: 0, fontSize: 16, fontWeight: 900, letterSpacing: "0.02em" }}>
            OVERALL SIGNAL STRICTNESS
          </h2>
          <span
            style={{
              fontSize: 10,
              fontWeight: 800,
              padding: "2px 8px",
              borderRadius: 999,
              border: "1px solid var(--amber)",
              color: "var(--amber)",
            }}
          >
            MASTER CONTROL
          </span>
        </div>
        <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--text-muted)" }}>
          One dial controls {profiles.length} of {thresholds.length} thresholds across all categories. Individual
          cards below remain available for fine-tuning.
        </p>
      </div>

      {drifted ? (
        <div
          style={{
            padding: "10px 12px",
            borderRadius: 8,
            border: "1px solid var(--red)",
            background: "color-mix(in srgb, var(--red) 10%, transparent)",
            fontSize: 12,
          }}
        >
          Custom configuration — thresholds have been individually tuned and no longer match a single dial position.
          Moving this dial will OVERWRITE your individual customizations for these {profiles.length} thresholds back
          to a uniform strictness level.
        </div>
      ) : (
        <div style={{ fontSize: 12, color: "var(--text-muted)" }}>
          Dial last set to {serverDialPosition?.toFixed(0)} ({dialZoneLabel(serverDialPosition ?? 50)})
        </div>
      )}

      <div style={{ display: "grid", gap: 6 }}>
        <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, fontWeight: 700 }}>
          <span style={{ color: "var(--green)" }}>
            LOOSE
            <br />
            <span style={{ fontWeight: 400, color: "var(--text-muted)" }}>More trades pass quality gates easily</span>
          </span>
          <span style={{ textAlign: "center", color: "var(--text-primary)" }}>
            CURRENT
            <br />
            <span style={{ fontWeight: 400, color: "var(--text-muted)" }}>No change from today&apos;s baseline</span>
          </span>
          <span style={{ textAlign: "right", color: "var(--red)" }}>
            STRICT
            <br />
            <span style={{ fontWeight: 400, color: "var(--text-muted)" }}>Fewer, higher-conviction trades pass</span>
          </span>
        </div>

        <input
          type="range"
          min={0}
          max={100}
          step={1}
          value={draftDial}
          onChange={(e) => setDraftDial(Number(e.target.value))}
          style={{ width: "100%", accentColor: "var(--amber)" }}
        />
        <div style={{ display: "flex", justifyContent: "space-between", fontSize: 10, color: "var(--text-muted)" }}>
          <span>0</span>
          <span style={{ fontWeight: 800, color: "var(--text-primary)" }}>{draftDial.toFixed(0)}</span>
          <span>100</span>
        </div>
      </div>

      <div
        style={{
          padding: "10px 12px",
          borderRadius: 8,
          background: "var(--surface-2)",
          fontSize: 12,
        }}
      >
        Estimated impact:{" "}
        <strong style={{ color: estimatedImpactPct >= 0 ? "var(--red)" : "var(--green)" }}>
          ~{Math.abs(Math.round(estimatedImpactPct))}% {estimatedImpactPct >= 0 ? "fewer" : "more"} signals
        </strong>{" "}
        would pass quality gates at this position vs. today&apos;s baseline.
      </div>

      <button
        type="button"
        onClick={() => setShowAffected((v) => !v)}
        style={{
          textAlign: "left",
          background: "transparent",
          border: "none",
          color: "var(--amber)",
          fontSize: 11,
          fontWeight: 700,
          cursor: "pointer",
          padding: 0,
        }}
      >
        {showAffected ? "▾ Hide" : "▸ Show"} all {profiles.length} affected thresholds at this position
      </button>

      {showAffected ? (
        <div style={{ overflowX: "auto" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <thead>
              <tr style={{ textAlign: "left", color: "var(--text-muted)", borderBottom: "1px solid var(--border)" }}>
                <th style={{ padding: "6px 8px" }}>Threshold</th>
                <th style={{ padding: "6px 8px" }}>Current</th>
                <th style={{ padding: "6px 8px" }}>At dial {draftDial.toFixed(0)}</th>
              </tr>
            </thead>
            <tbody>
              {deltasAtDraft.map((d) => {
                const t = labelFor(d.key, thresholds);
                const changed = Math.abs(d.newValue - d.oldValue) > 1e-9;
                return (
                  <tr key={d.key} style={{ borderBottom: "1px solid var(--border)" }}>
                    <td style={{ padding: "6px 8px" }}>{t?.label ?? d.key}</td>
                    <td style={{ padding: "6px 8px", fontFamily: "var(--font-mono)", color: "var(--text-muted)" }}>
                      {formatProfileValue(d.key, d.oldValue, thresholds)}
                    </td>
                    <td
                      style={{
                        padding: "6px 8px",
                        fontFamily: "var(--font-mono)",
                        fontWeight: changed ? 800 : 400,
                        color: changed ? (d.newValue > d.oldValue ? "var(--green)" : "var(--red)") : "var(--text-muted)",
                      }}
                    >
                      {formatProfileValue(d.key, d.newValue, thresholds)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      ) : null}

      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <button
          type="button"
          disabled={!dirty || status === "applying"}
          onClick={() => setShowConfirm(true)}
          style={{
            padding: "10px 16px",
            borderRadius: 8,
            border: "none",
            background: dirty ? "var(--amber)" : "var(--border)",
            color: dirty ? "#111" : "var(--text-muted)",
            fontWeight: 800,
            cursor: dirty && status !== "applying" ? "pointer" : "not-allowed",
          }}
        >
          Apply Strictness
        </button>
        <button
          type="button"
          disabled={!dirty}
          onClick={() => setDraftDial(serverDialPosition ?? 50)}
          style={{
            padding: "10px 16px",
            borderRadius: 8,
            border: "1px solid var(--border)",
            background: "var(--surface-2)",
            color: "var(--text-primary)",
            fontWeight: 700,
            cursor: dirty ? "pointer" : "not-allowed",
          }}
        >
          Reset to Current
        </button>
        {status === "applied" ? (
          <span style={{ color: "var(--green)", fontSize: 12, fontWeight: 700 }}>✓ Applied</span>
        ) : null}
        {status === "error" ? <span style={{ color: "var(--red)", fontSize: 12 }}>{applyError}</span> : null}
      </div>

      {showConfirm ? (
        <StrictnessConfirmModal
          dialPosition={draftDial}
          deltas={deltasAtDraft}
          thresholds={thresholds}
          drifted={drifted}
          busy={status === "applying"}
          onCancel={() => setShowConfirm(false)}
          onConfirm={() => void doApply()}
        />
      ) : null}
    </div>
  );
}
