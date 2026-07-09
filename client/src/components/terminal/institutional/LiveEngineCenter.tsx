"use client";

import { type CSSProperties, useCallback, useEffect, useRef, useState } from "react";
import { PageHeader } from "@/components/ui/PageHeader";
import { SkeletonBlock } from "@/components/ui/EmptyState";
import { TerminalCard } from "./TerminalCard";

// ── Constants ─────────────────────────────────────────────────────────────────
const POLL_MS = 5_000;

// ── Types (match Go livemirror JSON) ─────────────────────────────────────────
type WalletEntry = {
  asset: string;
  balance: number;
  availableBalance: number;
  blockedBalance: number;
  unrealisedPnl: number;
};

type LivePosition = {
  symbol: string;
  productId: number;
  size: number;
  entryPrice: number;
  markPrice: number;
  unrealisedPnl: number;
  realisedPnl: number;
  margin: number;
  side: "LONG" | "SHORT";
};

type OpenOrder = {
  orderId: string;
  symbol: string;
  side: string;
  size: number;
  price: number;
  state: string;
};

type LiveStats = {
  configured: boolean;
  testnet: boolean;
  enabled: boolean;
  symbol: string;
  productId?: number;
  contractValue?: number;
  maxContracts: number;
  fixedContracts?: number;
  leverage: number;
  totalTrades: number;
  openTrades: number;
  closedTrades: number;
  failedTrades: number;
  wins: number;
  losses: number;
  realizedPnlUsd: number;
  skippedOpens: number;
  droppedEvents: number;
  walletUsd: number;
  wallets?: WalletEntry[];
  livePositions?: LivePosition[];
  openOrders?: OpenOrder[];
  accountError?: string;
};

type MirrorTrade = {
  id: string;
  paperPositionId: string;
  strategyName: string;
  side: "LONG" | "SHORT";
  symbol: string;
  contracts: number;
  contractValue: number;
  paperSizeBtc: number;
  paperEntryPrice: number;
  paperExitPrice?: number;
  entryOrderId?: string;
  entryPrice?: number;
  exitOrderId?: string;
  exitPrice?: number;
  closeReason?: string;
  status: "PENDING" | "OPEN" | "CLOSED" | "FAILED";
  failureReason?: string;
  openedAt: string;
  closedAt?: string;
  realizedPnlUsd?: number;
};

