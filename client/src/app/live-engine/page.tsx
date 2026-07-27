"use client";

/**
 * Live Engine — REAL-MONEY option-buying control plane.
 *
 * Reads and mutates only through the session-gated /api/live-engine proxy. This
 * module trades real capital on Delta BTC options (long premium only), capped at
 * a $100 server-enforced ceiling. It ships DISARMED; the Delta Engine toggle
 * turns live trading on immediately. Built on the shared desk primitives so it matches
 * the Options Buying desk, but carries persistent, unmissable REAL MONEY
 * differentiation so it can never be mistaken for a paper desk.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskLinearProgress,
  DeskMetricTile,
  DeskSectionHeader,
  DeskSwitch,
  type DeskColumn,
} from "@/components/desk/ui";

const ARM_PHRASE = "ARM LIVE $100";
const CEILING = 100;

type LiveState = {
  state: "ARMED" | "DISARMED";
  armed: boolean;
  armedBy?: string;
  armedAt?: string;
  lastDisarmReason?: string;
  lastDisarmAt?: string;
  consecutiveRejects: number;
  maxConsecutiveRejects: number;
  ceilingUsd: number;
  configured: boolean;
  killSwitchActive: boolean;
  killSwitchReason?: string;
  killSwitchControllable?: boolean;
};

type Account = {
  equityUsd: number;
  tradableUsd: number;
  ceilingUsd: number;
  availableUsd: number;
  marginUsedUsd: number;
  openRiskUsd: number;
  realizedTodayUsd: number;
  distanceToBreakerPct: number;
  source: string;
  asOf: string;
  stale: boolean;
};

type Position = {
  symbol: string;
  side: string;
  size: number;
  entryPrice: number;
  markPrice: number;
  unrealizedPnl: number;
  marginUsd: number;
  takeProfitPrice: number;
  stopLossPrice: number;
  takeProfitUsd: number;
  stopLossUsd: number;
  liquidationPrice: string;
  strategy: string;
};

type ClosedPosition = {
  id: string;
  strategy: string;
  optionType: string;
  symbol: string;
  contracts: number;
  entryPrice: number;
  exitPrice: number;
  realizedPnl: number;
  exitReason: string;
  openedAt: string;
  closedAt?: string;
};

type DailyPnl = {
  date: string;
  capitalUsd: number;
  pnlUsd: number;
  roiPct: number;
  trades: number;
  wins: number;
  winRatePct: number;
};

type Order = {
  id: string;
  strategy: string;
  optionType: string;
  strike: number;
  symbol: string;
  contracts: number;
  side: string;
  premiumUsd: number;
  fillPrice: number;
  status: string;
  deltaOrderId: string;
  openedAt: string;
  rejectReason?: string;
};

type Gate = { name: string; pass: boolean; requirement: string; actual: string };
type Eligibility = { strategy: string; live: boolean; reason: string; gates: Gate[]; allowed: boolean };

type Recon = {
  matched: boolean;
  enginePositions: number;
  deltaPositions: number;
  mismatches?: string[];
  asOf: string;
  error?: string;
};

type AuditEntry = { at: string; actor: string; action: string; reason?: string; detail?: string };

function fmtUSD(v: number | undefined): string {
  if (v === undefined || Number.isNaN(v)) return "—";
  const abs = Math.abs(v);
  // Option premiums here are cents-scale; 2dp would round a real move to $0.00.
  const dp = abs > 0 && abs < 1 ? 4 : 2;
  return `${v < 0 ? "-" : ""}$${abs.toFixed(dp)}`;
}
function pnlTone(v: number): string {
  return v > 0 ? "desk-pnl-positive" : v < 0 ? "desk-pnl-negative" : "desk-pnl-neutral";
}
function ageLabel(iso?: string): string {
  if (!iso) return "no data";
  const ms = Date.now() - new Date(iso).getTime();
  if (Number.isNaN(ms)) return "no data";
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return `${s}s ago`;
  return `${Math.round(s / 60)}m ago`;
}

export default function LiveEnginePage() {
  const [state, setState] = useState<LiveState | null>(null);
  const [account, setAccount] = useState<Account | null>(null);
  const [positions, setPositions] = useState<Position[]>([]);
  const [closed, setClosed] = useState<ClosedPosition[]>([]);
  const [daily, setDaily] = useState<DailyPnl[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [roster, setRoster] = useState<Eligibility[]>([]);
  const [recon, setRecon] = useState<Recon | null>(null);
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [busy, setBusy] = useState<boolean>(false);
  const [actionMsg, setActionMsg] = useState<string>("");


  const refresh = useCallback(async () => {
    try {
      const [st, ac, po, cp, dp, or, ro, rc, au] = await Promise.all([
        fetch("/api/live-engine/state", { cache: "no-store" }),
        fetch("/api/live-engine/account", { cache: "no-store" }),
        fetch("/api/live-engine/positions", { cache: "no-store" }),
        fetch("/api/live-engine/closed-positions", { cache: "no-store" }),
        fetch("/api/live-engine/daily-pnl", { cache: "no-store" }),
        fetch("/api/live-engine/orders", { cache: "no-store" }),
        fetch("/api/live-engine/roster", { cache: "no-store" }),
        fetch("/api/live-engine/reconciliation", { cache: "no-store" }),
        fetch("/api/live-engine/audit", { cache: "no-store" }),
      ]);
      if (!st.ok) {
        setError(`control plane unreachable (HTTP ${st.status})`);
        return;
      }
      setState(await st.json());
      if (ac.ok) setAccount(await ac.json());
      if (po.ok) setPositions((await po.json()) as Position[]);
      if (cp.ok) setClosed((await cp.json()) as ClosedPosition[]);
      if (dp.ok) setDaily((await dp.json()) as DailyPnl[]);
      if (or.ok) setOrders((await or.json()) as Order[]);
      if (ro.ok) setRoster((await ro.json()) as Eligibility[]);
      if (rc.ok) setRecon(await rc.json());
      if (au.ok) setAudit(((await au.json()) as { entries: AuditEntry[] }).entries ?? []);
      setError("");
    } catch {
      setError("control plane unreachable");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), 15_000);
    return () => clearInterval(t);
  }, [refresh]);

  const mutate = useCallback(
    async (action: string, body: Record<string, unknown>) => {
      setBusy(true);
      setActionMsg("");
      try {
        const res = await fetch(`/api/live-engine/${action}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
        const data = (await res.json()) as { ok?: boolean; error?: string; result?: unknown };
        if (!res.ok || data.ok === false) {
          setActionMsg(data.error ?? `HTTP ${res.status}`);
        } else {
          setActionMsg(`${action} ok`);
        }
        await refresh();
      } catch (e) {
        setActionMsg(e instanceof Error ? e.message : "request failed");
      } finally {
        setBusy(false);
      }
    },
    [refresh],
  );

  const armed = state?.armed ?? false;

  const positionColumns: DeskColumn<Position>[] = useMemo(
    () => [
      { id: "sym", header: "Symbol", cell: (p) => p.symbol },
      { id: "side", header: "Side", cell: (p) => <DeskChip tone={p.side.toUpperCase() === "BUY" ? "success" : "default"}>{p.side}</DeskChip> },
      { id: "size", align: "right", header: "Size", cell: (p) => p.size },
      { id: "entry", align: "right", header: "Entry", cell: (p) => p.entryPrice.toFixed(2) },
      {
        id: "tp",
        align: "right",
        header: "TP (+80%)",
        cell: (p) => (
          <span title="Premium level that triggers the take-profit close, and the USD gain if touched">
            {p.takeProfitPrice ? p.takeProfitPrice.toFixed(2) : "—"}
            <span className="desk-pnl-positive" style={{ marginLeft: 6 }}>
              {p.takeProfitUsd ? fmtUSD(p.takeProfitUsd) : ""}
            </span>
          </span>
        ),
      },
      {
        id: "sl",
        align: "right",
        header: "SL (−50%)",
        cell: (p) => (
          <span title="Premium level that triggers the stop-loss close, and the USD loss if touched">
            {p.stopLossPrice ? p.stopLossPrice.toFixed(2) : "—"}
            <span className="desk-pnl-negative" style={{ marginLeft: 6 }}>
              {p.stopLossUsd ? fmtUSD(p.stopLossUsd) : ""}
            </span>
          </span>
        ),
      },
      { id: "mark", align: "right", header: "Mark", cell: (p) => p.markPrice.toFixed(2) },
      { id: "upnl", align: "right", header: "Unrealized", cell: (p) => <span className={pnlTone(p.unrealizedPnl)}>{fmtUSD(p.unrealizedPnl)}</span> },
      { id: "margin", align: "right", header: "Margin", cell: (p) => fmtUSD(p.marginUsd) },
      { id: "liq", align: "right", header: "Liquidation", cell: (p) => <span style={{ color: "var(--desk-on-surface-variant)" }}>{p.liquidationPrice}</span> },
      { id: "strat", header: "Strategy", cell: (p) => p.strategy || "—" },
    ],
    [],
  );

  const dailyColumns: DeskColumn<DailyPnl>[] = useMemo(
    () => [
      { id: "date", header: "Date (UTC)", cell: (d) => d.date },
      { id: "cap", align: "right", header: "Capital used", cell: (d) => fmtUSD(d.capitalUsd) },
      {
        id: "roi", align: "right", header: "ROI",
        cell: (d) => <span className={pnlTone(d.roiPct)} style={{ fontWeight: 600 }}>{d.roiPct.toFixed(1)}%</span>,
      },
      { id: "trades", align: "right", header: "Trades", cell: (d) => d.trades },
      {
        id: "pnl", align: "right", header: "P&L",
        cell: (d) => <span className={pnlTone(d.pnlUsd)} style={{ fontWeight: 600 }}>{fmtUSD(d.pnlUsd)}</span>,
      },
      {
        id: "win", align: "right", header: "Win %",
        cell: (d) => `${d.winRatePct.toFixed(0)}% (${d.wins}/${d.trades})`,
      },
    ],
    [],
  );

  const closedColumns: DeskColumn<ClosedPosition>[] = useMemo(
    () => [
      {
        id: "closedAt",
        header: "Closed (UTC)",
        cell: (c) => (c.closedAt ? new Date(c.closedAt).toISOString().slice(5, 16).replace("T", " ") : "—"),
      },
      { id: "sym", header: "Symbol", cell: (c) => c.symbol || "—" },
      { id: "strat", header: "Strategy", cell: (c) => c.strategy || "—" },
      { id: "ct", align: "right", header: "Contracts", cell: (c) => c.contracts },
      { id: "entry", align: "right", header: "Entry", cell: (c) => (c.entryPrice ? c.entryPrice.toFixed(2) : "—") },
      { id: "exit", align: "right", header: "Exit", cell: (c) => (c.exitPrice ? c.exitPrice.toFixed(2) : "—") },
      {
        id: "why",
        header: "Exit reason",
        cell: (c) => {
          const r = c.exitReason || "";
          const tone = r.includes("take_profit") ? "success" : r.includes("stop_loss") ? "error" : "default";
          return r ? <DeskChip tone={tone}>{r}</DeskChip> : "—";
        },
      },
      {
        id: "pnl",
        align: "right",
        header: "Realized P&L",
        cell: (c) => <span className={pnlTone(c.realizedPnl)} style={{ fontWeight: 600 }}>{fmtUSD(c.realizedPnl)}</span>,
      },
    ],
    [],
  );

  const orderColumns: DeskColumn<Order>[] = useMemo(
    () => [
      { id: "strat", header: "Strategy", cell: (o) => o.strategy || "—" },
      { id: "type", header: "Type", cell: (o) => <DeskChip tone={o.optionType === "CALL" ? "success" : "primary"}>{o.optionType}</DeskChip> },
      { id: "sym", header: "Symbol", cell: (o) => o.symbol || "—" },
      { id: "ct", align: "right", header: "Contracts", cell: (o) => o.contracts },
      { id: "prem", align: "right", header: "Premium", cell: (o) => fmtUSD(o.premiumUsd) },
      {
        id: "status",
        header: "Status",
        cell: (o) => (
          <DeskChip tone={o.status === "OPEN" ? "success" : o.status === "FAILED" ? "error" : "default"}>{o.status}</DeskChip>
        ),
      },
      { id: "reject", header: "Reject reason", cell: (o) => <span style={{ color: "var(--desk-error)" }}>{o.rejectReason ?? ""}</span> },
    ],
    [],
  );

  const rosterColumns: DeskColumn<Eligibility>[] = useMemo(
    () => [
      { id: "strat", header: "Strategy", cell: (e) => e.strategy },
      {
        id: "enabled",
        header: "Live-enabled",
        cell: (e) => (
          <DeskButton
            data-testid={`toggle-${e.strategy}`}
            variant={e.allowed ? "tonal" : "outlined"}
            disabled={busy}
            onClick={() => void mutate("strategy", { strategy: e.strategy, enabled: !e.allowed })}
          >
            {e.allowed ? "ENABLED" : "disabled"}
          </DeskButton>
        ),
      },
      { id: "live", header: "Gate", cell: (e) => <DeskChip tone={e.live ? "success" : "default"}>{e.live ? "LIVE" : "NOT LIVE"}</DeskChip> },
      { id: "reason", header: "Reason", cell: (e) => <span style={{ color: "var(--desk-on-surface-variant)" }}>{e.reason}</span> },
      {
        id: "gates",
        header: "Gates",
        cell: (e) => (
          <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
            {e.gates.map((g) => (
              <DeskChip key={g.name} tone={g.pass ? "success" : "error"} title={`${g.requirement} — have ${g.actual}`}>
                {g.name}
              </DeskChip>
            ))}
          </div>
        ),
      },
    ],
    [busy, mutate],
  );

  const auditColumns: DeskColumn<AuditEntry>[] = useMemo(
    () => [
      { id: "at", header: "Time (UTC)", cell: (a) => new Date(a.at).toISOString().slice(5, 19).replace("T", " ") },
      { id: "actor", header: "Actor", cell: (a) => a.actor },
      { id: "action", header: "Action", cell: (a) => <DeskChip tone={a.action.includes("DISARM") ? "warning" : a.action === "ARM" ? "error" : "default"}>{a.action}</DeskChip> },
      { id: "reason", header: "Reason", cell: (a) => a.reason ?? "" },
      { id: "detail", header: "Detail", cell: (a) => <span style={{ color: "var(--desk-on-surface-variant)" }}>{a.detail ?? ""}</span> },
    ],
    [],
  );

  const reconMismatch = recon ? !recon.matched : false;

  return (
    <div
      style={{ minHeight: "100%", background: "var(--desk-surface-dim)" }}
      data-testid="live-engine-root"
      data-armed={armed ? "true" : "false"}
    >
      <DeskLinearProgress visible={loading} />
      <main className="desk-page">
        {/* Header + persistent REAL MONEY differentiation */}
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: "0.8125rem" }}>
            <Link href="/terminal" className="desk-label-md" style={{ fontWeight: 400, textDecoration: "none" }}>Home</Link>
            <span style={{ color: "var(--desk-outline)" }}>›</span>
            <span className="desk-body-md" style={{ fontWeight: 500 }}>Live Engine</span>
          </div>
          <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12 }}>
            <h1 className="desk-display-lg" style={{ fontSize: "2rem" }}>Live Engine</h1>
            <span
              data-testid="real-money-badge"
              className="desk-mono"
              style={{
                fontWeight: 700,
                fontSize: "0.75rem",
                letterSpacing: "0.06em",
                padding: "4px 10px",
                borderRadius: 6,
                color: "#fff",
                // Green while the Delta Engine is on, red when it is off.
                background: armed ? "var(--desk-success)" : "var(--desk-error)",
              }}
            >
              REAL MONEY · ${CEILING}
            </span>
          </div>
          <p className="desk-body-md" style={{ marginTop: 6, maxWidth: 760, color: "var(--desk-on-surface-variant)" }}>
            Real-money option <strong>buying</strong> on Delta BTC options (long premium only), capped at a
            ${CEILING} server-enforced ceiling. Starts off; turning the Delta Engine on places real orders immediately.
            Naked selling is excluded by decision. 10× is inert on long options — buying pays the premium in full, with
            no borrow and no liquidation price.
          </p>
        </div>

        {error && <DeskBanner variant="warning">{error} — retrying every 15s</DeskBanner>}
        {actionMsg && <DeskBanner variant={actionMsg.endsWith("ok") ? "success" : "error"}>{actionMsg}</DeskBanner>}

        {/* Engine on/off state — visible without scrolling. Green when on. */}
        <DeskBanner
          variant={armed ? "success" : "info"}
          title={armed ? "● DELTA ENGINE ON — LIVE ORDERS ENABLED" : "○ DELTA ENGINE OFF — no live orders"}
        >
          <span data-testid="armed-state">
            {armed
              ? `On since ${ageLabel(state?.armedAt)}. Live orders can be placed against real capital.`
              : `Off${state?.lastDisarmReason ? ` · last reason: ${state.lastDisarmReason}` : ""}. No live orders will be placed.`}
            {state?.killSwitchActive ? " · KILL SWITCH ON" : ""}
            {state && !state.configured ? " · broker not configured" : ""}
          </span>
        </DeskBanner>

        {/* SECTION 1 — Arm / Disarm / Close All */}
        <DeskCard>
          <DeskSectionHeader
            title="Control"
            subtitle="Delta Engine on places real orders immediately. Auto-disarm is one-way."
          />
          <div style={{ display: "flex", flexWrap: "wrap", gap: 24, alignItems: "center", padding: "0 4px 8px" }}>
            {/* Delta Engine — green on, red off. Toggling on goes live at once. */}
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <DeskSwitch
                id="arm-toggle"
                checked={armed}
                disabled={busy}
                ariaLabel="Delta Engine"
                label={armed ? "Delta Engine on" : "Delta Engine off"}
                onColor="var(--desk-success)"
                offColor="var(--desk-error)"
                onChange={(next) => {
                  if (next) {
                    void mutate("arm", { confirmation: ARM_PHRASE });
                  } else {
                    void mutate("disarm", { reason: "Delta Engine turned off from UI" });
                  }
                }}
              />
              <span className="desk-label-md" style={{ color: "var(--desk-on-surface-variant)" }}>
                {armed
                  ? `live orders enabled · ${ageLabel(state?.armedAt)}`
                  : "off — no live orders will be placed"}
              </span>
            </div>

            {/* Kill switch — red on (halted), green off (trading allowed). */}
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <DeskSwitch
                id="kill-switch-toggle"
                checked={state?.killSwitchActive ?? false}
                disabled={busy || state?.killSwitchControllable === false}
                ariaLabel="Kill switch"
                label={state?.killSwitchActive ? "Kill switch on — trading halted" : "Kill switch off — trading allowed"}
                onColor="var(--desk-error)"
                offColor="var(--desk-success)"
                onChange={(next) => void mutate("kill-switch", { active: next })}
              />
              <span className="desk-label-md" style={{ color: state?.killSwitchActive ? "var(--desk-error)" : "var(--desk-on-surface-variant)" }}>
                {state?.killSwitchActive
                  ? (state?.killSwitchReason || "halted — blocks new orders and turns the engine off")
                  : "toggle on to halt all new orders immediately"}
              </span>
            </div>

            <DeskButton data-testid="close-all" variant="outlined" disabled={busy} onClick={() => void mutate("close-all", {})}>
              Panic — CLOSE ALL
            </DeskButton>
            <div style={{ marginLeft: "auto", alignSelf: "center" }} className="desk-label-md">
              Consecutive rejects: {state?.consecutiveRejects ?? 0} / {state?.maxConsecutiveRejects ?? 3} → auto-disarm
            </div>
          </div>
        </DeskCard>

        {/* SECTION 2 — Live account strip */}
        <DeskCard>
          <DeskSectionHeader
            title="Live Account"
            subtitle={account ? `${account.source} · ${ageLabel(account.asOf)}${account.stale ? " · STALE" : ""}` : "—"}
          />
          <div className="desk-metrics-row">
            <DeskMetricTile label="Real Equity" value={account ? fmtUSD(account.equityUsd) : "—"} sub={account?.stale ? "stale — see source" : "delta wallet"} highlight />
            <DeskMetricTile label={`Tradable (≤ $${CEILING})`} value={account ? fmtUSD(account.tradableUsd) : "—"} sub="ceiling enforced server-side" />
            <DeskMetricTile label="Available" value={account ? fmtUSD(account.availableUsd) : "—"} sub="equity − margin" />
            <DeskMetricTile label="Margin Used" value={account ? fmtUSD(account.marginUsedUsd) : "—"} sub="delta positions" />
            <DeskMetricTile label="Open Risk" value={account ? fmtUSD(account.openRiskUsd) : "—"} sub="premium at risk (long)" />
            <DeskMetricTile label="Realized Today" value={account ? fmtUSD(account.realizedTodayUsd) : "—"} valueClassName={account ? pnlTone(account.realizedTodayUsd) : undefined} sub="to daily breaker" />
          </div>
        </DeskCard>

        {/* SECTION 6 (surfaced high) — Reconciliation, shown loudly on mismatch */}
        {reconMismatch ? (
          <DeskBanner variant="error" title="⚠ RECONCILIATION MISMATCH — engine state vs Delta truth">
            <span data-testid="recon-status">
              engine {recon?.enginePositions} open · Delta {recon?.deltaPositions} positions.
              {(recon?.mismatches ?? []).map((m) => ` ${m}.`)}
              {recon?.error ? ` error: ${recon.error}` : ""} An armed engine auto-disarms on this.
            </span>
          </DeskBanner>
        ) : (
          <DeskCard>
            <DeskSectionHeader title="Reconciliation" subtitle={recon ? `${ageLabel(recon.asOf)}` : "—"} />
            <div className="desk-metrics-row">
              <DeskMetricTile compact label="Engine open" value={recon?.enginePositions ?? "—"} />
              <DeskMetricTile compact label="Delta positions" value={recon?.deltaPositions ?? "—"} />
              <DeskMetricTile compact label="Status" value={<span data-testid="recon-status">{recon ? (recon.matched ? "MATCHED" : "MISMATCH") : "—"}</span>} subColor={recon?.matched ? "profit" : "loss"} sub={recon?.error ?? ""} />
            </div>
          </DeskCard>
        )}

        {/* SECTION 3 — Live positions */}
        <DeskCard padding="md">
          <DeskSectionHeader title="Live Positions" subtitle={`${positions.length} open`} />
          <DeskDataTable
            columns={positionColumns}
            rows={positions}
            getRowKey={(p, i) => `${p.symbol}-${i}`}
            stickyHeader
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No live positions.</span>}
          />
        </DeskCard>

        {/* Closed positions — what SL/TP/expiry actually took off, and its result */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Closed Positions"
            subtitle={
              closed.length
                ? `${closed.length} closed · realized ${fmtUSD(closed.reduce((s, c) => s + (c.realizedPnl || 0), 0))}`
                : "closed by take-profit, stop-loss or expiry"
            }
          />
          <DeskDataTable
            columns={closedColumns}
            rows={closed}
            getRowKey={(c) => c.id}
            stickyHeader
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No closed positions yet.</span>}
          />
        </DeskCard>

        {/* Daily P&L — realised results per UTC day */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Daily P&L"
            subtitle={
              daily.length
                ? `${daily.length} day(s) · total realized ${fmtUSD(daily.reduce((s2, d) => s2 + (d.pnlUsd || 0), 0))}`
                : "realised results per UTC day, from closed positions"
            }
          />
          <DeskDataTable
            columns={dailyColumns}
            rows={daily}
            getRowKey={(d) => d.date}
            stickyHeader
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No closed trades yet.</span>}
          />
        </DeskCard>

        {/* SECTION 4 — Orders / fills */}
        <DeskCard padding="md">
          <DeskSectionHeader title="Orders & Fills" subtitle={`${orders.length} recent`} />
          <DeskDataTable
            columns={orderColumns}
            rows={orders.slice(0, 100)}
            getRowKey={(o) => o.id}
            stickyHeader
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No live orders yet.</span>}
          />
        </DeskCard>

        {/* SECTION 5 — Live roster with gate status */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Live Roster"
            subtitle="Long-premium only · toggle Live-enabled to add/remove a strategy from live capital (reversible). Only ENABLED strategies place real orders."
          />
          <DeskDataTable
            columns={rosterColumns}
            rows={roster}
            getRowKey={(e) => e.strategy}
            stickyHeader
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No strategies.</span>}
          />
        </DeskCard>

        {/* Cost transparency — the live-edge question, impossible to overlook */}
        <DeskCard>
          <DeskSectionHeader title="Cost — round-trip fee as % of premium" subtitle="Delta charges 0.03% of notional per side, capped at 10% of premium. This ratio does not improve with account size." />
          <div className="desk-metrics-row">
            <DeskMetricTile compact label="$1.78 premium" value="≈ 2%" sub="round-trip" subColor="profit" />
            <DeskMetricTile compact label="$0.45 premium" value="≈ 9%" sub="round-trip" subColor="neutral" />
            <DeskMetricTile compact label="$0.30 premium" value="≈ 13%" sub="round-trip" subColor="loss" />
            <DeskMetricTile compact label="$0.19 premium" value="≈ 20%" sub="cap binds" subColor="loss" />
          </div>
          <p className="desk-body-md" style={{ padding: "0 4px", color: "var(--desk-on-surface-variant)" }}>
            Per-contract cost against the strategy&apos;s real selected premium is populated from live Delta quotes at the
            testnet stage — the fee % of premium, not the account size, is the live-edge question.
          </p>
        </DeskCard>

        {/* SECTION 7 — Audit log */}
        <DeskCard padding="md">
          <DeskSectionHeader title="Audit Log" subtitle="Every arm, disarm, auto-disarm, close-all and roster change — actor + timestamp" />
          <DeskDataTable
            columns={auditColumns}
            rows={[...audit].reverse().slice(0, 100)}
            getRowKey={(a, i) => `${a.at}-${i}`}
            stickyHeader
            empty={<span style={{ color: "var(--desk-on-surface-variant)" }}>No audit entries yet.</span>}
          />
        </DeskCard>

        <p className="desk-label-md" style={{ textAlign: "center", padding: "8px 0 24px", color: "var(--desk-on-surface-variant)" }}>
          Real money · $100 ceiling enforced in the engine · long premium only · every order passes the risk gate, OMS,
          and kill switch · auto-disarm is one-way
        </p>
      </main>

    </div>
  );
}
