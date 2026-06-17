"use client";

import { useCallback, useEffect, useState } from "react";
import type {
  ExecutorConfigChangeView,
  ExecutorConfigView,
} from "@/lib/mockTradingExecutor/executorConfigConstants";
import type { SignalImpactReport } from "@/lib/mockTradingExecutor/signalImpactTypes";
import {
  MIN_SIGNAL_SCORE_MAX,
  MIN_SIGNAL_SCORE_MIN,
  SIGNAL_THRESHOLD_MAX,
  SIGNAL_THRESHOLD_MIN,
} from "@/lib/mockTradingExecutor/executorConfigConstants";

type SaveStatus = "idle" | "saving" | "saved" | "error";

type ThresholdSliderProps = {
  label: string;
  value: number;
  min: number;
  max: number;
  disabled?: boolean;
  description?: string;
  onChange?: (value: number) => void;
};

function ThresholdSlider({
  label,
  value,
  min,
  max,
  disabled = false,
  description,
  onChange,
}: ThresholdSliderProps) {
  const [inputValue, setInputValue] = useState(String(value));

  useEffect(() => {
    setInputValue(String(value));
  }, [value]);

  const applyValue = (raw: number) => {
    const clamped = Math.max(min, Math.min(max, Math.round(raw)));
    setInputValue(String(clamped));
    onChange?.(clamped);
  };

  return (
    <div style={{ display: "grid", gap: 8 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12 }}>
        <label style={{ fontSize: 13, fontWeight: 700, color: "var(--text-primary)" }}>{label}</label>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <input
            type="number"
            value={inputValue}
            disabled={disabled}
            min={min}
            max={max}
            onChange={(e) => {
              setInputValue(e.target.value);
              const num = Number(e.target.value);
              if (Number.isFinite(num)) applyValue(num);
            }}
            style={{
              width: 56,
              padding: "4px 8px",
              borderRadius: 6,
              border: "1px solid var(--border)",
              background: "var(--surface)",
              color: "var(--text-primary)",
              textAlign: "right",
              fontFamily: "var(--font-mono)",
            }}
          />
          <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
            ({min}–{max})
          </span>
        </div>
      </div>
      <input
        type="range"
        min={min}
        max={max}
        value={value}
        disabled={disabled}
        onChange={(e) => applyValue(Number(e.target.value))}
        style={{ width: "100%" }}
      />
      {description ? <p style={{ margin: 0, fontSize: 11, color: "var(--text-muted)" }}>{description}</p> : null}
    </div>
  );
}

function ConfigStatusBar({
  config,
  saveStatus,
  executorAgeSeconds,
}: {
  config: ExecutorConfigView;
  saveStatus: SaveStatus;
  executorAgeSeconds: number | null;
}) {
  return (
    <div
      style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        gap: 12,
        padding: "12px 14px",
        borderRadius: 8,
        border: "1px solid var(--border)",
        background: "var(--surface-2)",
        fontSize: 12,
      }}
    >
      <div>
        <p style={{ margin: 0, fontWeight: 700, color: "var(--text-primary)" }}>
          Saved to: Mock Trading DB
          <span style={{ marginLeft: 8, color: "var(--text-muted)", fontWeight: 500 }}>
            ({config.source === "mongodb" ? "MongoDB" : config.source === "env" ? "Environment default" : "Fallback"})
          </span>
        </p>
        <p style={{ margin: "4px 0 0", color: "var(--text-muted)" }}>
          Last updated: {new Date(config.updatedAt).toLocaleString()}
          {executorAgeSeconds != null ? ` · Executor cycle ${executorAgeSeconds}s ago` : ""}
        </p>
        <p style={{ margin: "4px 0 0", color: "var(--amber)" }}>Next cycle will use saved thresholds (≤5s cache).</p>
      </div>
      <div style={{ fontWeight: 700 }}>
        {saveStatus === "saved" ? <span style={{ color: "var(--green)" }}>✓ Saved</span> : null}
        {saveStatus === "saving" ? <span style={{ color: "var(--amber)" }}>Saving…</span> : null}
        {saveStatus === "error" ? <span style={{ color: "var(--red)" }}>✗ Error</span> : null}
      </div>
    </div>
  );
}

