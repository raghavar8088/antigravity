"use client";

/**
 * Crypto Positions — the F&O Positions desk, rebuilt for Delta Exchange.
 *
 * Live option chains and perpetuals, buy or sell with real quoted premiums and
 * hedge-aware portfolio margin, across paper accounts that each carry their own
 * editable balance. Paper money, real prices, no broker path.
 *
 * The tabs mirror the Indian desk one for one, with a single honest
 * substitution: its Futures tab becomes PERPETUALS, because Delta India lists
 * no dated futures at all. A perpetual has no expiry and pays funding, so that
 * column shows a funding rate where the original shows an expiry date.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AutoSortTable,
  DeskAppBar,
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskEmptyState,
  DeskMetricTile,
  DeskSectionHeader,
  DeskShell,
  DeskTabs,
  type DeskColumn,
} from "@/components/desk/ui";
import type {
  Account,
  BasketPreview,
  Instrument,
  LivePosition,
  OptionChain,
  Order,
  PositionsSummary,
  TopMover,
  TransactionType,
} from "@/lib/cryptoPositions/types";
import { formatExpiry } from "@/lib/cryptoPositions/types";

type Tab = "chain" | "perps" | "movers" | "positions" | "orders";

type BasketItem = {
  symbol: string;
  displayName: string;
  side: TransactionType;
  lots: number;
};

const API = "/api/crypto-positions";

async function get<T>(path: string): Promise<T> {
  const r = await fetch(`${API}/${path}`, { cache: "no-store" });
  const j = await r.json();
  if (!j.ok) throw new Error(j.error ?? "Request failed");
  return j as T;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const r = await fetch(`${API}/${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const j = await r.json();
  if (!j.ok) throw new Error(j.error ?? "Request failed");
  return j as T;
}

const usd = (n: number | null | undefined, dp = 2) =>
  n === null || n === undefined || !Number.isFinite(n)
    ? "—"
    : `${n < 0 ? "−" : ""}$${Math.abs(n).toLocaleString("en-US", { minimumFractionDigits: dp, maximumFractionDigits: dp })}`;

const pct = (n: number | null | undefined, dp = 2) =>
  n === null || n === undefined || !Number.isFinite(n) ? "—" : `${n >= 0 ? "+" : ""}${n.toFixed(dp)}%`;

const tone = (n: number) => (n > 0 ? "var(--desk-profit)" : n < 0 ? "var(--desk-loss)" : "inherit");

export default function CryptoPositionsPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountId, setAccountId] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>("chain");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [underlyings, setUnderlyings] = useState<{ symbol: string; spot: number | null }[]>([]);
  const [underlying, setUnderlying] = useState<string>("");
  const [expiries, setExpiries] = useState<string[]>([]);
  const [expiry, setExpiry] = useState<string>("");
  const [chain, setChain] = useState<OptionChain | null>(null);
  const [chainLoading, setChainLoading] = useState(false);

  const [perps, setPerps] = useState<Instrument[]>([]);
  const [movers, setMovers] = useState<{ topCalls: TopMover[]; topPuts: TopMover[] } | null>(null);

  const [positions, setPositions] = useState<LivePosition[]>([]);
  const [summary, setSummary] = useState<PositionsSummary | null>(null);
  const [posFilter, setPosFilter] = useState<"OPEN" | "all">("OPEN");
  const [orders, setOrders] = useState<Order[]>([]);

  const [basket, setBasket] = useState<BasketItem[]>([]);
  const [preview, setPreview] = useState<BasketPreview | null>(null);

  const [accountForm, setAccountForm] = useState<null | { mode: "create" | "edit"; name: string; capital: string }>(null);

  const say = useCallback((msg: string) => {
    setNotice(msg);
    setError(null);
  }, []);

  const blame = useCallback((e: unknown) => {
    setError(e instanceof Error ? e.message : String(e));
    setNotice(null);
  }, []);

  /* ── accounts ─────────────────────────────────────────────────────────── */
  const loadAccounts = useCallback(async () => {
    try {
      const d = await get<{ accounts: Account[] }>("accounts");
      setAccounts(d.accounts);
      setAccountId((cur) => cur ?? d.accounts[0]?.accountId ?? null);
    } catch (e) {
      blame(e);
    }
  }, [blame]);

  useEffect(() => {
    loadAccounts();
  }, [loadAccounts]);

  /* ── market reference data ────────────────────────────────────────────── */
  useEffect(() => {
    get<{ underlyings: { symbol: string; spot: number | null }[] }>("underlyings")
      .then((d) => {
        setUnderlyings(d.underlyings);
        setUnderlying((cur) => cur || d.underlyings[0]?.symbol || "");
      })
      .catch(blame);
  }, [blame]);

  useEffect(() => {
    if (!underlying) return;
    get<{ expiries: string[] }>(`options/expiries?underlying=${encodeURIComponent(underlying)}`)
      .then((d) => {
        setExpiries(d.expiries);
        setExpiry((cur) => (cur && d.expiries.includes(cur) ? cur : d.expiries[0] ?? ""));
      })
      .catch(blame);
  }, [underlying, blame]);

  const loadChain = useCallback(async () => {
    if (!underlying || !expiry) return;
    setChainLoading(true);
    try {
      const d = await get<{ chain: OptionChain }>(
        `options/chain?underlying=${encodeURIComponent(underlying)}&expiry=${encodeURIComponent(expiry)}`,
      );
      setChain(d.chain);
    } catch (e) {
      blame(e);
    } finally {
      setChainLoading(false);
    }
  }, [underlying, expiry, blame]);

  useEffect(() => {
    loadChain();
  }, [loadChain]);

  useEffect(() => {
    if (tab === "perps" && perps.length === 0) {
      get<{ perpetuals: Instrument[] }>("perpetuals").then((d) => setPerps(d.perpetuals)).catch(blame);
    }
    if (tab === "movers" && !movers) {
      get<{ topCalls: TopMover[]; topPuts: TopMover[] }>("top-movers?limit=12").then(setMovers).catch(blame);
    }
  }, [tab, perps.length, movers, blame]);

  /* ── book ─────────────────────────────────────────────────────────────── */
  const loadBook = useCallback(async () => {
    if (!accountId) return;
    try {
      const q = posFilter === "all" ? "" : "&status=OPEN";
      const d = await get<{ positions: LivePosition[]; summary: PositionsSummary }>(
        `positions?account_id=${encodeURIComponent(accountId)}${q}`,
      );
      setPositions(d.positions);
      setSummary(d.summary);
    } catch (e) {
      blame(e);
    }
  }, [accountId, posFilter, blame]);

  useEffect(() => {
    loadBook();
    const t = setInterval(loadBook, 20_000);
    return () => clearInterval(t);
  }, [loadBook]);

  useEffect(() => {
    if (tab === "orders" && accountId) {
      get<{ orders: Order[] }>(`orders?account_id=${encodeURIComponent(accountId)}`)
        .then((d) => setOrders(d.orders))
        .catch(blame);
    }
  }, [tab, accountId, blame]);

  /* ── basket ───────────────────────────────────────────────────────────── */
  const addLeg = useCallback((symbol: string, displayName: string, side: TransactionType) => {
    setBasket((b) => {
      const i = b.findIndex((x) => x.symbol === symbol && x.side === side);
      if (i >= 0) {
        const next = [...b];
        next[i] = { ...next[i], lots: next[i].lots + 1 };
        return next;
      }
      return [...b, { symbol, displayName, side, lots: 1 }];
    });
  }, []);

  useEffect(() => {
    if (!accountId || basket.length === 0) {
      setPreview(null);
      return;
    }
    let live = true;
    post<{ preview: BasketPreview }>("basket/preview", {
      account_id: accountId,
      legs: basket.map((b) => ({ symbol: b.symbol, transactionType: b.side, lots: b.lots })),
    })
      .then((d) => {
        if (live) setPreview(d.preview);
      })
      .catch((e) => {
        if (live) {
          setPreview(null);
          blame(e);
        }
      });
    return () => {
      live = false;
    };
  }, [accountId, basket, blame]);

  const execute = useCallback(async () => {
    if (!accountId || basket.length === 0) return;
    setBusy(true);
    try {
      const r = await post<{ filled: number; marginAdded: number; netPremium: number; feesUsd: number }>(
        "basket/execute",
        {
          account_id: accountId,
          legs: basket.map((b) => ({ symbol: b.symbol, transactionType: b.side, lots: b.lots })),
        },
      );
      say(
        `Filled ${r.filled} leg${r.filled === 1 ? "" : "s"} — margin blocked ${usd(r.marginAdded)}, ` +
          `net premium ${usd(Math.abs(r.netPremium))} ${r.netPremium >= 0 ? "credit" : "debit"}, fees ${usd(r.feesUsd)}`,
      );
      setBasket([]);
      setTab("positions");
      await loadBook();
    } catch (e) {
      blame(e);
    } finally {
      setBusy(false);
    }
  }, [accountId, basket, say, blame, loadBook]);

  const exit = useCallback(
    async (p: LivePosition) => {
      if (!accountId) return;
      setBusy(true);
      try {
        const r = await post<{ realizedPnl: number; fillPrice: number }>("positions/exit", {
          account_id: accountId,
          position_id: p.positionId,
        });
        say(`Exited ${p.displayName} at ${usd(r.fillPrice, 4)} — realized ${usd(r.realizedPnl)}`);
        await loadBook();
      } catch (e) {
        blame(e);
      } finally {
        setBusy(false);
      }
    },
    [accountId, say, blame, loadBook],
  );

  const reset = useCallback(async () => {
    if (!accountId) return;
    setBusy(true);
    try {
      const r = await post<{ positionsCleared: number; ordersCleared: number; archived: number }>("reset", {
        account_id: accountId,
      });
      say(
        `Cleared ${r.positionsCleared} position(s) and ${r.ordersCleared} order(s). ` +
          `${r.archived} record(s) archived — this reset is recoverable.`,
      );
      await loadBook();
    } catch (e) {
      blame(e);
    } finally {
      setBusy(false);
    }
  }, [accountId, say, blame, loadBook]);

  const submitAccount = useCallback(async () => {
    if (!accountForm) return;
    setBusy(true);
    try {
      const capital = Number(accountForm.capital);
      if (accountForm.mode === "create") {
        const d = await post<{ account: Account }>("accounts", {
          name: accountForm.name,
          initial_capital: Number.isFinite(capital) && capital > 0 ? capital : undefined,
        });
        setAccountId(d.account.accountId);
        say(`Created ${d.account.name} with ${usd(d.account.initialCapital)}`);
      } else {
        await post("accounts/edit", {
          account_id: accountId,
          name: accountForm.name,
          initial_capital: Number.isFinite(capital) && capital > 0 ? capital : undefined,
        });
        say("Account updated.");
      }
      setAccountForm(null);
      await loadAccounts();
      await loadBook();
    } catch (e) {
      blame(e);
    } finally {
      setBusy(false);
    }
  }, [accountForm, accountId, say, blame, loadAccounts, loadBook]);

  const active = useMemo(() => accounts.find((a) => a.accountId === accountId) ?? null, [accounts, accountId]);

  /* ── columns ──────────────────────────────────────────────────────────── */
  const positionCols: DeskColumn<LivePosition>[] = [
    { id: "name", header: "Contract", cell: (p) => <strong>{p.displayName}</strong> },
    {
      id: "side",
      header: "Side",
      cell: (p) => (
        <DeskChip label={p.side} tone={p.side === "BUY" ? "success" : "danger"} />
      ),
    },
    { id: "lots", header: "Lots", align: "right", cell: (p) => p.lots },
    { id: "entry", header: "Entry", align: "right", cell: (p) => usd(p.entryPrice, 4) },
    {
      id: "mark",
      header: "Mark",
      align: "right",
      cell: (p) => (p.status === "OPEN" ? usd(p.markPrice, 4) : usd(p.exitPrice, 4)),
    },
    {
      id: "pnl",
      header: "P&L",
      align: "right",
      sortValue: (p) => (p.status === "OPEN" ? p.unrealizedPnl : p.realizedPnl),
      cell: (p) => {
        const v = p.status === "OPEN" ? p.unrealizedPnl : p.realizedPnl;
        return <span style={{ color: tone(v), fontWeight: 600 }}>{usd(v)}</span>;
      },
    },
    { id: "fees", header: "Fees", align: "right", cell: (p) => usd(p.feesUsd) },
    {
      id: "status",
      header: "",
      align: "right",
      cell: (p) =>
        p.status === "OPEN" ? (
          <DeskButton variant="outlined" onClick={() => exit(p)} disabled={busy}>
            Exit
          </DeskButton>
        ) : (
          <DeskChip label="CLOSED" tone="default" />
        ),
    },
  ];

  const orderCols: DeskColumn<Order>[] = [
    {
      id: "time",
      header: "Time",
      sortValue: (o) => o.createdAt,
      cell: (o) => new Date(o.createdAt).toLocaleString(),
    },
    { id: "name", header: "Contract", cell: (o) => o.displayName },
    { id: "intent", header: "Intent", cell: (o) => <DeskChip label={o.intent} tone="default" /> },
    {
      id: "side",
      header: "Side",
      cell: (o) => <DeskChip label={o.transactionType} tone={o.transactionType === "BUY" ? "success" : "danger"} />,
    },
    { id: "lots", header: "Lots", align: "right", cell: (o) => o.lots },
    { id: "fill", header: "Fill", align: "right", cell: (o) => usd(o.fillPrice, 4) },
    {
      id: "status",
      header: "Status",
      cell: (o) => (
        <span title={o.rejectReason ?? undefined}>
          <DeskChip label={o.status} tone={o.status === "FILLED" ? "success" : "danger"} />
        </span>
      ),
    },
    { id: "why", header: "Note", cell: (o) => <span style={{ opacity: 0.7 }}>{o.rejectReason ?? "—"}</span> },
  ];

  const perpCols: DeskColumn<Instrument>[] = [
    { id: "sym", header: "Contract", cell: (i) => <strong>{i.symbol}</strong> },
    { id: "mark", header: "Mark", align: "right", cell: (i) => usd(i.markPrice, 4) },
    {
      id: "chg",
      header: "24h",
      align: "right",
      sortValue: (i) => i.change24hPct ?? 0,
      cell: (i) => <span style={{ color: tone(i.change24hPct ?? 0) }}>{pct(i.change24hPct)}</span>,
    },
    {
      id: "funding",
      header: "Funding / 8h",
      align: "right",
      sortValue: (i) => i.fundingRatePct8h ?? 0,
      cell: (i) => (i.fundingRatePct8h === null ? "—" : `${i.fundingRatePct8h.toFixed(4)}%`),
    },
    {
      id: "turnover",
      header: "24h turnover",
      align: "right",
      sortValue: (i) => i.turnoverUsd ?? 0,
      cell: (i) => usd(i.turnoverUsd, 0),
    },
    {
      id: "act",
      header: "",
      align: "right",
      cell: (i) => (
        <span style={{ display: "inline-flex", gap: 6 }}>
          <DeskButton variant="tonal" onClick={() => addLeg(i.symbol, `${i.symbol} PERP`, "BUY")}>
            Buy
          </DeskButton>
          <DeskButton variant="outlined" onClick={() => addLeg(i.symbol, `${i.symbol} PERP`, "SELL")}>
            Sell
          </DeskButton>
        </span>
      ),
    },
  ];

  const moverCols: DeskColumn<TopMover>[] = [
    { id: "name", header: "Contract", cell: (m) => <strong>{m.displayName}</strong> },
    { id: "mark", header: "Mark", align: "right", cell: (m) => usd(m.markPrice, 4) },
    {
      id: "chg",
      header: "24h",
      align: "right",
      sortValue: (m) => m.change24hPct ?? 0,
      cell: (m) => <span style={{ color: tone(m.change24hPct ?? 0) }}>{pct(m.change24hPct)}</span>,
    },
    { id: "oi", header: "OI", align: "right", sortValue: (m) => m.openInterest ?? 0, cell: (m) => (m.openInterest ?? 0).toLocaleString() },
    { id: "to", header: "Turnover", align: "right", sortValue: (m) => m.turnoverUsd ?? 0, cell: (m) => usd(m.turnoverUsd, 0) },
    {
      id: "act",
      header: "",
      align: "right",
      cell: (m) => (
        <span style={{ display: "inline-flex", gap: 6 }}>
          <DeskButton variant="tonal" onClick={() => addLeg(m.symbol, m.displayName, "BUY")}>
            Buy
          </DeskButton>
          <DeskButton variant="outlined" onClick={() => addLeg(m.symbol, m.displayName, "SELL")}>
            Sell
          </DeskButton>
        </span>
      ),
    },
  ];

  return (
    <DeskShell
      loading={busy || chainLoading}
      appBar={
        <DeskAppBar
          title="Crypto Positions"
          // "live" describes the PRICES, which are Delta's own and current.
          // The money is paper, which the footer and the balance label say
          // plainly — a status badge is about the data feed, not the capital.
          status={chain || perps.length > 0 ? "live" : "syncing"}
          subtitle="Live Delta option chains and perpetuals — buy or sell with real quoted premiums and hedge-aware portfolio margin, across paper accounts with their own editable balance. Not investment advice."
          equity={summary?.equity}
          equityCurrency="USD"
          equityLabel="Paper equity"
          equityDetail={summary ? `${pct(summary.roiPct)} on ${usd(summary.initialCapital, 0)}` : undefined}
        />
      }
    >
      {/* accounts */}
      <DeskCard padding="md">
        <DeskSectionHeader
          title="Accounts"
          subtitle={`${accounts.length} book${accounts.length === 1 ? "" : "s"} · settles in USD`}
          actions={
            <span style={{ display: "inline-flex", gap: 8, flexWrap: "wrap" }}>
              <select
                value={accountId ?? ""}
                onChange={(e) => setAccountId(e.target.value)}
                style={{ padding: "6px 10px", borderRadius: 8 }}
              >
                {accounts.map((a) => (
                  <option key={a.accountId} value={a.accountId}>
                    {a.name} · {usd(a.initialCapital, 0)}
                  </option>
                ))}
              </select>
              <DeskButton variant="outlined" onClick={loadBook} disabled={busy}>
                Refresh
              </DeskButton>
              <DeskButton
                variant="tonal"
                onClick={() => setAccountForm({ mode: "create", name: "", capital: "100000" })}
              >
                + New Account
              </DeskButton>
              <DeskButton
                variant="outlined"
                disabled={!active}
                onClick={() =>
                  active &&
                  setAccountForm({ mode: "edit", name: active.name, capital: String(active.initialCapital) })
                }
              >
                Edit
              </DeskButton>
              <DeskButton variant="danger-tonal" onClick={reset} disabled={busy || !accountId}>
                Reset
              </DeskButton>
            </span>
          }
        />

        {accountForm && (
          <DeskCard variant="outlined" padding="md" style={{ marginTop: 12 }}>
            <div style={{ display: "flex", gap: 12, flexWrap: "wrap", alignItems: "flex-end" }}>
              <label style={{ display: "grid", gap: 4 }}>
                <span style={{ fontSize: 12, opacity: 0.7 }}>Name</span>
                <input
                  value={accountForm.name}
                  onChange={(e) => setAccountForm({ ...accountForm, name: e.target.value })}
                  style={{ padding: "6px 10px", borderRadius: 8 }}
                />
              </label>
              <label style={{ display: "grid", gap: 4 }}>
                <span style={{ fontSize: 12, opacity: 0.7 }}>Balance (USD)</span>
                <input
                  value={accountForm.capital}
                  inputMode="decimal"
                  onChange={(e) => setAccountForm({ ...accountForm, capital: e.target.value })}
                  style={{ padding: "6px 10px", borderRadius: 8 }}
                />
              </label>
              <DeskButton variant="filled" onClick={submitAccount} disabled={busy}>
                {accountForm.mode === "create" ? "Create" : "Save"}
              </DeskButton>
              <DeskButton variant="text" onClick={() => setAccountForm(null)}>
                Cancel
              </DeskButton>
            </div>
            <p style={{ marginTop: 8, fontSize: 12, opacity: 0.7 }}>
              Changing the balance moves the base capital only. Open positions and realized P&amp;L are untouched, so
              ROI is restated against the new base rather than recalculated from it.
            </p>
          </DeskCard>
        )}
      </DeskCard>

      {/* summary tiles */}
      {summary && (
        <div
          style={{
            display: "grid",
            gap: 12,
            gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))",
            marginTop: 12,
          }}
        >
          <DeskMetricTile label="Equity" value={usd(summary.equity)} sub={pct(summary.roiPct)} subColor={summary.roiPct >= 0 ? "profit" : "loss"} />
          <DeskMetricTile label="Realized" value={usd(summary.realizedPnl)} subColor={summary.realizedPnl >= 0 ? "profit" : "loss"} />
          <DeskMetricTile label="Unrealized" value={usd(summary.unrealizedPnl)} subColor={summary.unrealizedPnl >= 0 ? "profit" : "loss"} />
          <DeskMetricTile
            label="Win %"
            value={summary.winPct === null ? "—" : `${summary.winPct.toFixed(1)}%`}
            sub={`${summary.closedPositions} closed`}
          />
          <DeskMetricTile label="Margin deployed" value={usd(summary.deployedMargin)} sub="netted across the book" />
          {summary.marginBenefit > 0 && (
            <DeskMetricTile label="Hedge benefit" value={`−${usd(summary.marginBenefit)}`} subColor="profit" sub="saved by offsets" />
          )}
          <DeskMetricTile label="Available cash" value={usd(summary.availableCash)} />
          <DeskMetricTile label="Fees paid" value={usd(summary.totalFeesUsd)} />
        </div>
      )}

      {error && (
        <div style={{ marginTop: 12 }}>
          <DeskBanner variant="error" title="That did not go through">
            {error}
          </DeskBanner>
        </div>
      )}
      {notice && (
        <div style={{ marginTop: 12 }}>
          <DeskBanner variant="success">{notice}</DeskBanner>
        </div>
      )}

      {/* basket */}
      {basket.length > 0 && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader
            title={preview?.label ?? "Basket"}
            subtitle={`${basket.length} leg${basket.length === 1 ? "" : "s"} · margin is charged on what this ADDS to the book, so a hedge can cost less than it would alone`}
            actions={
              <span style={{ display: "inline-flex", gap: 8 }}>
                <DeskButton variant="text" onClick={() => setBasket([])}>
                  Clear
                </DeskButton>
                <DeskButton
                  variant="filled"
                  onClick={execute}
                  disabled={busy || !preview?.affordable}
                >
                  Execute
                </DeskButton>
              </span>
            }
          />
          <div style={{ display: "grid", gap: 8, marginTop: 10 }}>
            {basket.map((b, idx) => (
              <div key={`${b.symbol}-${b.side}`} style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
                <DeskChip label={b.side} tone={b.side === "BUY" ? "success" : "danger"} />
                <span style={{ flex: 1, minWidth: 180 }}>{b.displayName}</span>
                <input
                  value={b.lots}
                  inputMode="numeric"
                  onChange={(e) => {
                    const n = Number(e.target.value);
                    setBasket((cur) => cur.map((x, i) => (i === idx ? { ...x, lots: Number.isFinite(n) && n > 0 ? n : 1 } : x)));
                  }}
                  style={{ width: 70, padding: "4px 8px", borderRadius: 6 }}
                />
                <DeskButton variant="text" onClick={() => setBasket((cur) => cur.filter((_, i) => i !== idx))}>
                  Remove
                </DeskButton>
              </div>
            ))}
          </div>
          {preview && (
            <div
              style={{
                display: "grid",
                gap: 12,
                gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
                marginTop: 12,
              }}
            >
              <DeskMetricTile label="Margin required" value={usd(preview.marginRequired)} />
              <DeskMetricTile
                label="Net premium"
                value={`${usd(Math.abs(preview.netPremium))} ${preview.netPremium >= 0 ? "credit" : "debit"}`}
                subColor={preview.netPremium >= 0 ? "profit" : "loss"}
              />
              <DeskMetricTile label="Worst case" value={usd(preview.worstCaseLossUsd)} sub={`over ±20% spot`} />
              {preview.marginBenefit > 0 && (
                <DeskMetricTile label="Hedge benefit" value={`−${usd(preview.marginBenefit)}`} subColor="profit" />
              )}
              <DeskMetricTile label="Fees" value={usd(preview.feesUsd)} />
              <DeskMetricTile label="Available" value={usd(preview.availableCash)} />
            </div>
          )}
          {preview && !preview.affordable && (
            <div style={{ marginTop: 10 }}>
              <DeskBanner variant="warning" title="Not enough margin">
                This basket needs {usd(preview.marginRequired)} but only {usd(preview.availableCash)} is available.
                Reduce lots, add a hedge that offsets it, or raise the account balance.
              </DeskBanner>
            </div>
          )}
        </DeskCard>
      )}

      <div style={{ marginTop: 12 }}>
        <DeskTabs
          items={[
            { key: "chain", label: "Option Chain" },
            { key: "perps", label: "Perpetuals" },
            { key: "movers", label: "Top Movers" },
            { key: "positions", label: `Positions (${summary?.openPositions ?? 0})` },
            { key: "orders", label: "Orders" },
          ]}
          active={tab}
          onChange={(k) => setTab(k as Tab)}
        />
      </div>

      {/* ── option chain ─────────────────────────────────────────────────── */}
      {tab === "chain" && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader
            title="Option chain"
            subtitle={
              chain
                ? `${chain.rows.length} strikes · spot ${usd(chain.spot)} · click BUY or SELL to build a basket`
                : "Loading the live chain…"
            }
            actions={
              <span style={{ display: "inline-flex", gap: 8 }}>
                <select value={underlying} onChange={(e) => setUnderlying(e.target.value)} style={{ padding: "6px 10px", borderRadius: 8 }}>
                  {underlyings.map((u) => (
                    <option key={u.symbol} value={u.symbol}>
                      {u.symbol}
                    </option>
                  ))}
                </select>
                <select value={expiry} onChange={(e) => setExpiry(e.target.value)} style={{ padding: "6px 10px", borderRadius: 8 }}>
                  {expiries.map((e) => (
                    <option key={e} value={e}>
                      {formatExpiry(e)}
                    </option>
                  ))}
                </select>
              </span>
            }
          />
          {chain && chain.rows.length > 0 ? (
            <div style={{ overflowX: "auto", marginTop: 10 }}>
              <AutoSortTable>
                <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13, minWidth: 900 }}>
                  <thead>
                    <tr>
                      <th colSpan={5} style={{ textAlign: "center", padding: 6, opacity: 0.7 }}>
                        CALLS
                      </th>
                      <th style={{ padding: 6 }}>Strike</th>
                      <th colSpan={5} style={{ textAlign: "center", padding: 6, opacity: 0.7 }}>
                        PUTS
                      </th>
                    </tr>
                    <tr style={{ opacity: 0.7 }}>
                      <th style={{ padding: 4 }}>OI</th>
                      <th style={{ padding: 4 }}>IV</th>
                      <th style={{ padding: 4 }}>Δ</th>
                      <th style={{ padding: 4 }}>Mark</th>
                      <th style={{ padding: 4 }} />
                      <th />
                      <th style={{ padding: 4 }} />
                      <th style={{ padding: 4 }}>Mark</th>
                      <th style={{ padding: 4 }}>Δ</th>
                      <th style={{ padding: 4 }}>IV</th>
                      <th style={{ padding: 4 }}>OI</th>
                    </tr>
                  </thead>
                  <tbody>
                    {chain.rows.map((r) => {
                      const atm = r.strike === chain.atmStrike;
                      return (
                        <tr
                          key={r.strike}
                          style={{
                            borderTop: "1px solid var(--desk-outline-variant)",
                            background: atm ? "var(--desk-primary-container)" : undefined,
                          }}
                        >
                          <td style={{ padding: 4, textAlign: "right" }}>{r.call?.openInterest?.toLocaleString() ?? "—"}</td>
                          <td style={{ padding: 4, textAlign: "right" }}>{r.call?.ivPct ? `${r.call.ivPct.toFixed(1)}%` : "—"}</td>
                          <td style={{ padding: 4, textAlign: "right" }}>{r.call?.greeks ? r.call.greeks.delta.toFixed(3) : "—"}</td>
                          <td style={{ padding: 4, textAlign: "right", fontWeight: 600 }}>{r.call ? usd(r.call.markPrice, 2) : "—"}</td>
                          <td style={{ padding: 4, whiteSpace: "nowrap" }}>
                            {r.call && (
                              <>
                                <DeskButton variant="tonal" onClick={() => addLeg(r.call!.symbol, `${chain.underlying} ${formatExpiry(chain.expiry)} ${r.strike} CALL`, "BUY")}>
                                  B
                                </DeskButton>{" "}
                                <DeskButton variant="outlined" onClick={() => addLeg(r.call!.symbol, `${chain.underlying} ${formatExpiry(chain.expiry)} ${r.strike} CALL`, "SELL")}>
                                  S
                                </DeskButton>
                              </>
                            )}
                          </td>
                          <td style={{ padding: 4, textAlign: "center", fontWeight: 700 }}>{r.strike.toLocaleString()}</td>
                          <td style={{ padding: 4, whiteSpace: "nowrap" }}>
                            {r.put && (
                              <>
                                <DeskButton variant="tonal" onClick={() => addLeg(r.put!.symbol, `${chain.underlying} ${formatExpiry(chain.expiry)} ${r.strike} PUT`, "BUY")}>
                                  B
                                </DeskButton>{" "}
                                <DeskButton variant="outlined" onClick={() => addLeg(r.put!.symbol, `${chain.underlying} ${formatExpiry(chain.expiry)} ${r.strike} PUT`, "SELL")}>
                                  S
                                </DeskButton>
                              </>
                            )}
                          </td>
                          <td style={{ padding: 4, textAlign: "right", fontWeight: 600 }}>{r.put ? usd(r.put.markPrice, 2) : "—"}</td>
                          <td style={{ padding: 4, textAlign: "right" }}>{r.put?.greeks ? r.put.greeks.delta.toFixed(3) : "—"}</td>
                          <td style={{ padding: 4, textAlign: "right" }}>{r.put?.ivPct ? `${r.put.ivPct.toFixed(1)}%` : "—"}</td>
                          <td style={{ padding: 4, textAlign: "right" }}>{r.put?.openInterest?.toLocaleString() ?? "—"}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </AutoSortTable>
            </div>
          ) : (
            <DeskEmptyState title="No chain" subtitle="Waiting for the live Delta option chain." />
          )}
        </DeskCard>
      )}

      {/* ── perpetuals ───────────────────────────────────────────────────── */}
      {tab === "perps" && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader
            title="Perpetuals"
            subtitle={`${perps.length} contracts · Delta India lists no dated futures, so these never expire and pay funding instead`}
          />
          <DeskDataTable
            columns={perpCols}
            rows={perps}
            getRowKey={(i) => i.symbol}
            empty={<DeskEmptyState title="No perpetuals" subtitle="Waiting for the venue." />}
          />
        </DeskCard>
      )}

      {/* ── top movers ───────────────────────────────────────────────────── */}
      {tab === "movers" && (
        <div style={{ display: "grid", gap: 12, marginTop: 12 }}>
          <DeskCard padding="md">
            <DeskSectionHeader title="Top calls" subtitle="Ranked by traded value, not by percent change — a 200% move on a $0.05 option is not a mover" />
            <DeskDataTable columns={moverCols} rows={movers?.topCalls ?? []} getRowKey={(m) => m.symbol} empty={<DeskEmptyState title="Loading" />} />
          </DeskCard>
          <DeskCard padding="md">
            <DeskSectionHeader title="Top puts" subtitle="Ranked by traded value" />
            <DeskDataTable columns={moverCols} rows={movers?.topPuts ?? []} getRowKey={(m) => m.symbol} empty={<DeskEmptyState title="Loading" />} />
          </DeskCard>
        </div>
      )}

      {/* ── positions ────────────────────────────────────────────────────── */}
      {tab === "positions" && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader
            title="Positions"
            subtitle={`${positions.length} in ${active?.name ?? "—"}`}
            actions={
              <span style={{ display: "inline-flex", gap: 6 }}>
                <DeskButton variant={posFilter === "OPEN" ? "filled" : "outlined"} onClick={() => setPosFilter("OPEN")}>
                  Open
                </DeskButton>
                <DeskButton variant={posFilter === "all" ? "filled" : "outlined"} onClick={() => setPosFilter("all")}>
                  All
                </DeskButton>
              </span>
            }
          />
          <DeskDataTable
            columns={positionCols}
            rows={positions}
            getRowKey={(p) => p.positionId}
            empty={<DeskEmptyState title="No positions" subtitle="Build a basket from the chain or the perpetuals list." />}
          />
        </DeskCard>
      )}

      {/* ── orders ───────────────────────────────────────────────────────── */}
      {tab === "orders" && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader
            title="Orders"
            subtitle="Every fill and every rejection, with the reason it was refused"
          />
          <DeskDataTable
            columns={orderCols}
            rows={orders}
            getRowKey={(o) => o.orderId}
            empty={<DeskEmptyState title="No orders yet" subtitle="Nothing has been placed on this account." />}
          />
        </DeskCard>
      )}

      <p style={{ marginTop: 16, fontSize: 12, opacity: 0.65, lineHeight: 1.6 }}>
        Paper money, real prices. This desk holds no API key, signs no request and has no order-routing path — nothing
        here can reach a broker. Fills cross the spread: a buy pays the ask and a sell hits the bid, from Delta&apos;s
        published quotes, and where a contract has no quote the fill says it used the mark. Margin is scenario-based:
        the whole book is revalued across a ±20% shock in spot and the worst loss is what gets blocked, so a bought
        option that caps a sold one genuinely lowers the requirement. That model is ours, not the venue&apos;s — a real
        Delta account would be charged something different.
      </p>
    </DeskShell>
  );
}
