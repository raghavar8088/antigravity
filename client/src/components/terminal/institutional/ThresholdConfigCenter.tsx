"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { TerminalCard } from "./TerminalCard";
import { StrictnessDial } from "@/components/StrictnessDial";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorBanner } from "@/components/ui/ErrorBanner";
import { Badge } from "@/components/ui/StatusChip";
import type {
  ConfigChangeEntry,
  ThresholdCategory,
  ThresholdConfigDTO,
} from "@/lib/thresholdConfig/types";
import {
  RISK_LEVEL_LABEL,
  THRESHOLD_CATEGORIES,
  formatThresholdValue,
} from "@/lib/thresholdConfig/types";

// Threshold keys controlled by the Master Strictness Dial — mirrors
// strictnessCatalog in engine/internal/config/strictness.go. Used only to
// append a one-line cross-reference to each affected card's "what happens if
// I change this" section; the engine remains the source of truth for which
// keys actually move with the dial.
const DIAL_CONTROLLED_KEYS = new Set([
  "MIN_EXECUTABLE_CONFIDENCE",
  "MIN_BRIDGE_CONFIDENCE",
  "MIN_REWARD_TO_RISK_RATIO",
  "MIN_SIGNAL_TAKE_PROFIT_PCT",
  "MAX_SIGNAL_STOP_LOSS_PCT",
  "SCALER_RR_MINIMUM",
  "RISK_CORRELATION_MIN_CONFIDENCE",
  "WALKFORWARD_WIN_RATE_THRESHOLD",
  "WALKFORWARD_SHARPE_THRESHOLD",
  "WALKFORWARD_MAX_DRAWDOWN",
  "WALKFORWARD_CONSECUTIVE_WINDOWS",
  "MIN_EXECUTION_WEIGHT_TO_TRADE",
]);

type SaveStatus = "idle" | "saving" | "saved" | "error";

// ── small shared building blocks ────────────────────────────────────────────

function RiskBadge({ level }: { level: ThresholdConfigDTO["riskLevel"] }) {
  return (
    <Badge variant={level === "critical" ? "loss" : level === "high" ? "warning" : level === "medium" ? "caution" : "neutral"} size="sm">
      {RISK_LEVEL_LABEL[level]}
    </Badge>
  );
}

function RestartBadge() {
  return (
    <span title="This change is written immediately but only takes effect after the engine restarts.">
      <Badge variant="warning" size="sm">Requires Restart</Badge>
    </span>
  );
}

function pillButtonStyle(active: boolean): React.CSSProperties {
  return {
    padding: "6px 12px",
    borderRadius: 8,
    border: "1px solid var(--border)",
    background: active ? "var(--amber)" : "var(--surface)",
    color: active ? "var(--text)" : "var(--text-primary)",
    fontWeight: 700,
    fontSize: 12,
    cursor: "pointer",
    whiteSpace: "nowrap",
  };
}

// ── summary stats header ────────────────────────────────────────────────────

type FilterMode = "all" | "pending_restart" | "changed_from_default";

function SummaryStats({
  thresholds,
  filter,
  onFilterChange,
}: {
  thresholds: ThresholdConfigDTO[];
  filter: FilterMode;
  onFilterChange: (f: FilterMode) => void;
}) {
  const total = thresholds.length;
  const pendingRestart = thresholds.filter((t) => t.requiresRestart).length;
  const changed = thresholds.filter((t) => t.currentValue !== t.defaultValue).length;

  const stat = (label: string, value: number, mode: FilterMode) => (
    <button
      type="button"
      onClick={() => onFilterChange(filter === mode ? "all" : mode)}
      style={{
        display: "flex",
        flexDirection: "column",
        gap: 2,
        padding: "10px 16px",
        borderRadius: 8,
        border: filter === mode ? "1px solid var(--amber)" : "1px solid var(--border)",
        background: filter === mode ? "color-mix(in srgb, var(--amber) 12%, var(--surface))" : "var(--surface-2)",
        cursor: "pointer",
        textAlign: "left",
        minWidth: 120,
      }}
    >
      <span style={{ fontSize: 20, fontWeight: 800, fontFamily: "var(--font-mono)" }}>{value}</span>
      <span style={{ fontSize: 11, color: "var(--text-muted)" }}>{label}</span>
    </button>
  );

  return (
    <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
      {stat("Thresholds", total, "all")}
      {stat("Pending Restart", pendingRestart, "pending_restart")}
      {stat("Changed From Default", changed, "changed_from_default")}
    </div>
  );
}