function ImpactPreview({ impact }: { impact: SignalImpactReport }) {
  return (
    <div
      style={{
        border: "1px solid color-mix(in srgb, var(--amber) 35%, var(--border))",
        background: "color-mix(in srgb, var(--surface) 90%, var(--amber) 10%)",
        borderRadius: 8,
        padding: 14,
        display: "grid",
        gap: 10,
      }}
    >
      <h3 style={{ margin: 0, fontSize: 13, fontWeight: 800 }}>Preview: impact of new thresholds</h3>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 10, fontSize: 12 }}>
        <div>
          <p style={{ margin: 0, color: "var(--text-muted)" }}>Signal threshold</p>
          <p style={{ margin: 0, fontWeight: 800 }}>
            {impact.currentThreshold} → {impact.testThreshold}
          </p>
        </div>
        <div>
          <p style={{ margin: 0, color: "var(--text-muted)" }}>Open threshold</p>
          <p style={{ margin: 0, fontWeight: 800 }}>
            {impact.currentMinSignalScore} → {impact.testMinSignalScore}
          </p>
        </div>
      </div>
      <p style={{ margin: 0, fontSize: 12 }}>
        {impact.strategiesAboveSignalThreshold} of {impact.evaluatedStrategies} strategies pass signal threshold ·{" "}
        {impact.strategiesAboveOpenThreshold} pass open threshold · {impact.strategiesFullyQualified} fully qualified
      </p>
      <div style={{ display: "grid", gap: 4, maxHeight: 180, overflowY: "auto" }}>
        {impact.strategies.map((strat) => (
          <div
            key={`${strat.strategyId}-${strat.name}`}
            style={{
              display: "grid",
              gridTemplateColumns: "1fr auto auto",
              gap: 8,
              alignItems: "center",
              fontSize: 11,
              padding: "6px 8px",
              borderRadius: 6,
              background: "var(--surface)",
            }}
          >
            <span style={{ color: "var(--text-secondary)" }}>{strat.name}</span>
            <span style={{ fontFamily: "var(--font-mono)", color: "var(--text-muted)" }}>
              {strat.currentScore.toFixed(1)}
            </span>
            <span style={{ color: strat.wouldQualify ? "var(--green)" : "var(--red)", fontWeight: 700 }}>
              {strat.wouldQualify ? "PASS" : strat.signalThresholdPass ? "OPEN BLOCK" : "SIGNAL BLOCK"}
            </span>
          </div>
        ))}
      </div>
      <p style={{ margin: 0, fontSize: 11, color: "var(--amber)" }}>
        Higher thresholds mean fewer trades but stronger setups. Lower thresholds increase frequency and false-signal risk.
      </p>
    </div>
  );
}

function ConfigChangeHistory({
  history,
  onRevert,
  busy,
}: {
  history: ExecutorConfigChangeView[];
  onRevert: (entry: ExecutorConfigChangeView) => void;
  busy: boolean;
}) {
  if (history.length === 0) {
    return (
      <p style={{ margin: 0, fontSize: 12, color: "var(--text-muted)" }}>No configuration changes recorded yet.</p>
    );
  }

  return (
    <div style={{ display: "grid", gap: 8 }}>
      {history.map((entry, idx) => (
        <div
          key={entry.id || `${entry.timestamp}-${idx}`}
          style={{
            display: "flex",
            justifyContent: "space-between",
            gap: 12,
            padding: "10px 12px",
            borderRadius: 8,
            border: "1px solid var(--border)",
            background: "var(--surface-2)",
            fontSize: 12,
          }}
        >
          <div>
            <p style={{ margin: 0, fontWeight: 700 }}>
              #{idx + 1} · {new Date(entry.timestamp).toLocaleString()}
            </p>
            <p style={{ margin: "4px 0 0", color: "var(--text-muted)" }}>
              {Object.entries(entry.changes)
                .map(([key, vals]) => `${key}: ${vals.old} → ${vals.new}`)
                .join(" · ")}
            </p>
            {entry.reason ? (
              <p style={{ margin: "4px 0 0", color: "var(--text-muted)", fontStyle: "italic" }}>{entry.reason}</p>
            ) : null}
          </div>
          <button
            type="button"
            disabled={busy}
            onClick={() => onRevert(entry)}
            style={{
              border: "none",
              background: "transparent",
              color: "var(--amber)",
              fontWeight: 700,
              cursor: busy ? "not-allowed" : "pointer",
              whiteSpace: "nowrap",
            }}
          >
            Revert
          </button>
        </div>
      ))}
    </div>
  );
}

export type MockTradingConfigurationPanelProps = {
  accountKey: string;
  executorAgeSeconds?: number | null;
};

