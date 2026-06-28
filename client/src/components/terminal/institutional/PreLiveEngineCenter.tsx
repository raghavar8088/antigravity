"use client";

import { type CSSProperties, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { PageHeader } from "@/components/ui/PageHeader";

import { Sparkline } from "@/components/ui/Sparkline";
import { SkeletonBlock } from "@/components/ui/EmptyState";
import { TerminalCard } from "./TerminalCard";
import { RegimeBadge } from "./MockStageTradingSuite";

// ── Constants ─────────────────────────────────────────────────────────────────
const POLL_MS = 5_000;
const SIGNALS_POLL_MS = 3_000;
const PRE_LIVE_STRATEGIES = 100;
const LEVERAGE = 10;

// ── Types ─────────────────────────────────────────────────────────────────────
// Field names match the Go engine JSON output from /api/positions and /api/trades
type OpenPosition = {
  id: string;
  strategyName: string;
  side: "BUY" | "SELL" | "LONG" | "SHORT";
  openedAt: string | number;   // ISO string from engine
  size: number;                // engine uses "size" not "quantity"
  entryPrice: number;
  stopLoss: number;            // engine uses "stopLoss" not "stopLossPrice"
  takeProfit: number;          // engine uses "takeProfit" not "takeProfitPrice"
  stopLossPct: number;
  takeProfitPct: number;
  // unrealizedPnl not returned by engine; computed from live price
};

type ClosedTrade = {
  id: string;
  strategyName: string;
  side: "BUY" | "SELL";
  entryPrice: number;
  exitPrice: number;
  netPnl: number;
  entryTime: string | number;  // ISO string from engine
  exitTime: string | number;   // ISO string from engine
  duration: number;            // nanoseconds
  reason: string;
};

type Stats = {
  balance: number;
  equity: number;
  initialBalance?: number;
  dailyPnl: number;
  openPositions: number;
  strategies: number;
  aggregate?: {
    totalTrades?: number;
    winRate?: number;
    realizedPnl?: number;
    unrealizedPnl?: number;
  };
};

type Signal = {
  strategy: string;
  side: string;
  confidence: number;
  timestamp?: string;
  price?: number;
};

type ScalersStats = {
  regime: string;
  recentSignals: Signal[];
};

// ── Styles ────────────────────────────────────────────────────────────────────
const tableStyle: CSSProperties = {
  width: "100%",
  borderCollapse: "separate",
  borderSpacing: 0,
  fontSize: 13,
  minWidth: 920,
};
const thStyle: CSSProperties = {
  padding: "12px 12px",
  fontSize: 11,
  fontWeight: 700,
  textTransform: "uppercase",
  letterSpacing: "0.06em",
  color: "var(--text-muted)",
  borderBottom: "1px solid var(--border-subtle, var(--border))",
  background: "var(--surface-2)",
  whiteSpace: "nowrap",
};
const tdStyle: CSSProperties = {
  padding: "13px 12px",
  verticalAlign: "middle",
  color: "var(--text-secondary)",
  borderBottom: "1px solid var(--border-subtle, var(--border))",
};
const monoCellStyle: CSSProperties = {
  fontFamily: "var(--font-mono)",
  fontVariantNumeric: "tabular-nums",
  whiteSpace: "nowrap",
};

function rowBg(i: number) { return i % 2 === 1 ? "#fbfdff" : "#ffffff"; }
function fmtUsd(v: number) {
  if (!Number.isFinite(v)) return "—";
  const sign = v > 0 ? "+" : v < 0 ? "-" : "";
  const abs = Math.abs(v);
  const decimals = abs > 0 && abs < 0.01 ? 4 : 2;
  return `${sign}$${abs.toLocaleString("en-US", { minimumFractionDigits: decimals, maximumFractionDigits: decimals })}`;
}
function fmtPrice(v: number) {
  if (!Number.isFinite(v)) return "—";
  return `$${v.toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}
function fmtPct(v: number) { return Number.isFinite(v) ? `${v.toFixed(1)}%` : "—"; }
function pnlColor(v: number) { return v > 0 ? "var(--green)" : v < 0 ? "var(--red)" : "var(--text-muted)"; }
function sideColor(s: string) { return s === "BUY" || s === "LONG" ? "var(--green)" : "var(--red)"; }
// Convert ISO string or ms number to ms timestamp
function toMs(ts: string | number | undefined | null): number {
  if (!ts) return NaN;
  if (typeof ts === "number") return ts < 1e12 ? ts * 1000 : ts;
  const ms = Date.parse(ts);
  return isNaN(ms) ? NaN : ms;
}
function fmtAge(start: string | number, end: string | number | null | undefined) {
  const startMs = toMs(start);
  const endMs = end != null ? toMs(end) : Date.now();
  if (isNaN(startMs) || isNaN(endMs)) return "—";
  const mins = Math.max(0, Math.floor((endMs - startMs) / 60_000));
  if (mins < 60) return `${mins}m`;
  return `${Math.floor(mins / 60)}h ${mins % 60}m`;
}
function fmtDuration(ns: number | undefined) {
  if (!ns || !Number.isFinite(ns)) return "—";
  const mins = Math.floor(ns / 60_000_000_000);
  if (mins < 60) return `${mins}m`;
  return `${Math.floor(mins / 60)}h ${mins % 60}m`;
}
function fmtTime(ts: string | number | undefined | null) {
  const ms = toMs(ts);
  if (isNaN(ms)) return "—";
  return new Intl.DateTimeFormat("en-IN", {
    month: "short", day: "2-digit",
    hour: "numeric", minute: "2-digit", hour12: true,
    timeZone: "Asia/Kolkata",
  }).format(ms) + " IST";
}

// ── Fetchers ──────────────────────────────────────────────────────────────────
async function fetchJson<T>(path: string): Promise<T | null> {
  try {
    const r = await fetch(`/api/pre-live${path}`, { cache: "no-store" });
    if (!r.ok) return null;
    return r.json() as Promise<T>;
  } catch { return null; }
}

// ── Sub-components ────────────────────────────────────────────────────────────
function EmptyState({ label }: { label: string }) {
  return (
    <div className="google-empty-state" role="status" style={{ minHeight: 72, padding: "16px 16px" }}>
      <p className="m3-empty-state__title">{label}</p>
      <p className="m3-empty-state__subtitle">Updates automatically when Pre-Live Engine activity is available.</p>
    </div>
  );
}

function ScrollHint() {
  return (
    <div style={{ textAlign: "center", fontSize: 10, color: "var(--text-muted)", marginTop: 5, letterSpacing: "0.05em", userSelect: "none", pointerEvents: "none" }}>
      ← swipe to scroll →
    </div>
  );
}

function OpenPositionsTable({ positions, nowMs, markPrice }: { positions: OpenPosition[]; nowMs: number; markPrice: number }) {
  if (positions.length === 0) return <EmptyState label="No open positions" />;
  const hasLivePrice = Number.isFinite(markPrice) && markPrice > 0;
  return (
    <>
      <div style={{ overflowX: "scroll", overflowY: "visible", paddingBottom: 4 }}>
        <table style={tableStyle}>
          <thead>
            <tr>
              {["STRATEGY", "SIDE", "OPENED (IST)", "SIZE", "ENTRY", "MARK", "UNREALIZED PnL", "SL", "TP", "AGE"].map((h, i) => (
                <th key={h} style={{ ...thStyle, textAlign: i >= 3 ? "right" : "left" }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {positions.map((p, i) => {
              const mark = hasLivePrice ? markPrice : p.entryPrice;
              const isLong = p.side === "BUY" || p.side === "LONG";
              const unrealizedPnl = Number.isFinite(p.size) && Number.isFinite(p.entryPrice)
                ? (isLong ? mark - p.entryPrice : p.entryPrice - mark) * p.size * LEVERAGE
                : NaN;
              return (
                <tr key={p.id} style={{ background: rowBg(i) }}>
                  <td style={{ ...tdStyle, maxWidth: 210, color: "var(--text-primary)", fontWeight: 600 }}>
                    <div style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{p.strategyName}</div>
                  </td>
                  <td style={{ ...tdStyle, color: sideColor(p.side), fontWeight: 700 }}>{p.side}</td>
                  <td style={{ ...tdStyle, ...monoCellStyle, fontSize: 12 }}>{fmtTime(p.openedAt)}</td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{Number.isFinite(p.size) ? `${p.size.toFixed(4)} BTC` : "—"}</td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(p.entryPrice)}</td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right", color: "var(--text-primary)" }}>{fmtPrice(mark)}</td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right", fontWeight: 700, color: pnlColor(unrealizedPnl) }}>{fmtUsd(unrealizedPnl)}</td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(p.stopLoss)}</td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(p.takeProfit)}</td>
                  <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtAge(p.openedAt, null)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <ScrollHint />
    </>
  );
}

function ClosedTradesTable({ trades, nowMs }: { trades: ClosedTrade[]; nowMs: number }) {
  const [limit, setLimit] = useState(50);
  if (trades.length === 0) return <EmptyState label="No closed trades" />;
  const visible = trades.slice(0, limit);
  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 10, color: "var(--text-muted)", fontSize: 11 }}>
        <span>{trades.length.toLocaleString()} trades</span>
        {trades.length > limit && (
          <button type="button" onClick={() => setLimit((l) => l + 50)}
            style={{ border: 0, background: "transparent", color: "var(--accent)", cursor: "pointer", fontSize: 11, fontWeight: 700, padding: 0, textDecoration: "underline" }}>
            Show more
          </button>
        )}
      </div>
      <div style={{ overflowX: "scroll", overflowY: "visible", paddingBottom: 4 }}>
        <table style={tableStyle}>
          <thead>
            <tr>
              {["STRATEGY", "SIDE", "OPENED (IST)", "ENTRY", "EXIT", "PnL", "REASON", "DURATION"].map((h, i) => (
                <th key={h} style={{ ...thStyle, textAlign: i <= 2 || i === 6 ? "left" : "right" }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((t, i) => (
              <tr key={t.id} style={{ background: rowBg(i) }}>
                <td style={{ ...tdStyle, maxWidth: 210, color: "var(--text-primary)", fontWeight: 600 }}>
                  <div style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{t.strategyName}</div>
                </td>
                <td style={{ ...tdStyle, color: sideColor(t.side), fontWeight: 700 }}>{t.side}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, fontSize: 12 }}>{fmtTime(t.entryTime as string)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(t.entryPrice)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(t.exitPrice)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right", color: pnlColor(t.netPnl), fontWeight: 700 }}>{fmtUsd(t.netPnl)}</td>
                <td style={tdStyle}>
                  {(() => {
                    const r = t.reason ?? "";
                    const isTP = r === "TAKE_PROFIT" || r === "TP";
                    const isSL = r === "STOP_LOSS" || r === "SL";
                    const isExpired = r === "EXPIRED" || r === "MANUAL";
                    const bg = isTP ? "var(--green-dim)" : isSL ? "var(--red-dim)" : isExpired ? "color-mix(in srgb, var(--text-muted) 15%, transparent)" : "var(--amber-dim)";
                    const fg = isTP ? "var(--green)" : isSL ? "var(--red)" : isExpired ? "var(--text-muted)" : "var(--amber)";
                    const label = isTP ? "TP" : isSL ? "SL" : isExpired ? "EXPIRED" : (r || "—");
                    return (
                      <span style={{ display: "inline-flex", alignItems: "center", borderRadius: 999, background: bg, color: fg, fontSize: 10, fontWeight: 700, letterSpacing: "0.04em", padding: "2px 7px", textTransform: "uppercase" }}>
                        {label}
                      </span>
                    );
                  })()}
                </td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtDuration(t.duration)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <ScrollHint />
    </div>
  );
}

function LiveSignalsPanel({ scalers }: { scalers: ScalersStats }) {
  const signals = scalers.recentSignals ?? [];
  if (signals.length === 0) {
    return (
      <div className="m3-terminal-empty-state" style={{ minHeight: 72, padding: "16px 16px" }}>
        <div className="m3-terminal-empty-state__title">No live signals</div>
        <div className="m3-terminal-empty-state__subtitle">Waiting for pre-live signal telemetry.</div>
      </div>
    );
  }
  return (
    <div style={{ display: "grid", gap: 8 }}>
      {signals.slice(0, 10).map((sig, idx) => {
        const color = sideColor(sig.side);
        return (
          <article key={`${sig.strategy}-${idx}`} style={{ border: "1px solid var(--border-subtle, var(--border))", borderRadius: 8, background: "var(--surface-2)", padding: "8px 9px" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <span style={{ width: 7, height: 7, borderRadius: 999, background: color, flex: "0 0 auto" }} />
              <span style={{ color: "var(--text-primary)", fontSize: 12, fontWeight: 700, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{sig.strategy}</span>
              <span style={{ color, fontSize: 11, fontWeight: 800, marginLeft: "auto" }}>{sig.side}</span>
              <span style={{ color: "var(--text-primary)", fontFamily: "var(--font-mono)", fontSize: 12 }}>{(sig.confidence ?? 0).toFixed(2)}</span>
            </div>
            {sig.price != null && (
              <div style={{ marginTop: 5, paddingLeft: 15, color: "var(--text-muted)", fontFamily: "var(--font-mono)", fontSize: 10 }}>
                {fmtPrice(sig.price)}
              </div>
            )}
          </article>
        );
      })}
    </div>
  );
}

function StageStat({ label, value, accent, tone = "neutral", size = "md", children }: {
  label: string; value: string; accent: string;
  tone?: "neutral" | "positive" | "negative" | "warning";
  size?: "sm" | "md" | "lg"; children?: ReactNode;
}) {
  const toneClass = tone === "positive" ? "m3-stage-stat__value--positive" : tone === "negative" ? "m3-stage-stat__value--negative" : tone === "warning" ? "m3-stage-stat__value--warning" : "";
  return (
    <div className="m3-stage-stat" style={{ borderTopColor: accent }}>
      <div className="m3-stage-stat__label">{label}</div>
      <div className={["m3-stage-stat__value", `m3-stage-stat__value--${size}`, toneClass].filter(Boolean).join(" ")}>{value}</div>
      {children ? <div style={{ marginTop: 6, minHeight: 28, display: "flex", alignItems: "end" }}>{children}</div> : null}
    </div>
  );
}

// ── Pre-Live Daily PnL (computed from pre-live closed trades only) ─────────────
function PreLiveDailyPnL({ trades, balance }: { trades: ClosedTrade[]; balance: number }) {
  const rows = useMemo(() => {
    const byDay = new Map<string, { pnl: number; wins: number; losses: number; best: number; worst: number }>();
    for (const t of trades) {
      if (!t.exitTime) continue;
      const d = new Date(typeof t.exitTime === "number" && t.exitTime < 1e12 ? t.exitTime * 1000 : t.exitTime);
      const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
      const row = byDay.get(key) ?? { pnl: 0, wins: 0, losses: 0, best: -Infinity, worst: Infinity };
      row.pnl += t.netPnl ?? 0;
      if ((t.netPnl ?? 0) >= 0) row.wins++; else row.losses++;
      row.best = Math.max(row.best, t.netPnl ?? 0);
      row.worst = Math.min(row.worst, t.netPnl ?? 0);
      byDay.set(key, row);
    }
    return [...byDay.entries()]
      .sort((a, b) => b[0].localeCompare(a[0]))
      .map(([date, row]) => ({ date, ...row, best: isFinite(row.best) ? row.best : 0, worst: isFinite(row.worst) ? row.worst : 0 }));
  }, [trades]);

  const fmt = (v: number) => {
    const sign = v > 0 ? "+" : v < 0 ? "-" : "";
    return `${sign}$${Math.abs(v).toFixed(2)}`;
  };
  const fmtPct = (v: number, bal: number) => {
    const pct = bal > 0 ? (v / bal) * 100 : 0;
    return `${pct >= 0 ? "+" : ""}${pct.toFixed(2)}%`;
  };
  const labelDate = (iso: string) => {
    const [, m, d] = iso.split("-");
    const months = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];
    return `${parseInt(d)} ${months[parseInt(m) - 1]}`;
  };

  if (rows.length === 0) return (
    <div style={{ background: "var(--surface-1)", border: "1px solid var(--border)", borderRadius: 12, padding: "20px 22px" }}>
      <div style={{ fontSize: 13, fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--text-muted)", marginBottom: 8 }}>Daily PnL</div>
      <div style={{ color: "var(--text-muted)", fontSize: 13 }}>No closed trades yet — daily breakdown will appear here once trades close.</div>
    </div>
  );

  return (
    <div style={{ background: "var(--surface-1)", border: "1px solid var(--border)", borderRadius: 12, padding: "20px 22px" }}>
      <div style={{ fontSize: 13, fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--text-muted)", marginBottom: 14 }}>
        Daily PnL <span style={{ fontWeight: 400, textTransform: "none", letterSpacing: 0, color: "var(--text-tertiary)", marginLeft: 8 }}>{rows.length} day{rows.length !== 1 ? "s" : ""} traded · pre-live engine only</span>
      </div>
      <div style={{ overflowX: "scroll", overflowY: "visible", paddingBottom: 4, scrollbarWidth: "thin" }}>
        <table style={{ width: "100%", borderCollapse: "separate", borderSpacing: 0, fontSize: 13, minWidth: 700 }}>
          <thead>
            <tr>
              {["Date","PnL ($)","PnL (%)","Trades","W / L","Best / Worst"].map(h => (
                <th key={h} style={{ padding: "10px 12px", fontSize: 11, fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--text-muted)", borderBottom: "1px solid var(--border)", background: "var(--surface-2)", whiteSpace: "nowrap", textAlign: h === "Date" ? "left" : "right" }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map(r => (
              <tr key={r.date}>
                <td style={{ padding: "13px 12px", borderBottom: "1px solid var(--border)", color: "var(--text-secondary)" }}>{labelDate(r.date)}</td>
                <td style={{ padding: "13px 12px", borderBottom: "1px solid var(--border)", textAlign: "right", fontFamily: "var(--font-mono)", color: r.pnl >= 0 ? "var(--green)" : "var(--red)" }}>{fmt(r.pnl)}</td>
                <td style={{ padding: "13px 12px", borderBottom: "1px solid var(--border)", textAlign: "right", fontFamily: "var(--font-mono)", color: r.pnl >= 0 ? "var(--green)" : "var(--red)" }}>{fmtPct(r.pnl, balance)}</td>
                <td style={{ padding: "13px 12px", borderBottom: "1px solid var(--border)", textAlign: "right", color: "var(--text-secondary)" }}>{r.wins + r.losses} trades</td>
                <td style={{ padding: "13px 12px", borderBottom: "1px solid var(--border)", textAlign: "right", fontFamily: "var(--font-mono)" }}>
                  <span style={{ color: "var(--green)" }}>{r.wins}W</span>
                  <span style={{ color: "var(--text-muted)", margin: "0 4px" }}>/</span>
                  <span style={{ color: "var(--red)" }}>{r.losses}L</span>
                </td>
                <td style={{ padding: "13px 12px", borderBottom: "1px solid var(--border)", textAlign: "right", fontFamily: "var(--font-mono)" }}>
                  <span style={{ color: "var(--green)" }}>{fmt(r.best)}</span>
                  <span style={{ color: "var(--text-muted)", margin: "0 4px" }}>/</span>
                  <span style={{ color: "var(--red)" }}>{fmt(r.worst)}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <ScrollHint />
    </div>
  );
}

// ── Main Component ─────────────────────────────────────────────────────────────
export function PreLiveEngineCenter() {
  const live = useLiveBTCPrice();
  const [nowMs, setNowMs] = useState(() => Date.now());
  const [positions, setPositions] = useState<OpenPosition[]>([]);
  const [trades, setTrades] = useState<ClosedTrade[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [scalers, setScalers] = useState<ScalersStats>({ regime: "UNKNOWN", recentSignals: [] });
  const [loading, setLoading] = useState(true);
  const inflightRef = useRef(false);

  useEffect(() => {
    const timer = window.setInterval(() => setNowMs(Date.now()), 60_000);
    return () => window.clearInterval(timer);
  }, []);

  const fetchAll = useCallback(async () => {
    if (inflightRef.current) return;
    inflightRef.current = true;
    try {
      const [pos, tr, st] = await Promise.all([
        fetchJson<OpenPosition[]>("/api/positions"),
        fetchJson<ClosedTrade[]>("/api/trades"),
        fetchJson<Stats>("/api/stats"),
      ]);
      if (pos) setPositions(pos);
      if (tr) setTrades(tr);
      if (st) setStats(st);
    } finally {
      inflightRef.current = false;
      setLoading(false);
    }
  }, []);

  const fetchScalers = useCallback(async () => {
    const data = await fetchJson<ScalersStats>("/api/scalers/stats");
    if (data) setScalers(data);
  }, []);

  const [resetting, setResetting] = useState(false);
  const resetEngine = useCallback(async () => {
    if (!window.confirm("Reset the pre-live engine? This will close all positions and clear all trade history.")) return;
    setResetting(true);
    try {
      const r = await fetch("/api/pre-live/api/admin/reset", { method: "POST", cache: "no-store" });
      if (!r.ok) { alert("Reset failed: " + (await r.text())); return; }
      setPositions([]);
      setTrades([]);
      setStats(null);
      await fetchAll();
    } finally {
      setResetting(false);
    }
  }, [fetchAll]);

  useEffect(() => {
    void fetchAll();
    const id = window.setInterval(() => void fetchAll(), POLL_MS);
    return () => window.clearInterval(id);
  }, [fetchAll]);

  useEffect(() => {
    void fetchScalers();
    const id = window.setInterval(() => void fetchScalers(), SIGNALS_POLL_MS);
    return () => window.clearInterval(id);
  }, [fetchScalers]);

  const openTrades = positions;
  const closedTrades = trades.filter((t) => t.exitTime);

  const initialBalance = stats?.initialBalance ?? 100;
  const realizedPnl = stats?.aggregate?.realizedPnl ?? closedTrades.reduce((s, t) => s + (t.netPnl ?? 0), 0);

  // Compute unrealized PnL from live positions × current price × leverage (journal never stores this)
  const unrealizedPnl = useMemo(() => {
    if (!Number.isFinite(live.price) || live.price <= 0 || openTrades.length === 0) return 0;
    return openTrades.reduce((sum, p) => {
      if (!Number.isFinite(p.size) || !Number.isFinite(p.entryPrice)) return sum;
      const isLong = p.side === "BUY" || p.side === "LONG";
      const diff = isLong ? live.price - p.entryPrice : p.entryPrice - live.price;
      return sum + diff * p.size * LEVERAGE;
    }, 0);
  }, [openTrades, live.price]);

  // Equity = starting margin + leveraged realized PnL + leveraged unrealized PnL
  const equity = initialBalance + realizedPnl + unrealizedPnl;
  const totalTrades = stats?.aggregate?.totalTrades ?? closedTrades.length;
  const winRate = stats?.aggregate?.winRate ?? 0;

  const equityHistory = useMemo(() => {
    let eq = initialBalance;
    const vals = [eq];
    for (const t of [...closedTrades].sort((a, b) => toMs(a.exitTime) - toMs(b.exitTime))) {
      eq += t.netPnl ?? 0;
      vals.push(eq);
    }
    return vals;
  }, [closedTrades, initialBalance]);

  const winRatePct = winRate <= 1 ? winRate * 100 : winRate;
  const priceReady = Number.isFinite(live.price) && live.price > 0;

  const summaryStrip = (
    <section aria-label="Pre-Live Engine summary" style={{
      overflowX: "auto",
      border: "1px solid var(--card-border, var(--border))",
      borderRadius: "var(--radius-card)",
      background: "var(--card-bg, var(--surface))",
      boxShadow: "var(--shadow-card)",
      padding: "18px",
    }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 14 }}>
        <div style={{ display: "flex", alignItems: "center", flexWrap: "wrap", gap: 10 }}>
          <h1 style={{ margin: 0, color: "var(--text-primary)", fontSize: 20, fontWeight: 800, letterSpacing: "-0.03em", whiteSpace: "nowrap" }}>
            Pre-Live Trade Engine
          </h1>
          <RegimeBadge regime={scalers.regime} />
          <span style={{
            display: "inline-flex", alignItems: "center", gap: 6, padding: "4px 10px",
            borderRadius: 999, background: "color-mix(in srgb, var(--gold) 15%, transparent)",
            border: "1px solid var(--gold)", fontSize: 12, fontWeight: 700,
            letterSpacing: "0.06em", color: "var(--gold)", textTransform: "uppercase",
          }}>
            REAL MONEY · {PRE_LIVE_STRATEGIES} STRATEGIES · NO LIMITS
          </span>
          <span style={{
            display: "inline-flex", alignItems: "center", gap: 4, padding: "4px 10px",
            borderRadius: 999, background: "color-mix(in srgb, var(--red, #ef4444) 15%, transparent)",
            border: "1px solid var(--red, #ef4444)", fontSize: 12, fontWeight: 800,
            letterSpacing: "0.06em", color: "var(--red, #ef4444)", textTransform: "uppercase",
          }}>
            {LEVERAGE}× LEVERAGE
          </span>
        </div>
        <button
          type="button"
          onClick={() => void resetEngine()}
          disabled={resetting}
          style={{
            display: "inline-flex", alignItems: "center", gap: 6, padding: "6px 14px",
            borderRadius: 8, border: "1px solid var(--border)", background: "var(--surface-2)",
            color: "var(--text-secondary)", fontSize: 12, fontWeight: 600, cursor: resetting ? "not-allowed" : "pointer",
            opacity: resetting ? 0.6 : 1, whiteSpace: "nowrap",
          }}
        >
          {resetting ? "Resetting…" : "↺ Reset Engine"}
        </button>
      </div>

      {loading ? (
        <div className="m3-kpi-strip" style={{ gridTemplateColumns: "repeat(8, minmax(150px, 1fr))", minWidth: 1100 }}>
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="m3-stage-stat" style={{ borderTopColor: "var(--border)" }}>
              <SkeletonBlock width="56%" height={10} rounded={3} />
              <div style={{ marginTop: 9 }}><SkeletonBlock width={82} height={24} rounded={4} /></div>
            </div>
          ))}
        </div>
      ) : (
        <div className="m3-kpi-strip" style={{ gridTemplateColumns: "repeat(8, minmax(150px, 1fr))", minWidth: 1100 }}>
          <StageStat label="BTC Mark" value={priceReady ? fmtPrice(live.price) : "$0.00"} accent={priceReady ? "var(--green)" : "var(--amber)"} tone={priceReady ? "positive" : "warning"} size="lg" />
          <StageStat label="Equity" value={fmtUsd(equity)} accent="var(--gold)" size="lg">
            <Sparkline data={equityHistory} width={88} height={28}
              color={equityHistory.length < 2 || equityHistory[equityHistory.length - 1] >= equityHistory[0] ? "var(--green)" : "var(--red)"} />
          </StageStat>
          <StageStat label="Open Positions" value={String(openTrades.length)} accent={openTrades.length > 0 ? "var(--amber)" : "var(--border)"} tone={openTrades.length > 0 ? "warning" : "neutral"} size="md" />
          <StageStat label="Closed Trades" value={String(totalTrades)} accent="var(--border)" size="sm" />
          <StageStat label="Win Rate" value={fmtPct(winRatePct)} accent={winRatePct > 50 ? "var(--green)" : totalTrades > 0 ? "var(--amber)" : "var(--border)"} tone={winRatePct > 50 ? "positive" : totalTrades > 0 ? "warning" : "neutral"} size="md" />
          <StageStat label="Realized PnL" value={fmtUsd(realizedPnl)} accent={realizedPnl >= 0 ? "var(--green)" : "var(--red)"} tone={realizedPnl >= 0 ? "positive" : "negative"} size="md" />
          <StageStat label="Unrealized PnL" value={fmtUsd(unrealizedPnl)} accent={unrealizedPnl >= 0 ? "var(--green)" : "var(--red)"} tone={unrealizedPnl >= 0 ? "positive" : "negative"} size="md" />
          <StageStat label="Strategies Active" value={`${PRE_LIVE_STRATEGIES} / ${PRE_LIVE_STRATEGIES}`} accent="var(--gold)" tone="positive" size="sm" />
        </div>
      )}
    </section>
  );

  const leftPanel = (
    <>
      <TerminalCard title="Open Positions" subtitle={`${openTrades.length.toLocaleString()} open · no position limit · TP/SL live`}>
        <OpenPositionsTable positions={openTrades} nowMs={nowMs} markPrice={live.price} />
      </TerminalCard>
      <TerminalCard title="Closed Trades" subtitle={`${closedTrades.length.toLocaleString()} closed · newest first`}>
        <ClosedTradesTable trades={closedTrades} nowMs={nowMs} />
      </TerminalCard>
    </>
  );

  const rightPanel = (
    <>
      <TerminalCard title="Live Signals" subtitle="Pre-live feed · last 10 signals · 3s refresh">
        <LiveSignalsPanel scalers={scalers} />
      </TerminalCard>
      <TerminalCard title="Engine Info" subtitle="100 backtested-qualified strategies · real broker">
        <div style={{ display: "grid", gap: 10, fontSize: 13 }}>
          {[
            { label: "Strategy Pool", value: `${PRE_LIVE_STRATEGIES} qualified strategies` },
            { label: "Leverage", value: `${LEVERAGE}× (Delta Exchange futures)` },
            { label: "Qualifying Criteria", value: "Sharpe≥1.0 · WR≥45% · DD≤20% · PF≥1.30 · Trades≥50" },
            { label: "Position Limit", value: "Unlimited (all 100 fire in parallel)" },
            { label: "Cooldown", value: "None — all strategies evaluate every tick" },
            { label: "Risk Gate", value: "Strategy SL/TP only (no circuit breaker)" },
            { label: "Execution", value: "Delta Exchange (real money)" },
            { label: "Price Feed", value: "Coinbase WS + Binance Klines (15m/1h)" },
          ].map(({ label, value }) => (
            <div key={label} style={{ display: "flex", justifyContent: "space-between", gap: 8, padding: "8px 0", borderBottom: "1px solid var(--border-subtle, var(--border))" }}>
              <span style={{ color: "var(--text-muted)", fontWeight: 600 }}>{label}</span>
              <span style={{ color: "var(--text-primary)", textAlign: "right", maxWidth: "60%" }}>{value}</span>
            </div>
          ))}
        </div>
      </TerminalCard>
    </>
  );

  return (
    <div className="google-page" style={{ gap: 16 }}>
      <PageHeader title="Pre-Live Trade Engine" subtitle="100 Backtested-Qualified Strategies — Real Broker, No Limits" />
      {summaryStrip}
      <div className="icc-trade-engine-grid" style={{ display: "grid", gridTemplateColumns: "3fr 2fr", gap: 16, minHeight: 0 }}>
        <div style={{ display: "grid", alignContent: "start", gap: 16, minHeight: 0 }}>{leftPanel}</div>
        <div style={{ display: "grid", alignContent: "start", gap: 16, minHeight: 0 }}>{rightPanel}</div>
      </div>
      <PreLiveDailyPnL trades={trades} balance={initialBalance} />
    </div>
  );
}
