"use client";

import { useEffect, useState } from "react";

interface Candle {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

interface TradeReplayDrawerProps {
  trade: {
    trade_id?: string;
    strategy_name?: string;
    strategy_family?: string;
    side?: "BUY" | "SELL";
    entry_price?: number;
    close_price?: number;
    realized_pnl?: number;
    fees?: number;
    opened_at?: number;
    closed_at?: number;
    exit_reason?: string;
    notional?: number;
    leverage?: number;
  };
  onClose: () => void;
}

function formatMs(ms?: number): string {
  if (!ms) return "—";
  return new Date(ms).toLocaleString();
}

function holdMinutes(openedAt?: number, closedAt?: number): string {
  if (!openedAt || !closedAt) return "—";
  const mins = Math.round((closedAt - openedAt) / 60000);
  if (mins < 60) return `${mins}m`;
  return `${Math.floor(mins / 60)}h ${mins % 60}m`;
}

export function TradeReplayDrawer({ trade, onClose }: TradeReplayDrawerProps) {
  const [candles, setCandles] = useState<Candle[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!trade.opened_at) return;
    const from = trade.opened_at - 5 * 60 * 1000; // 5min before open
    const to = trade.closed_at ? trade.closed_at + 5 * 60 * 1000 : Date.now();

    setLoading(true);
    setError(null);

    fetch(`/api/btc/trade-candles?from=${from}&to=${to}&interval=1m`)
      .then((r) => r.json())
      .then((data: { candles?: Candle[]; error?: string }) => {
        if (data.error) throw new Error(data.error);
        setCandles(data.candles ?? []);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [trade.opened_at, trade.closed_at]);

  const netPnl = (trade.realized_pnl ?? 0) - (trade.fees ?? 0);
  const pnlColor = netPnl > 0 ? "#22c55e" : netPnl < 0 ? "#ef4444" : "#6b7280";

  return (
    <div
      style={{
        position: "fixed",
        right: 0,
        top: 0,
        bottom: 0,
        width: "400px",
        background: "#0f172a",
        color: "#e2e8f0",
        boxShadow: "-4px 0 24px rgba(0,0,0,0.5)",
        zIndex: 1000,
        overflowY: "auto",
        fontFamily: "monospace",
        fontSize: "13px",
      }}
    >
      {/* Header */}
      <div style={{ padding: "16px", borderBottom: "1px solid #1e293b", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <span style={{ fontWeight: "bold", fontSize: "15px" }}>Trade Replay</span>
        <button onClick={onClose} style={{ background: "none", border: "none", color: "#94a3b8", cursor: "pointer", fontSize: "18px" }}>✕</button>
      </div>

      {/* Trade Summary */}
      <div style={{ padding: "16px", borderBottom: "1px solid #1e293b" }}>
        <div style={{ marginBottom: "8px", color: "#94a3b8", fontSize: "11px", textTransform: "uppercase", letterSpacing: "0.1em" }}>Trade Summary</div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "8px" }}>
          <div><span style={{ color: "#64748b" }}>Strategy</span><br /><strong>{trade.strategy_name ?? "—"}</strong></div>
          <div><span style={{ color: "#64748b" }}>Family</span><br /><strong>{trade.strategy_family ?? "—"}</strong></div>
          <div>
            <span style={{ color: "#64748b" }}>Side</span><br />
            <strong style={{ color: trade.side === "BUY" ? "#22c55e" : "#ef4444" }}>{trade.side ?? "—"}</strong>
          </div>
          <div><span style={{ color: "#64748b" }}>Exit</span><br /><strong>{trade.exit_reason ?? "—"}</strong></div>
          <div><span style={{ color: "#64748b" }}>Entry</span><br /><strong>${(trade.entry_price ?? 0).toFixed(2)}</strong></div>
          <div><span style={{ color: "#64748b" }}>Exit</span><br /><strong>${(trade.close_price ?? 0).toFixed(2)}</strong></div>
          <div>
            <span style={{ color: "#64748b" }}>Net PnL</span><br />
            <strong style={{ color: pnlColor }}>${netPnl.toFixed(2)}</strong>
          </div>
          <div><span style={{ color: "#64748b" }}>Hold</span><br /><strong>{holdMinutes(trade.opened_at, trade.closed_at)}</strong></div>
          <div style={{ gridColumn: "1 / -1" }}><span style={{ color: "#64748b" }}>Opened</span><br /><strong>{formatMs(trade.opened_at)}</strong></div>
          <div style={{ gridColumn: "1 / -1" }}><span style={{ color: "#64748b" }}>Closed</span><br /><strong>{formatMs(trade.closed_at)}</strong></div>
        </div>
      </div>

      {/* Price Summary */}
      <div style={{ padding: "16px" }}>
        <div style={{ marginBottom: "8px", color: "#94a3b8", fontSize: "11px", textTransform: "uppercase", letterSpacing: "0.1em" }}>Price Action ({candles.length} candles)</div>
        {loading && <div style={{ color: "#64748b" }}>Loading candles…</div>}
        {error && <div style={{ color: "#ef4444" }}>Error: {error}</div>}
        {!loading && !error && candles.length === 0 && <div style={{ color: "#64748b" }}>No candle data available.</div>}
        {!loading && candles.length > 0 && (
          <div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr 1fr", gap: "4px", color: "#64748b", fontSize: "11px", marginBottom: "4px" }}>
              <span>TIME</span><span>OPEN</span><span>HIGH/LOW</span><span>CLOSE</span>
            </div>
            {candles.slice(0, 30).map((c) => {
              const isEntry = trade.opened_at && c.time <= trade.opened_at && c.time + 60000 > trade.opened_at;
              const isExit = trade.closed_at && c.time <= trade.closed_at && c.time + 60000 > trade.closed_at;
              const bg = isEntry ? "#1e3a5f" : isExit ? "#3f1f1f" : "transparent";
              return (
                <div
                  key={c.time}
                  style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr 1fr", gap: "4px", padding: "2px 0", background: bg, borderRadius: "2px" }}
                >
                  <span style={{ color: "#64748b" }}>{new Date(c.time).toLocaleTimeString()}</span>
                  <span>${c.open.toFixed(0)}</span>
                  <span style={{ color: "#94a3b8" }}>{c.high.toFixed(0)}/{c.low.toFixed(0)}</span>
                  <span style={{ color: c.close >= c.open ? "#22c55e" : "#ef4444" }}>${c.close.toFixed(0)}</span>
                </div>
              );
            })}
            {candles.length > 30 && (
              <div style={{ color: "#64748b", marginTop: "8px" }}>… and {candles.length - 30} more candles</div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export default TradeReplayDrawer;