export function MockTradingConfigurationPanel({
  accountKey,
  executorAgeSeconds = null,
}: MockTradingConfigurationPanelProps) {
  const [config, setConfig] = useState<ExecutorConfigView | null>(null);
  const [loading, setLoading] = useState(true);
  const [testMode, setTestMode] = useState(false);
  const [testThreshold, setTestThreshold] = useState(SIGNAL_THRESHOLD_MIN);
  const [testMinScore, setTestMinScore] = useState(MIN_SIGNAL_SCORE_MIN);
  const [impact, setImpact] = useState<SignalImpactReport | null>(null);
  const [history, setHistory] = useState<ExecutorConfigChangeView[]>([]);
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("idle");
  const [errorMsg, setErrorMsg] = useState("");

  const fetchConfig = useCallback(async () => {
    const params = new URLSearchParams({ account_key: accountKey });
    const res = await fetch(`/api/mock-trading/config?${params.toString()}`);
    const json = (await res.json()) as { ok?: boolean; config?: ExecutorConfigView; error?: string };
    if (!res.ok || !json.ok || !json.config) {
      throw new Error(json.error ?? "Failed to load config");
    }
    setConfig(json.config);
    if (!testMode) {
      setTestThreshold(json.config.signalThreshold);
      setTestMinScore(json.config.minSignalScore);
    }
  }, [accountKey, testMode]);

  const fetchHistory = useCallback(async () => {
    const params = new URLSearchParams({ account_key: accountKey, limit: "10" });
    const res = await fetch(`/api/mock-trading/config/history?${params.toString()}`);
    const json = (await res.json()) as { ok?: boolean; history?: ExecutorConfigChangeView[] };
    if (res.ok && json.ok && json.history) setHistory(json.history);
  }, [accountKey]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        await fetchConfig();
        await fetchHistory();
      } catch (err) {
        if (!cancelled) {
          setErrorMsg(err instanceof Error ? err.message : "Failed to load configuration");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    const interval = window.setInterval(() => {
      void fetchConfig().catch(() => {});
    }, 10_000);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [fetchConfig, fetchHistory]);

  useEffect(() => {
    if (!testMode) return;
    let cancelled = false;
    const params = new URLSearchParams({
      account_key: accountKey,
      testThreshold: String(testThreshold),
      testMinScore: String(testMinScore),
    });
    const timer = window.setTimeout(() => {
      void fetch(`/api/mock-trading/signal-impact?${params.toString()}`)
        .then((r) => r.json())
        .then((json: { ok?: boolean; impact?: SignalImpactReport }) => {
          if (!cancelled && json.ok && json.impact) setImpact(json.impact);
        })
        .catch(() => {
          if (!cancelled) setImpact(null);
        });
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [accountKey, testMinScore, testMode, testThreshold]);

  const saveConfig = async () => {
    setSaveStatus("saving");
    setErrorMsg("");
    try {
      const params = new URLSearchParams({ account_key: accountKey });
      const res = await fetch(`/api/mock-trading/config?${params.toString()}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          signalThreshold: testThreshold,
          minSignalScore: testMinScore,
          reason: "Manual adjustment via Trade Engine UI",
        }),
      });
      const json = (await res.json()) as { ok?: boolean; config?: ExecutorConfigView; error?: string };
      if (!res.ok || !json.ok || !json.config) {
        throw new Error(json.error ?? "Save failed");
      }
      setConfig(json.config);
      setTestMode(false);
      setSaveStatus("saved");
      window.setTimeout(() => setSaveStatus("idle"), 3000);
      await fetchHistory();
    } catch (err) {
      setSaveStatus("error");
      setErrorMsg(err instanceof Error ? err.message : "Save failed");
    }
  };

  const revertToEntry = async (entry: ExecutorConfigChangeView) => {
    const signalOld = entry.changes.signalThreshold?.old;
    const minOld = entry.changes.minSignalScore?.old;
    if (signalOld == null && minOld == null) return;
    setSaveStatus("saving");
    setErrorMsg("");
    try {
      const params = new URLSearchParams({ account_key: accountKey });
      const res = await fetch(`/api/mock-trading/config?${params.toString()}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          signalThreshold: signalOld ?? config?.signalThreshold ?? 26,
          minSignalScore: minOld ?? config?.minSignalScore ?? 50,
          reason: `Reverted to config from ${new Date(entry.timestamp).toLocaleString()}`,
        }),
      });
      const json = (await res.json()) as { ok?: boolean; config?: ExecutorConfigView; error?: string };
      if (!res.ok || !json.ok || !json.config) {
        throw new Error(json.error ?? "Revert failed");
      }
      setConfig(json.config);
      setTestThreshold(json.config.signalThreshold);
      setTestMinScore(json.config.minSignalScore);
      setTestMode(false);
      setSaveStatus("saved");
      await fetchHistory();
    } catch (err) {
      setSaveStatus("error");
      setErrorMsg(err instanceof Error ? err.message : "Revert failed");
    }
  };

  if (loading) {
    return <p style={{ fontSize: 12, color: "var(--text-muted)" }}>Loading threshold configuration…</p>;
  }
  if (!config) {
    return <p style={{ fontSize: 12, color: "var(--red)" }}>{errorMsg || "Failed to load configuration."}</p>;
  }

  return (
    <div style={{ display: "grid", gap: 14 }}>
      <ConfigStatusBar config={config} saveStatus={saveStatus} executorAgeSeconds={executorAgeSeconds} />

      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <button
          type="button"
          onClick={() => setTestMode(false)}
          style={{
            padding: "8px 14px",
            borderRadius: 8,
            border: "1px solid var(--border)",
            background: !testMode ? "var(--amber)" : "var(--surface)",
            color: !testMode ? "#111" : "var(--text-primary)",
            fontWeight: 700,
            cursor: "pointer",
          }}
        >
          Current Config
        </button>
        <button
          type="button"
          onClick={() => {
            setTestThreshold(config.signalThreshold);
            setTestMinScore(config.minSignalScore);
            setTestMode(true);
          }}
          style={{
            padding: "8px 14px",
            borderRadius: 8,
            border: "1px solid var(--border)",
            background: testMode ? "var(--amber)" : "var(--surface)",
            color: testMode ? "#111" : "var(--text-primary)",
            fontWeight: 700,
            cursor: "pointer",
          }}
        >
          Test / Preview
        </button>
      </div>

      <div
        style={{
          display: "grid",
          gap: 16,
          padding: 16,
          borderRadius: 10,
          border: "1px solid var(--card-border, var(--border))",
          background: "var(--card-bg, var(--surface))",
        }}
      >
        <ThresholdSlider
          label="Signal Threshold"
          value={testMode ? testThreshold : config.signalThreshold}
          min={SIGNAL_THRESHOLD_MIN}
          max={SIGNAL_THRESHOLD_MAX}
          disabled={!testMode}
          onChange={setTestThreshold}
          description="Minimum strategy score to qualify (18–32). Higher = fewer false setups."
        />
        <ThresholdSlider
          label="Trade Open Threshold (minSignalScore)"
          value={testMode ? testMinScore : config.minSignalScore}
          min={MIN_SIGNAL_SCORE_MIN}
          max={MIN_SIGNAL_SCORE_MAX}
          disabled={!testMode}
          onChange={setTestMinScore}
          description="Minimum score to open a trade (30–70). Trade opens only when score ≥ this value."
        />

        {testMode && impact ? <ImpactPreview impact={impact} /> : null}

        {testMode ? (
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button
              type="button"
              disabled={saveStatus === "saving"}
              onClick={() => void saveConfig()}
              style={{
                padding: "10px 16px",
                borderRadius: 8,
                border: "none",
                background: "var(--green)",
                color: "#111",
                fontWeight: 800,
                cursor: saveStatus === "saving" ? "not-allowed" : "pointer",
              }}
            >
              {saveStatus === "saving" ? "Saving…" : "Apply Changes"}
            </button>
            <button
              type="button"
              onClick={() => {
                setTestThreshold(config.signalThreshold);
                setTestMinScore(config.minSignalScore);
                setTestMode(false);
              }}
              style={{
                padding: "10px 16px",
                borderRadius: 8,
                border: "1px solid var(--border)",
                background: "var(--surface-2)",
                color: "var(--text-primary)",
                fontWeight: 700,
                cursor: "pointer",
              }}
            >
              Cancel
            </button>
          </div>
        ) : null}
      </div>

      {errorMsg ? (
        <div
          style={{
            padding: 12,
            borderRadius: 8,
            border: "1px solid var(--red)",
            color: "var(--red)",
            fontSize: 12,
          }}
        >
          {errorMsg}
        </div>
      ) : null}

      <div style={{ display: "grid", gap: 10 }}>
        <h3 style={{ margin: 0, fontSize: 14, fontWeight: 800 }}>Configuration History</h3>
        <ConfigChangeHistory
          history={history}
          onRevert={(entry) => void revertToEntry(entry)}
          busy={saveStatus === "saving"}
        />
      </div>
    </div>
  );
}