// ── read-only snapshot table ────────────────────────────────────────────────

function SnapshotTable({ thresholds }: { thresholds: ThresholdConfigDTO[] }) {
  return (
    <div style={{ overflowX: "auto" }}>
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
        <thead>
          <tr style={{ textAlign: "left", color: "var(--text-muted)", borderBottom: "1px solid var(--border)" }}>
            <th style={{ padding: "8px 10px" }}>Threshold</th>
            <th style={{ padding: "8px 10px" }}>Current</th>
            <th style={{ padding: "8px 10px" }}>Default</th>
            <th style={{ padding: "8px 10px" }}>Δ</th>
            <th style={{ padding: "8px 10px" }}>Category</th>
          </tr>
        </thead>
        <tbody>
          {thresholds.map((t) => {
            const changed = t.currentValue !== t.defaultValue;
            const delta = t.currentValue - t.defaultValue;
            return (
              <tr
                key={t.key}
                style={{
                  borderBottom: "1px solid var(--border)",
                  background: changed ? "color-mix(in srgb, var(--amber) 6%, transparent)" : "transparent",
                }}
              >
                <td style={{ padding: "8px 10px", fontWeight: changed ? 700 : 500 }}>{t.label}</td>
                <td style={{ padding: "8px 10px", fontFamily: "var(--font-mono)" }}>
                  {formatThresholdValue(t, t.currentValue)}
                </td>
                <td style={{ padding: "8px 10px", fontFamily: "var(--font-mono)", color: "var(--text-muted)" }}>
                  {formatThresholdValue(t, t.defaultValue)}
                </td>
                <td
                  style={{
                    padding: "8px 10px",
                    fontFamily: "var(--font-mono)",
                    color: delta > 0 ? "var(--green)" : delta < 0 ? "var(--red)" : "var(--text-muted)",
                  }}
                >
                  {changed ? (delta > 0 ? "+" : "") + delta.toFixed(4) : "—"}
                </td>
                <td style={{ padding: "8px 10px", color: "var(--text-muted)" }}>
                  {THRESHOLD_CATEGORIES.find((c) => c.key === t.category)?.label ?? t.category}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ── save confirmation modal ─────────────────────────────────────────────────

function SaveConfirmModal({
  threshold,
  newValue,
  onCancel,
  onConfirm,
  busy,
}: {
  threshold: ThresholdConfigDTO;
  newValue: number;
  onCancel: () => void;
  onConfirm: () => void;
  busy: boolean;
}) {
  const increased = newValue > threshold.currentValue;
  const isCritical = threshold.riskLevel === "critical";
  const [confirmText, setConfirmText] = useState("");
  const confirmRequired = isCritical ? confirmText.trim() === String(newValue) : true;

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
          width: "min(520px, 92vw)",
          borderRadius: 12,
          border: "1px solid var(--border)",
          background: "var(--surface)",
          padding: 20,
          display: "grid",
          gap: 14,
        }}
      >
        <div>
          <h3 style={{ margin: 0, fontSize: 15, fontWeight: 800 }}>Confirm threshold change</h3>
          <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--text-muted)" }}>{threshold.label}</p>
        </div>

        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            padding: "10px 12px",
            borderRadius: 8,
            background: "var(--surface-2)",
            fontFamily: "var(--font-mono)",
            fontSize: 14,
            fontWeight: 800,
          }}
        >
          <span style={{ color: "var(--text-muted)" }}>{formatThresholdValue(threshold, threshold.currentValue)}</span>
          <span>→</span>
          <span style={{ color: increased ? "var(--green)" : "var(--red)" }}>
            {formatThresholdValue(threshold, newValue)}
          </span>
        </div>

        <p style={{ margin: 0, fontSize: 12 }}>
          {threshold.requiresRestart
            ? "This value is written immediately but only takes effect after the next engine restart."
            : "This value takes effect on the next evaluation cycle — no restart required."}
        </p>

        <div style={{ display: "grid", gap: 4 }}>
          <p style={{ margin: 0, fontSize: 11, fontWeight: 700, color: "var(--text-muted)" }}>
            {increased ? "If increased:" : "If decreased:"}
          </p>
          <p style={{ margin: 0, fontSize: 12 }}>
            {increased ? threshold.whatHappensIfIncreased : threshold.whatHappensIfDecreased}
          </p>
        </div>

        {isCritical ? (
          <div style={{ display: "grid", gap: 6 }}>
            <label style={{ fontSize: 11, fontWeight: 700, color: "var(--red)" }}>
              CRITICAL threshold — type the new value ({newValue}) to confirm
            </label>
            <input
              type="text"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder={String(newValue)}
              style={{
                padding: "8px 10px",
                borderRadius: 6,
                border: "1px solid var(--red)",
                background: "var(--surface-2)",
                color: "var(--text-primary)",
                fontFamily: "var(--font-mono)",
              }}
            />
          </div>
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
            disabled={busy || !confirmRequired}
            style={{
              padding: "8px 14px",
              borderRadius: 8,
              border: "none",
              background: confirmRequired ? "var(--green)" : "var(--border)",
              color: confirmRequired ? "var(--text)" : "var(--text-muted)",
              fontWeight: 800,
              cursor: busy || !confirmRequired ? "not-allowed" : "pointer",
            }}
          >
            {busy ? "Saving…" : "Confirm & Save"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── per-threshold card ──────────────────────────────────────────────────────

function ThresholdEditor({
  threshold,
  value,
  onChange,
}: {
  threshold: ThresholdConfigDTO;
  value: number;
  onChange: (v: number) => void;
}) {
  const isSlider = threshold.dataType === "percentage" || threshold.dataType === "decimal";
  const step =
    threshold.dataType === "integer" || threshold.dataType === "duration"
      ? 1
      : Math.max((threshold.max - threshold.min) / 200, 0.0001);

  return (
    <div style={{ display: "grid", gap: 6 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10 }}>
        <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
          Range: {formatThresholdValue(threshold, threshold.min)} – {formatThresholdValue(threshold, threshold.max)}
        </span>
        <input
          type="number"
          value={value}
          min={threshold.min}
          max={threshold.max}
          step={step}
          onChange={(e) => {
            const num = Number(e.target.value);
            if (Number.isFinite(num)) onChange(num);
          }}
          style={{
            width: 100,
            padding: "4px 8px",
            borderRadius: 6,
            border: "1px solid var(--border)",
            background: "var(--surface)",
            color: "var(--text-primary)",
            textAlign: "right",
            fontFamily: "var(--font-mono)",
          }}
        />
      </div>
      {isSlider ? (
        <input
          type="range"
          min={threshold.min}
          max={threshold.max}
          step={step}
          value={value}
          onChange={(e) => onChange(Number(e.target.value))}
          style={{ width: "100%" }}
        />
      ) : null}
    </div>
  );
}

function ThresholdCard({
  threshold,
  onSaved,
}: {
  threshold: ThresholdConfigDTO;
  onSaved: (t: ThresholdConfigDTO) => void;
}) {
  const [draft, setDraft] = useState(threshold.currentValue);
  const [expandedWhatIf, setExpandedWhatIf] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [status, setStatus] = useState<SaveStatus>("idle");
  const [errorMsg, setErrorMsg] = useState("");

  useEffect(() => {
    setDraft(threshold.currentValue);
  }, [threshold.currentValue]);

  const dirty = draft !== threshold.currentValue;

  const doSave = async () => {
    setStatus("saving");
    setErrorMsg("");
    try {
      const res = await fetch("/api/engine/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: threshold.key, value: draft, reason: "Manual edit via Trade Threshold Configuration UI" }),
      });
      const json = (await res.json()) as { ok?: boolean; threshold?: ThresholdConfigDTO; error?: string };
      if (!res.ok || !json.ok || !json.threshold) {
        throw new Error(json.error ?? "Save failed");
      }
      setStatus("saved");
      setShowConfirm(false);
      onSaved(json.threshold);
      window.setTimeout(() => setStatus("idle"), 2500);
    } catch (err) {
      setStatus("error");
      setErrorMsg(err instanceof Error ? err.message : "Save failed");
    }
  };

  return (
    <div
      style={{
        display: "grid",
        gap: 10,
        padding: 16,
        borderRadius: 10,
        border: "1px solid var(--border)",
        background: "var(--surface)",
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 10 }}>
        <div>
          <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
            <span style={{ fontSize: 13, fontWeight: 800 }}>{threshold.label}</span>
            <RiskBadge level={threshold.riskLevel} />
            {threshold.requiresRestart ? <RestartBadge /> : null}
          </div>
          <p style={{ margin: "4px 0 0", fontSize: 11, color: "var(--text-muted)", maxWidth: 520 }}>
            {threshold.description}
          </p>
        </div>
        <div style={{ textAlign: "right", flexShrink: 0 }}>
          <p style={{ margin: 0, fontSize: 16, fontWeight: 800, fontFamily: "var(--font-mono)" }}>
            {formatThresholdValue(threshold, threshold.currentValue)}
          </p>
          <p style={{ margin: 0, fontSize: 10, color: "var(--text-muted)" }}>
            default {formatThresholdValue(threshold, threshold.defaultValue)}
          </p>
        </div>
      </div>

      <ThresholdEditor threshold={threshold} value={draft} onChange={setDraft} />

      <button
        type="button"
        onClick={() => setExpandedWhatIf((v) => !v)}
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
        {expandedWhatIf ? "▾ Hide impact details" : "▸ What happens if I change this?"}
      </button>
      {expandedWhatIf ? (
        <div style={{ display: "grid", gap: 8, fontSize: 11 }}>
          <div>
            <p style={{ margin: 0, fontWeight: 700, color: "var(--green)" }}>If increased</p>
            <p style={{ margin: "2px 0 0" }}>{threshold.whatHappensIfIncreased}</p>
          </div>
          <div>
            <p style={{ margin: 0, fontWeight: 700, color: "var(--red)" }}>If decreased</p>
            <p style={{ margin: "2px 0 0" }}>{threshold.whatHappensIfDecreased}</p>
          </div>
          <p style={{ margin: 0, color: "var(--text-muted)" }}>
            Recommended range: {formatThresholdValue(threshold, threshold.recommendedMin)} –{" "}
            {formatThresholdValue(threshold, threshold.recommendedMax)}
          </p>
          {DIAL_CONTROLLED_KEYS.has(threshold.key) ? (
            <p style={{ margin: 0, color: "var(--amber)" }}>
              This threshold is one of {DIAL_CONTROLLED_KEYS.size} controlled by the Overall Signal Strictness dial
              above. You can adjust it individually here, or move the master dial to adjust it together with related
              thresholds.
            </p>
          ) : null}
          {threshold.lastChangedAt ? (
            <p style={{ margin: 0, color: "var(--text-muted)" }}>
              Last changed: {new Date(threshold.lastChangedAt).toLocaleString()} (source: {threshold.source})
            </p>
          ) : null}
        </div>
      ) : null}

      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <button
          type="button"
          disabled={!dirty || status === "saving"}
          onClick={() => setShowConfirm(true)}
          style={{
            padding: "8px 14px",
            borderRadius: 8,
            border: "none",
            background: dirty ? "var(--green)" : "var(--border)",
            color: dirty ? "var(--text)" : "var(--text-muted)",
            fontWeight: 800,
            cursor: dirty && status !== "saving" ? "pointer" : "not-allowed",
          }}
        >
          Save
        </button>
        <button
          type="button"
          disabled={draft === threshold.defaultValue}
          onClick={() => setDraft(threshold.defaultValue)}
          style={{
            padding: "8px 14px",
            borderRadius: 8,
            border: "1px solid var(--border)",
            background: "var(--surface-2)",
            color: "var(--text-primary)",
            fontWeight: 700,
            cursor: draft === threshold.defaultValue ? "not-allowed" : "pointer",
          }}
        >
          Reset to Default
        </button>
        {status === "saved" ? <span style={{ color: "var(--green)", fontSize: 12, fontWeight: 700 }}>✓ Saved</span> : null}
        {status === "error" ? <span style={{ color: "var(--red)", fontSize: 12 }}>{errorMsg}</span> : null}
      </div>

      {showConfirm ? (
        <SaveConfirmModal
          threshold={threshold}
          newValue={draft}
          busy={status === "saving"}
          onCancel={() => setShowConfirm(false)}
          onConfirm={() => void doSave()}
        />
      ) : null}
    </div>
  );
}

// ── change history ──────────────────────────────────────────────────────────

function ChangeHistorySection() {
  const [open, setOpen] = useState(false);
  const [history, setHistory] = useState<ConfigChangeEntry[]>([]);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/engine/config/history?limit=50");
      const json = (await res.json()) as { ok?: boolean; history?: ConfigChangeEntry[] };
      if (res.ok && json.ok && json.history) setHistory(json.history);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (open) void load();
  }, [open, load]);

  return (
    <TerminalCard
      title="Configuration Change History"
      subtitle="Last 50 threshold edits across all categories, newest first"
      actions={
        <button type="button" onClick={() => setOpen((v) => !v)} style={pillButtonStyle(open)}>
          {open ? "Collapse" : "Expand"}
        </button>
      }
    >
      {!open ? (
        <p style={{ margin: 0, fontSize: 12, color: "var(--text-muted)" }}>Click Expand to load the audit trail.</p>
      ) : loading ? (
        <p style={{ margin: 0, fontSize: 12, color: "var(--text-muted)" }}>Loading…</p>
      ) : history.length === 0 ? (
        <p style={{ margin: 0, fontSize: 12, color: "var(--text-muted)" }}>No configuration changes recorded yet.</p>
      ) : (
        <div style={{ display: "grid", gap: 8 }}>
          {history.map((entry, idx) => {
            const id = entry.id || `${entry.key}-${entry.changedAt}-${idx}`;
            const expanded = expandedId === id;
            return (
              <div
                key={id}
                style={{
                  borderRadius: 8,
                  border: "1px solid var(--border)",
                  background: "var(--surface-2)",
                  padding: "10px 12px",
                  fontSize: 12,
                }}
              >
                <button
                  type="button"
                  onClick={() => setExpandedId(expanded ? null : id)}
                  style={{
                    width: "100%",
                    display: "flex",
                    justifyContent: "space-between",
                    background: "transparent",
                    border: "none",
                    color: "var(--text-primary)",
                    cursor: "pointer",
                    padding: 0,
                    textAlign: "left",
                  }}
                >
                  {entry.isBatch ? (
                    <span>
                      <strong style={{ color: "var(--amber)" }}>Master Strictness Dial</strong>: {entry.dialFrom?.toFixed(0)} →{" "}
                      {entry.dialTo?.toFixed(0)} · {entry.batchDeltas?.length ?? 0} thresholds adjusted [expand to view all]
                    </span>
                  ) : (
                    <span>
                      <strong>{entry.key}</strong>: {entry.oldValue} → {entry.newValue}
                      {entry.requiresRestart ? "  (requires restart)" : ""}
                    </span>
                  )}
                  <span style={{ color: "var(--text-muted)" }}>{new Date(entry.changedAt).toLocaleString()}</span>
                </button>
                {expanded ? (
                  <div style={{ marginTop: 8, display: "grid", gap: 4, color: "var(--text-muted)" }}>
                    <p style={{ margin: 0 }}>Changed by: {entry.changedBy}</p>
                    {entry.reason ? <p style={{ margin: 0 }}>Reason: {entry.reason}</p> : null}
                    {entry.isBatch && entry.batchDeltas ? (
                      <div style={{ display: "grid", gap: 4, marginTop: 4 }}>
                        {entry.batchDeltas.map((d) => (
                          <div
                            key={d.key}
                            style={{
                              display: "flex",
                              justifyContent: "space-between",
                              padding: "4px 8px",
                              borderRadius: 6,
                              background: "var(--surface)",
                              fontFamily: "var(--font-mono)",
                              fontSize: 11,
                            }}
                          >
                            <span>{d.key}</span>
                            <span>
                              {d.oldValue} → {d.newValue}
                            </span>
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      )}
    </TerminalCard>
  );
}

// ── main component ──────────────────────────────────────────────────────────

export function ThresholdConfigCenter() {
  const [thresholds, setThresholds] = useState<ThresholdConfigDTO[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState("");
  const [activeCategory, setActiveCategory] = useState<ThresholdCategory>("signal_quality");
  const [filter, setFilter] = useState<FilterMode>("all");

  const load = useCallback(async () => {
    try {
      const res = await fetch("/api/engine/config");
      const json = (await res.json()) as { ok?: boolean; thresholds?: ThresholdConfigDTO[]; error?: string };
      if (!res.ok || !json.ok || !json.thresholds) {
        throw new Error(json.error ?? "Failed to load threshold configuration");
      }
      setThresholds(json.thresholds);
      setErrorMsg("");
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : "Failed to load threshold configuration");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const handleSaved = useCallback((updated: ThresholdConfigDTO) => {
    setThresholds((prev) => prev.map((t) => (t.key === updated.key ? updated : t)));
  }, []);

  const filtered = useMemo(() => {
    let list = thresholds;
    if (filter === "pending_restart") list = list.filter((t) => t.requiresRestart);
    if (filter === "changed_from_default") list = list.filter((t) => t.currentValue !== t.defaultValue);
    return list;
  }, [thresholds, filter]);

  const byCategory = useMemo(() => filtered.filter((t) => t.category === activeCategory), [filtered, activeCategory]);

  const exportConfig = useCallback(() => {
    const blob = new Blob([JSON.stringify(thresholds, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `trade-threshold-config-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-")}.json`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }, [thresholds]);

  if (loading) {
    return (
      <div className="m3-page-stack">
        <PageHeader title="Trade Thresholds" subtitle="Live risk/signal threshold configuration for the trading engine" />
        <p style={{ fontSize: 12, color: "var(--text-muted)" }}>Loading threshold configuration…</p>
      </div>
    );
  }
  if (errorMsg && thresholds.length === 0) {
    return (
      <div className="m3-page-stack">
        <PageHeader title="Trade Thresholds" subtitle="Live risk/signal threshold configuration for the trading engine" />
        <ErrorBanner message={errorMsg} onRetry={load} />
      </div>
    );
  }

  return (
    <div className="m3-page-stack">
      <PageHeader title="Trade Thresholds" subtitle="Live risk/signal threshold configuration for the trading engine" />
      <div style={{ display: "grid", gap: 16 }}>
      <StrictnessDial thresholds={thresholds} onApplied={() => void load()} />

      <SummaryStats thresholds={thresholds} filter={filter} onFilterChange={setFilter} />

      <TerminalCard
        title="All Thresholds — Snapshot"
        subtitle="Current vs. default for every tunable threshold; rows where current ≠ default are highlighted"
        actions={
          <button type="button" onClick={exportConfig} style={pillButtonStyle(false)}>
            Export Configuration
          </button>
        }
      >
        <SnapshotTable thresholds={filtered} />
      </TerminalCard>

      <TerminalCard title="Edit Thresholds" subtitle="Select a category, adjust a value, then save with confirmation">
        <div style={{ display: "grid", gap: 14 }}>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            {THRESHOLD_CATEGORIES.map((c) => (
              <button
                key={c.key}
                type="button"
                onClick={() => setActiveCategory(c.key)}
                style={pillButtonStyle(activeCategory === c.key)}
              >
                {c.label} ({thresholds.filter((t) => t.category === c.key).length})
              </button>
            ))}
          </div>

          <div style={{ display: "grid", gap: 12 }}>
            {byCategory.length === 0 ? (
              <p style={{ margin: 0, fontSize: 12, color: "var(--text-muted)" }}>
                No thresholds in this category match the current filter.
              </p>
            ) : (
              byCategory.map((t) => <ThresholdCard key={t.key} threshold={t} onSaved={handleSaved} />)
            )}
          </div>
        </div>
      </TerminalCard>

      <ChangeHistorySection />
      </div>
    </div>
  );
}