// ── Styles (match PreLiveEngineCenter idiom) ─────────────────────────────────
const tableStyle: CSSProperties = {
  width: "100%",
  borderCollapse: "separate",
  borderSpacing: 0,
  fontSize: 13,
  minWidth: 860,
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
function fmtUsd(v: number | undefined) {
  if (v == null || !Number.isFinite(v)) return "—";
  const sign = v > 0 ? "+" : v < 0 ? "-" : "";
  return `${sign}$${Math.abs(v).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}
function fmtPrice(v: number | undefined) {
  if (v == null || !Number.isFinite(v) || v === 0) return "—";
  return `$${v.toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}
function pnlColor(v: number | undefined) {
  if (v == null || v === 0) return "var(--text-muted)";
  return v > 0 ? "var(--green)" : "var(--red)";
}
function sideColor(s: string) { return s === "LONG" || s === "buy" ? "var(--green)" : "var(--red)"; }
function fmtTime(ts: string | undefined | null) {
  if (!ts) return "—";
  const ms = Date.parse(ts);
  if (isNaN(ms)) return "—";
  return new Intl.DateTimeFormat("en-IN", {
    month: "short", day: "2-digit",
    hour: "numeric", minute: "2-digit", hour12: true,
    timeZone: "Asia/Kolkata",
  }).format(ms) + " IST";
}

async function fetchJson<T>(path: string, init?: RequestInit): Promise<T | null> {
  try {
    const r = await fetch(`/api/pre-live${path}`, { cache: "no-store", ...init });
    if (!r.ok) return null;
    return r.json() as Promise<T>;
  } catch { return null; }
}

// ── Sub-components ────────────────────────────────────────────────────────────
function EmptyState({ label, hint }: { label: string; hint?: string }) {
  return (
    <div className="google-empty-state" role="status" style={{ minHeight: 72, padding: "16px 16px" }}>
      <p className="m3-empty-state__title">{label}</p>
      <p className="m3-empty-state__subtitle">{hint ?? "Updates automatically as the Live Engine clones pre-live trades."}</p>
    </div>
  );
}

function StatusPill({ on, onLabel, offLabel, tone = "danger" }: { on: boolean; onLabel: string; offLabel: string; tone?: "danger" | "gold" }) {
  const color = on ? (tone === "gold" ? "var(--gold)" : "var(--green)") : "var(--text-muted)";
  return (
    <span style={{
      display: "inline-flex", alignItems: "center", gap: 6, padding: "4px 10px",
      borderRadius: 999, border: `1px solid ${color}`,
      background: `color-mix(in srgb, ${color} 12%, transparent)`,
      fontSize: 12, fontWeight: 700, letterSpacing: "0.06em",
      color, textTransform: "uppercase", whiteSpace: "nowrap",
    }}>
      <span style={{ width: 7, height: 7, borderRadius: 999, background: color }} />
      {on ? onLabel : offLabel}
    </span>
  );
}

function StageStat({ label, value, accent, tone = "neutral", size = "md" }: {
  label: string; value: string; accent: string;
  tone?: "neutral" | "positive" | "negative" | "warning";
  size?: "sm" | "md" | "lg";
}) {
  const toneClass = tone === "positive" ? "m3-stage-stat__value--positive" : tone === "negative" ? "m3-stage-stat__value--negative" : tone === "warning" ? "m3-stage-stat__value--warning" : "";
  return (
    <div className="m3-stage-stat" style={{ borderTopColor: accent }}>
      <div className="m3-stage-stat__label">{label}</div>
      <div className={["m3-stage-stat__value", `m3-stage-stat__value--${size}`, toneClass].filter(Boolean).join(" ")}>{value}</div>
    </div>
  );
}

function DeltaPositionsTable({ positions }: { positions: LivePosition[] }) {
  if (positions.length === 0) return <EmptyState label="No open positions on Delta Exchange" hint="Positions appear here as soon as a pre-live trade is cloned live." />;
  return (
    <div className="pre-live-scroll-table">
      <table style={tableStyle}>
        <thead>
          <tr>
            {["SYMBOL", "SIDE", "CONTRACTS", "ENTRY", "MARK", "UNREALIZED PnL", "MARGIN"].map((h, i) => (
              <th key={h} style={{ ...thStyle, textAlign: i >= 2 ? "right" : "left" }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {positions.map((p, i) => (
            <tr key={`${p.productId}-${i}`} style={{ background: rowBg(i) }}>
              <td style={{ ...tdStyle, color: "var(--text-primary)", fontWeight: 700 }}>{p.symbol}</td>
              <td style={{ ...tdStyle, color: sideColor(p.side), fontWeight: 700 }}>{p.side}</td>
              <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{Math.abs(p.size)}</td>
              <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(p.entryPrice)}</td>
              <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right", color: "var(--text-primary)" }}>{fmtPrice(p.markPrice)}</td>
              <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right", fontWeight: 700, color: pnlColor(p.unrealisedPnl) }}>{fmtUsd(p.unrealisedPnl)}</td>
              <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtUsd(p.margin)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function StatusBadge({ status }: { status: MirrorTrade["status"] }) {
  const map: Record<MirrorTrade["status"], { bg: string; fg: string }> = {
    OPEN: { bg: "var(--green-dim)", fg: "var(--green)" },
    PENDING: { bg: "var(--amber-dim)", fg: "var(--amber)" },
    CLOSED: { bg: "color-mix(in srgb, var(--text-muted) 15%, transparent)", fg: "var(--text-muted)" },
    FAILED: { bg: "var(--red-dim)", fg: "var(--red)" },
  };
  const { bg, fg } = map[status] ?? map.CLOSED;
  return (
    <span style={{ display: "inline-flex", alignItems: "center", borderRadius: 999, background: bg, color: fg, fontSize: 10, fontWeight: 700, letterSpacing: "0.04em", padding: "2px 7px", textTransform: "uppercase" }}>
      {status}
    </span>
  );
}

function MirrorTradesTable({ trades }: { trades: MirrorTrade[] }) {
  const [limit, setLimit] = useState(50);
  if (trades.length === 0) return <EmptyState label="No cloned trades yet" hint="When the Pre-Live Engine fires a trade while the Live Engine is armed, its Delta Exchange clone appears here." />;
  const visible = trades.slice(0, limit);
  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 10, color: "var(--text-muted)", fontSize: 11 }}>
        <span>{trades.length.toLocaleString()} cloned trades</span>
        {trades.length > limit && (
          <button type="button" onClick={() => setLimit((l) => l + 50)}
            style={{ border: 0, background: "transparent", color: "var(--accent)", cursor: "pointer", fontSize: 11, fontWeight: 700, padding: 0, textDecoration: "underline" }}>
            Show more
          </button>
        )}
      </div>
      <div className="pre-live-scroll-table">
        <table style={{ ...tableStyle, minWidth: 1050 }}>
          <thead>
            <tr>
              {["ID", "STRATEGY", "SIDE", "STATUS", "OPENED (IST)", "CONTRACTS", "LIVE ENTRY", "LIVE EXIT", "PnL", "REASON"].map((h, i) => (
                <th key={h} style={{ ...thStyle, textAlign: i >= 5 ? "right" : "left" }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((t, i) => (
              <tr key={t.id} style={{ background: rowBg(i) }}>
                <td style={{ ...tdStyle, ...monoCellStyle, fontSize: 12 }}>{t.id}</td>
                <td style={{ ...tdStyle, maxWidth: 200, color: "var(--text-primary)", fontWeight: 600 }}>
                  <div style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={t.failureReason ?? t.strategyName}>{t.strategyName}</div>
                </td>
                <td style={{ ...tdStyle, color: sideColor(t.side), fontWeight: 700 }}>{t.side}</td>
                <td style={tdStyle}><StatusBadge status={t.status} /></td>
                <td style={{ ...tdStyle, ...monoCellStyle, fontSize: 12 }}>{fmtTime(t.openedAt)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{t.contracts || "—"}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(t.entryPrice)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right" }}>{fmtPrice(t.exitPrice)}</td>
                <td style={{ ...tdStyle, ...monoCellStyle, textAlign: "right", fontWeight: 700, color: pnlColor(t.realizedPnlUsd) }}>{t.status === "CLOSED" ? fmtUsd(t.realizedPnlUsd ?? 0) : "—"}</td>
                <td style={{ ...tdStyle, fontSize: 11, color: t.status === "FAILED" ? "var(--red)" : "var(--text-muted)", maxWidth: 180 }}>
                  <div style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={t.failureReason ?? t.closeReason ?? ""}>
                    {t.status === "FAILED" ? (t.failureReason ?? "FAILED") : (t.closeReason ?? "—")}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ── Main Component ─────────────────────────────────────────────────────────────
export function LiveEngineCenter() {
  const [stats, setStats] = useState<LiveStats | null>(null);
  const [trades, setTrades] = useState<MirrorTrade[]>([]);
  const [loading, setLoading] = useState(true);
  const [engineOffline, setEngineOffline] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [closingAll, setClosingAll] = useState(false);
  const inflightRef = useRef(false);

  const fetchAll = useCallback(async () => {
    if (inflightRef.current) return;
    inflightRef.current = true;
    try {
      const [st, tr] = await Promise.all([
        fetchJson<LiveStats>("/api/live/stats"),
        fetchJson<MirrorTrade[]>("/api/live/trades"),
      ]);
      if (st) setStats(st);
      if (tr) setTrades(tr);
      setEngineOffline(!st);
    } finally {
      inflightRef.current = false;
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchAll();
    const id = window.setInterval(() => void fetchAll(), POLL_MS);
    return () => window.clearInterval(id);
  }, [fetchAll]);

  const toggleEnabled = useCallback(async () => {
    if (!stats) return;
    const arming = !stats.enabled;
    const message = arming
      ? `ARM the Live Engine?\n\nEvery trade the Pre-Live Engine fires will be cloned as a REAL ${stats.symbol} order on Delta Exchange${stats.testnet ? " (TESTNET)" : " with REAL MONEY"}.\n\nMax ${stats.maxContracts} contracts per order · ${stats.leverage}× leverage.`
      : "Disarm the Live Engine? No new live orders will be placed. Open live positions remain open (use Close All to flatten them).";
    if (!window.confirm(message)) return;
    setToggling(true);
    try {
      const res = await fetchJson<{ ok: boolean; enabled: boolean; error?: string }>("/api/live/enable", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ enabled: arming }),
      });
      if (res && !res.ok && res.error) alert(res.error);
      await fetchAll();
    } finally {
      setToggling(false);
    }
  }, [stats, fetchAll]);

  const closeAll = useCallback(async () => {
    if (!window.confirm("Close ALL live positions on Delta Exchange?\n\nThis places reduce-only market orders for every open cloned position AND flattens any residual position on the configured symbol.")) return;
    setClosingAll(true);
    try {
      const res = await fetchJson<Record<string, unknown>>("/api/live/close-all", { method: "POST" });
      if (!res) alert("Close-all failed — engine unreachable");
      await fetchAll();
    } finally {
      setClosingAll(false);
    }
  }, [fetchAll]);

  const configured = stats?.configured ?? false;
  const enabled = stats?.enabled ?? false;
  const openLive = stats?.livePositions ?? [];
  const unrealized = openLive.reduce((s, p) => s + (p.unrealisedPnl ?? 0), 0);

  const actionBtn = (label: string, onClick: () => void, opts: { danger?: boolean; disabled?: boolean; busy?: boolean } = {}) => (
    <button
      type="button"
      onClick={onClick}
      disabled={opts.disabled || opts.busy}
      style={{
        display: "inline-flex", alignItems: "center", gap: 6, padding: "7px 16px",
        borderRadius: 8,
        border: `1px solid ${opts.danger ? "var(--red, #ef4444)" : "var(--green, #22c55e)"}`,
        background: `color-mix(in srgb, ${opts.danger ? "var(--red,#ef4444)" : "var(--green,#22c55e)"} 12%, transparent)`,
        color: opts.danger ? "var(--red, #ef4444)" : "var(--green, #16a34a)",
        fontSize: 12, fontWeight: 700,
        cursor: opts.disabled || opts.busy ? "not-allowed" : "pointer",
        opacity: opts.disabled || opts.busy ? 0.55 : 1, whiteSpace: "nowrap",
        letterSpacing: "0.02em",
      }}
    >
      {opts.busy ? "⟳ " : ""}{label}
    </button>
  );

  const summaryStrip = (
    <section aria-label="Live Engine summary" className="pre-live-scroll-table" style={{
      border: "1px solid var(--card-border, var(--border))",
      borderRadius: "var(--radius-card)",
      background: "var(--card-bg, var(--surface))",
      boxShadow: "var(--shadow-card)",
      padding: "18px",
    }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 14, flexWrap: "wrap" }}>
        <div style={{ display: "flex", alignItems: "center", flexWrap: "wrap", gap: 10 }}>
          <h1 style={{ margin: 0, color: "var(--text-primary)", fontSize: 20, fontWeight: 800, letterSpacing: "-0.03em", whiteSpace: "nowrap" }}>
            Live Engine
          </h1>
          <StatusPill on={enabled} onLabel="ARMED — CLONING LIVE" offLabel="DISARMED" />
          <StatusPill on={configured} onLabel="DELTA CONNECTED" offLabel="DELTA KEYS MISSING" />
          {stats?.testnet ? <StatusPill on tone="gold" onLabel="TESTNET" offLabel="" /> : null}
          <span style={{
            display: "inline-flex", alignItems: "center", gap: 6, padding: "4px 10px",
            borderRadius: 999, background: "color-mix(in srgb, var(--red, #ef4444) 15%, transparent)",
            border: "1px solid var(--red, #ef4444)", fontSize: 12, fontWeight: 800,
            letterSpacing: "0.06em", color: "var(--red, #ef4444)", textTransform: "uppercase",
          }}>
            {stats?.testnet ? "TESTNET ORDERS" : "REAL MONEY"} · {stats?.symbol ?? "BTCUSD"} PERP · {stats?.leverage ?? 10}× LEVERAGE
          </span>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          {actionBtn(
            enabled ? "Disarm Live Engine" : "Arm Live Engine",
            () => void toggleEnabled(),
            { danger: enabled, disabled: !configured, busy: toggling },
          )}
          {actionBtn("Close All Live Positions", () => void closeAll(), { danger: true, disabled: !configured, busy: closingAll })}
        </div>
      </div>

      {loading ? (
        <div className="m3-kpi-strip" style={{ gridTemplateColumns: "repeat(7, minmax(150px, 1fr))", minWidth: 1050 }}>
          {Array.from({ length: 7 }).map((_, i) => (
            <div key={i} className="m3-stage-stat" style={{ borderTopColor: "var(--border)" }}>
              <SkeletonBlock width="56%" height={10} rounded={3} />
              <div style={{ marginTop: 9 }}><SkeletonBlock width={82} height={24} rounded={4} /></div>
            </div>
          ))}
        </div>
      ) : (
        <div className="m3-kpi-strip" style={{ gridTemplateColumns: "repeat(7, minmax(150px, 1fr))", minWidth: 1050 }}>
          <StageStat label="Delta Wallet" value={configured ? fmtUsd(stats?.walletUsd ?? 0).replace("+", "") : "—"} accent="var(--gold)" size="lg" />
          <StageStat label="Live Positions" value={String(openLive.length)} accent={openLive.length > 0 ? "var(--amber)" : "var(--border)"} tone={openLive.length > 0 ? "warning" : "neutral"} size="md" />
          <StageStat label="Unrealized PnL" value={fmtUsd(unrealized)} accent={unrealized >= 0 ? "var(--green)" : "var(--red)"} tone={unrealized >= 0 ? "positive" : "negative"} size="md" />
          <StageStat label="Realized PnL" value={fmtUsd(stats?.realizedPnlUsd ?? 0)} accent={(stats?.realizedPnlUsd ?? 0) >= 0 ? "var(--green)" : "var(--red)"} tone={(stats?.realizedPnlUsd ?? 0) >= 0 ? "positive" : "negative"} size="md" />
          <StageStat label="Cloned Trades" value={String(stats?.totalTrades ?? 0)} accent="var(--border)" size="sm" />
          <StageStat label="W / L" value={`${stats?.wins ?? 0} / ${stats?.losses ?? 0}`} accent={(stats?.wins ?? 0) >= (stats?.losses ?? 0) ? "var(--green)" : "var(--red)"} size="sm" />
          <StageStat label="Failed Orders" value={String(stats?.failedTrades ?? 0)} accent={(stats?.failedTrades ?? 0) > 0 ? "var(--red)" : "var(--border)"} tone={(stats?.failedTrades ?? 0) > 0 ? "negative" : "neutral"} size="sm" />
        </div>
      )}
    </section>
  );

  const infoRows = [
    { label: "Trade Source", value: "Pre-Live Trade Engine (every fired trade is cloned 1:1)" },
    { label: "Broker", value: `Delta Exchange ${stats?.testnet ? "TESTNET" : "India (mainnet)"}` },
    { label: "Instrument", value: `${stats?.symbol ?? "BTCUSD"} perpetual futures${stats?.contractValue ? ` · ${stats.contractValue} BTC/contract` : ""}` },
    { label: "Sizing", value: stats?.fixedContracts ? `Fixed ${stats.fixedContracts} contracts per trade` : `Mirrors pre-live BTC size · capped at ${stats?.maxContracts ?? 5} contracts` },
    { label: "Leverage", value: `${stats?.leverage ?? 10}×` },
    { label: "Entry Orders", value: "Market · placed the moment a pre-live position opens" },
    { label: "Exit Orders", value: "Reduce-only market · placed on pre-live TP/SL/expiry close" },
    { label: "Safety", value: "Kill switch honoured · starts disarmed on every restart" },
  ];

  return (
    <div className="google-page" style={{ gap: 16 }}>
      <PageHeader title="Live Engine" subtitle="Clones Pre-Live Engine trades to Delta Exchange — real orders, same strategies" />
      {engineOffline && !loading ? (
        <div role="alert" style={{
          border: "1px solid var(--red, #ef4444)", borderRadius: 10, padding: "12px 16px",
          background: "color-mix(in srgb, var(--red,#ef4444) 8%, transparent)", color: "var(--red, #ef4444)", fontSize: 13, fontWeight: 600,
        }}>
          Live Engine unreachable — it runs inside the Pre-Live Engine process. Start it with: <code style={{ fontFamily: "var(--font-mono)" }}>cd engine && go run ./cmd/pre_live/main.go</code>
        </div>
      ) : null}
      {!configured && !loading && !engineOffline ? (
        <div role="alert" style={{
          border: "1px solid var(--amber, #f59e0b)", borderRadius: 10, padding: "12px 16px",
          background: "color-mix(in srgb, var(--amber,#f59e0b) 10%, transparent)", color: "var(--amber, #b45309)", fontSize: 13, fontWeight: 600,
        }}>
          Delta Exchange API keys not configured — set <code style={{ fontFamily: "var(--font-mono)" }}>DELTA_API_KEY</code> and <code style={{ fontFamily: "var(--font-mono)" }}>DELTA_API_SECRET</code> in the engine .env, then restart the Pre-Live Engine.
        </div>
      ) : null}
      {stats?.accountError ? (
        <div role="alert" style={{
          border: "1px solid var(--amber, #f59e0b)", borderRadius: 10, padding: "12px 16px",
          background: "color-mix(in srgb, var(--amber,#f59e0b) 10%, transparent)", color: "var(--amber, #b45309)", fontSize: 13,
        }}>
          Delta account fetch error: {stats.accountError}
        </div>
      ) : null}
      {summaryStrip}
      <div className="icc-trade-engine-grid" style={{ display: "grid", gridTemplateColumns: "3fr 2fr", gap: 16, minHeight: 0 }}>
        <div style={{ display: "grid", alignContent: "start", gap: 16, minHeight: 0 }}>
          <TerminalCard title="Live Positions on Delta Exchange" subtitle={`${openLive.length} open · fetched from broker · 5s refresh`}>
            <DeltaPositionsTable positions={openLive} />
          </TerminalCard>
          <TerminalCard title="Cloned Trades" subtitle="Every pre-live trade mirrored to Delta · newest first">
            <MirrorTradesTable trades={trades} />
          </TerminalCard>
        </div>
        <div style={{ display: "grid", alignContent: "start", gap: 16, minHeight: 0 }}>
          <TerminalCard title="How It Works" subtitle="Pre-Live signal → Live order pipeline">
            <div style={{ display: "grid", gap: 10, fontSize: 13 }}>
              {infoRows.map(({ label, value }) => (
                <div key={label} style={{ display: "flex", justifyContent: "space-between", gap: 8, padding: "8px 0", borderBottom: "1px solid var(--border-subtle, var(--border))" }}>
                  <span style={{ color: "var(--text-muted)", fontWeight: 600, whiteSpace: "nowrap" }}>{label}</span>
                  <span style={{ color: "var(--text-primary)", textAlign: "right" }}>{value}</span>
                </div>
              ))}
            </div>
          </TerminalCard>
          <TerminalCard title="Wallet" subtitle="Delta Exchange balances">
            {(stats?.wallets?.length ?? 0) === 0 ? (
              <EmptyState label="No wallet data" hint={configured ? "Waiting for Delta Exchange wallet response." : "Configure Delta API keys to see balances."} />
            ) : (
              <div style={{ display: "grid", gap: 8 }}>
                {stats!.wallets!.map((w) => (
                  <div key={w.asset} style={{ display: "flex", justifyContent: "space-between", gap: 8, padding: "8px 0", borderBottom: "1px solid var(--border-subtle, var(--border))", fontSize: 13 }}>
                    <span style={{ color: "var(--text-primary)", fontWeight: 700 }}>{w.asset}</span>
                    <span style={{ ...monoCellStyle, color: "var(--text-primary)" }}>
                      {w.availableBalance.toLocaleString("en-US", { maximumFractionDigits: 4 })} avail
                      <span style={{ color: "var(--text-muted)" }}> / {w.balance.toLocaleString("en-US", { maximumFractionDigits: 4 })}</span>
                    </span>
                  </div>
                ))}
              </div>
            )}
          </TerminalCard>
        </div>
      </div>
    </div>
  );
}
