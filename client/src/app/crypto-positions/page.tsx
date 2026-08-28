"use client";

/**
 * Crypto Positions — the F&O / Commodity Positions desk, rebuilt for Delta.
 *
 * Live option chains and perpetuals, buy or sell at real quoted premiums with
 * hedge-aware portfolio margin, across paper accounts that each carry their own
 * editable balance. Paper money, real prices, no broker path.
 *
 * Two honest substitutions from the desks this clones:
 *
 *   FUTURES -> PERPETUALS. Delta India lists no dated futures, so that tab
 *   shows a funding rate where the original shows an expiry. Inventing an
 *   expiry column would be inventing a market.
 *
 *   LOTS ARE CONTRACTS, and one contract is 0.001 BTC while the quote is per
 *   whole BTC. The order ticket states that multiplier on screen for the same
 *   reason the commodity desk states that a ZINC lot is 5 tonnes: it is the
 *   number that decides what a position is actually worth, and inferring it
 *   from a fill that came out the wrong size is how it gets found too late.
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
  ContractSpec,
  Instrument,
  LivePosition,
  OptionChain,
  Order,
  PositionsSummary,
  RollResult,
  TopMover,
  TransactionType,
} from "@/lib/cryptoPositions/types";
import { formatExpiry } from "@/lib/cryptoPositions/types";

type Tab = "chain" | "perps" | "movers" | "positions" | "orders" | "specs" | "history";

type BasketItem = {
  symbol: string;
  displayName: string;
  side: TransactionType;
  lots: number;
  leverage: number | null;
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

const compact = (n: number | null | undefined) =>
  n === null || n === undefined || !Number.isFinite(n)
    ? "—"
    : `${n < 0 ? "−" : ""}$${Math.abs(n).toLocaleString("en-US", { notation: "compact", maximumFractionDigits: 2 })}`;

const pct = (n: number | null | undefined, dp = 2) =>
  n === null || n === undefined || !Number.isFinite(n) ? "—" : `${n >= 0 ? "+" : ""}${n.toFixed(dp)}%`;

const tone = (n: number) => (n > 0 ? "var(--desk-profit)" : n < 0 ? "var(--desk-loss)" : "inherit");

export default function CryptoPositionsPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountId, setAccountId] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>("chain");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const [underlyings, setUnderlyings] = useState<{ symbol: string; spot: number | null }[]>([]);
  const [underlying, setUnderlying] = useState<string>("");
  const [expiries, setExpiries] = useState<string[]>([]);
  const [expiry, setExpiry] = useState<string>("");
  const [chain, setChain] = useState<OptionChain | null>(null);
  const [specs, setSpecs] = useState<ContractSpec[]>([]);

  /** Order-ticket lots. Applies to every Buy/Sell button on the page. */
  const [lots, setLots] = useState(1);
  /**
   * Order-ticket leverage, as Delta sets it per position.
   *
   * Null means "the venue's default for this contract". It is clamped
   * server-side to what the venue actually allows at the chosen size, so a
   * number typed here can never buy more leverage than Delta would give.
   */
  const [leverage, setLeverage] = useState<number | null>(null);

  const [perps, setPerps] = useState<Instrument[]>([]);
  const [movers, setMovers] = useState<{ topCalls: TopMover[]; topPuts: TopMover[] } | null>(null);

  const [positions, setPositions] = useState<LivePosition[]>([]);
  const [summary, setSummary] = useState<PositionsSummary | null>(null);
  const [posFilter, setPosFilter] = useState<"OPEN" | "all">("OPEN");
  const [orders, setOrders] = useState<Order[]>([]);
  const [closed, setClosed] = useState<LivePosition[]>([]);

  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [reduceFor, setReduceFor] = useState<{ id: string; name: string; max: number; lots: string } | null>(null);

  const [basket, setBasket] = useState<BasketItem[]>([]);
  const [preview, setPreview] = useState<BasketPreview | null>(null);
  const [accountForm, setAccountForm] = useState<null | { mode: "create" | "edit"; name: string; capital: string }>(null);

  const say = useCallback((m: string) => {
    setNotice(m);
    setError(null);
  }, []);
  const blame = useCallback((e: unknown) => {
    setError(e instanceof Error ? e.message : String(e));
    setNotice(null);
  }, []);

  /** Run a mutation under a named busy flag, then refresh. */
  const act = useCallback(
    async (key: string, fn: () => Promise<void>) => {
      setBusy(key);
      try {
        await fn();
      } catch (e) {
        blame(e);
      } finally {
        setBusy(null);
      }
    },
    [blame],
  );

  /* ── loaders ──────────────────────────────────────────────────────────── */
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

  useEffect(() => {
    get<{ underlyings: { symbol: string; spot: number | null }[] }>("underlyings")
      .then((d) => {
        setUnderlyings(d.underlyings);
        setUnderlying((cur) => cur || d.underlyings[0]?.symbol || "");
      })
      .catch(blame);
    get<{ specs: ContractSpec[] }>("specs").then((d) => setSpecs(d.specs)).catch(() => {});
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
    try {
      const d = await get<{ chain: OptionChain }>(
        `options/chain?underlying=${encodeURIComponent(underlying)}&expiry=${encodeURIComponent(expiry)}`,
      );
      setChain(d.chain);
    } catch (e) {
      blame(e);
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
    if (tab === "history" && accountId) {
      get<{ positions: LivePosition[] }>(`positions?account_id=${encodeURIComponent(accountId)}&status=CLOSED`)
        .then((d) => setClosed(d.positions))
        .catch(blame);
    }
  }, [tab, accountId, blame]);

  /* ── selection ────────────────────────────────────────────────────────── */
  // Only options can be rolled: a perpetual has no strike to be at the money.
  const rollable = useMemo(() => positions.filter((p) => p.status === "OPEN" && p.optionType !== null), [positions]);
  const openRows = useMemo(() => positions.filter((p) => p.status === "OPEN"), [positions]);

  // Read the selection back through the LIVE book. A roll replaces position ids
  // and a row can be closed from another tab, so a stale tick would otherwise
  // ask the server to act on legs that no longer exist.
  const selected = useMemo(() => openRows.filter((p) => picked.has(p.positionId)), [openRows, picked]);
  const selectedRollable = useMemo(() => selected.filter((p) => p.optionType !== null), [selected]);

  const toggle = useCallback((id: string) => {
    setPicked((s) => {
      const n = new Set(s);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });
  }, []);

  const toggleAll = useCallback(() => {
    setPicked((s) => (s.size === openRows.length ? new Set() : new Set(openRows.map((p) => p.positionId))));
  }, [openRows]);

  /* ── actions ──────────────────────────────────────────────────────────── */
  const addLeg = useCallback(
    (symbol: string, displayName: string, side: TransactionType) => {
      setBasket((b) => {
        const i = b.findIndex((x) => x.symbol === symbol && x.side === side);
        if (i >= 0) {
          const next = [...b];
          next[i] = { ...next[i], lots: next[i].lots + lots };
          return next;
        }
        return [...b, { symbol, displayName, side, lots, leverage }];
      });
    },
    [lots, leverage],
  );

  useEffect(() => {
    if (!accountId || basket.length === 0) {
      setPreview(null);
      return;
    }
    let live = true;
    post<{ preview: BasketPreview }>("basket/preview", {
      account_id: accountId,
      legs: basket.map((b) => ({ symbol: b.symbol, transactionType: b.side, lots: b.lots, leverage: b.leverage ?? undefined })),
    })
      .then((d) => live && setPreview(d.preview))
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

  const execute = () =>
    act("basket", async () => {
      const r = await post<{ filled: number; marginAdded: number; netPremium: number; feesUsd: number }>(
        "basket/execute",
        {
          account_id: accountId,
          legs: basket.map((b) => ({ symbol: b.symbol, transactionType: b.side, lots: b.lots, leverage: b.leverage ?? undefined })),
        },
      );
      say(
        `Filled ${r.filled} leg${r.filled === 1 ? "" : "s"} — margin blocked ${usd(r.marginAdded)}, ` +
          `net premium ${usd(Math.abs(r.netPremium))} ${r.netPremium >= 0 ? "credit" : "debit"}, fees ${usd(r.feesUsd)}`,
      );
      setBasket([]);
      setTab("positions");
      await loadBook();
    });

  const exitOne = (p: LivePosition) =>
    act(`exit-${p.positionId}`, async () => {
      const r = await post<{ realizedPnl: number; fillPrice: number }>("positions/exit", {
        account_id: accountId,
        position_id: p.positionId,
      });
      say(`Exited ${p.displayName} at ${usd(r.fillPrice, 4)} — realized ${usd(r.realizedPnl)}`);
      setPicked((s) => {
        const n = new Set(s);
        n.delete(p.positionId);
        return n;
      });
      await loadBook();
    });

  const closeSelected = () => {
    const n = selected.length;
    if (!n) return;
    if (!window.confirm(`Close ${n} selected position${n === 1 ? "" : "s"} at the live price?`)) return;
    act("close-many", async () => {
      const r = await post<{ closed: number; realizedPnl: number; failed: { reason: string }[] }>(
        "positions/close-many",
        { account_id: accountId, position_ids: selected.map((p) => p.positionId) },
      );
      say(`Closed ${r.closed} position${r.closed === 1 ? "" : "s"} — realized ${usd(r.realizedPnl)}`);
      if (r.failed.length) setError(r.failed.map((f) => f.reason).join(" · "));
      setPicked(new Set());
      await loadBook();
    });
  };

  const roll = (ids?: string[]) => {
    const n = ids?.length ?? rollable.length;
    if (!n) return;
    if (
      !window.confirm(
        `Roll ${ids ? `the ${n} selected` : `all ${n}`} option leg${n === 1 ? "" : "s"} to the money?\n\n` +
          "Each is closed at the live price, realising its P&L, and re-opened at the strike nearest its own " +
          "spot — same side, same lots. Perpetuals are left alone.\n\n" +
          (ids && ids.length < rollable.length
            ? "Note: rolling only part of a pair leaves the other leg where it is, which can cost MORE margin " +
              "than the pair did. It is checked against your margin first.\n\n"
            : "") +
          "Nothing in a group is closed unless the whole group fits.",
      )
    )
      return;
    act("roll", async () => {
      const r = await post<RollResult>("positions/roll-atm", {
        account_id: accountId,
        position_ids: ids,
      });
      say(
        `${r.note} Realized ${usd(r.realizedPnl)}, margin ${r.marginDelta >= 0 ? "+" : "−"}${usd(Math.abs(r.marginDelta))}, fees ${usd(r.feesUsd)}.`,
      );
      if (r.failed.length) {
        setError(r.failed.map((f) => `${f.underlying} ${formatExpiry(f.expiry)}: ${f.reason}`).join(" · "));
      }
      setPicked(new Set());
      await loadBook();
    });
  };

  const submitReduce = () => {
    if (!reduceFor) return;
    const n = Number(reduceFor.lots);
    if (!Number.isFinite(n) || n <= 0) return;
    act("reduce", async () => {
      const r = await post<{ closedLots: number; remainingLots: number; realizedPnl: number; fillPrice: number }>(
        "positions/reduce",
        { account_id: accountId, position_id: reduceFor.id, lots: n },
      );
      say(
        `Closed ${r.closedLots} of ${reduceFor.max} lot(s) at ${usd(r.fillPrice, 4)} — realized ${usd(r.realizedPnl)}. ` +
          `${r.remainingLots} left open at the original entry.`,
      );
      setReduceFor(null);
      await loadBook();
    });
  };

  const reset = () => {
    if (!window.confirm("Clear this account's positions and orders?\n\nThey are archived with a balance snapshot first, so this can be answered for afterwards.")) return;
    act("reset", async () => {
      const r = await post<{ positionsCleared: number; ordersCleared: number; archived: number }>("reset", {
        account_id: accountId,
      });
      say(
        `Cleared ${r.positionsCleared} position(s) and ${r.ordersCleared} order(s). ${r.archived} record(s) archived — recoverable.`,
      );
      setPicked(new Set());
      await loadBook();
    });
  };

  const del = () => {
    const a = accounts.find((x) => x.accountId === accountId);
    if (!window.confirm(`Delete "${a?.name ?? "this account"}" and its whole history?\n\nIt is archived first.`)) return;
    act("delete", async () => {
      await post("accounts/delete", { account_id: accountId });
      say(`Deleted "${a?.name ?? "account"}".`);
      setAccountId(null);
      await loadAccounts();
    });
  };

  const submitAccount = () => {
    if (!accountForm) return;
    act("account", async () => {
      const capital = Number(accountForm.capital);
      const cap = Number.isFinite(capital) && capital > 0 ? capital : undefined;
      if (accountForm.mode === "create") {
        const d = await post<{ account: Account }>("accounts", { name: accountForm.name, initial_capital: cap });
        setAccountId(d.account.accountId);
        say(`Created ${d.account.name} with ${usd(d.account.initialCapital)}`);
      } else {
        await post("accounts/edit", { account_id: accountId, name: accountForm.name, initial_capital: cap });
        say("Account updated.");
      }
      setAccountForm(null);
      await loadAccounts();
      await loadBook();
    });
  };

  const active = useMemo(() => accounts.find((a) => a.accountId === accountId) ?? null, [accounts, accountId]);
  const spec = useMemo(() => specs.find((s) => s.underlying === underlying) ?? null, [specs, underlying]);

  /* ── columns ──────────────────────────────────────────────────────────── */
  const positionCols: DeskColumn<LivePosition>[] = [
    {
      id: "pick",
      header: (
        <input
          type="checkbox"
          aria-label="Select every open position"
          checked={openRows.length > 0 && picked.size === openRows.length}
          onChange={toggleAll}
        />
      ),
      width: "36px",
      cell: (p) =>
        p.status === "OPEN" ? (
          <input
            type="checkbox"
            aria-label={`Select ${p.displayName}`}
            checked={picked.has(p.positionId)}
            onChange={() => toggle(p.positionId)}
          />
        ) : null,
    },
    {
      id: "name",
      header: "Contract",
      cell: (p) => (
        <span style={{ display: "grid" }}>
          <strong>{p.displayName}</strong>
          <span style={{ fontSize: 11, opacity: 0.6 }}>{p.kind === "OPTION" ? "OPTION" : "PERPETUAL"}</span>
        </span>
      ),
    },
    { id: "side", header: "Side", cell: (p) => <DeskChip label={p.side} tone={p.side === "BUY" ? "success" : "danger"} /> },
    { id: "lots", header: "Lots", align: "right", cell: (p) => p.lots },
    {
      id: "qty",
      header: "Qty",
      align: "right",
      sortValue: (p) => p.lots * p.contractValue,
      cell: (p) => (
        <span title={`${p.lots} x ${p.contractValue} ${p.underlying}`}>
          {(p.lots * p.contractValue).toLocaleString("en-US", { maximumFractionDigits: 6 })}
        </span>
      ),
    },
    { id: "entry", header: "Entry", align: "right", cell: (p) => usd(p.entryPrice, 4) },
    {
      id: "ltp",
      header: "LTP",
      align: "right",
      cell: (p) => (p.status === "OPEN" ? usd(p.markPrice, 4) : usd(p.exitPrice, 4)),
    },
    {
      id: "value",
      header: "Contract value",
      align: "right",
      sortValue: (p) => Math.abs((p.markPrice ?? p.entryPrice) * p.lots * p.contractValue),
      cell: (p) => compact(Math.abs((p.markPrice ?? p.entryPrice) * p.lots * p.contractValue)),
    },
    { id: "margin", header: "Margin", align: "right", cell: (p) => compact(p.standaloneMarginUsd) },
    {
      id: "lev",
      header: "Lev",
      align: "right",
      sortValue: (p) => p.liquidation?.leverage ?? p.leverage,
      cell: (p) =>
        p.liquidation ? (
          <span title={`initial margin ${usd(p.liquidation.initialMarginUsd)}`}>
            {p.liquidation.leverage.toFixed(p.liquidation.leverage < 10 ? 1 : 0)}x
          </span>
        ) : (
          <span title="A bought option is paid for in full — nothing is borrowed" style={{ opacity: 0.5 }}>1x</span>
        ),
    },
    {
      id: "liq",
      header: "Liquidation",
      align: "right",
      sortValue: (p) => (p.liquidation ? Math.abs(p.liquidation.distancePct) : Number.POSITIVE_INFINITY),
      cell: (p) => {
        if (p.status !== "OPEN") return "—";
        if (!p.liquidation) {
          // Not a missing number. Buying pays the premium in full, so there is
          // no price at which the venue takes the position away.
          return <span style={{ opacity: 0.55 }} title="Premium paid in full — no borrow, no liquidation">none</span>;
        }
        const d = Math.abs(p.liquidation.distancePct);
        // Under 10% away is close enough that it should read as a warning
        // rather than as another number in the row.
        const danger = d < 10;
        return (
          <span
            style={{ color: danger ? "var(--desk-loss)" : undefined, fontWeight: danger ? 700 : 500 }}
            title={`bankruptcy ${usd(p.liquidation.bankruptcyPrice, 4)} · maintenance margin ${usd(p.liquidation.maintenanceMarginUsd)} · liquidation penalty ${(p.liquidation.penaltyFactor * 100).toFixed(0)}%`}
          >
            {usd(p.liquidation.price, 4)}
            <span style={{ display: "block", fontSize: 11, opacity: 0.75 }}>{pct(p.liquidation.distancePct, 1)} away</span>
          </span>
        );
      },
    },
    {
      id: "pnl",
      header: "Unrealised",
      align: "right",
      sortValue: (p) => (p.status === "OPEN" ? p.unrealizedPnl : p.realizedPnl),
      cell: (p) => {
        const v = p.status === "OPEN" ? p.unrealizedPnl : p.realizedPnl;
        return <span style={{ color: tone(v), fontWeight: 600 }}>{usd(v)}</span>;
      },
    },
    {
      id: "act",
      header: "",
      align: "right",
      cell: (p) =>
        p.status !== "OPEN" ? (
          <DeskChip label="CLOSED" tone="default" />
        ) : (
          <span style={{ display: "inline-flex", gap: 6, whiteSpace: "nowrap" }}>
            {p.optionType && (
              <DeskButton variant="tonal" onClick={() => roll([p.positionId])} disabled={busy !== null}>
                {busy === "roll" ? "Rolling…" : "Re-add ATM"}
              </DeskButton>
            )}
            <DeskButton
              variant="outlined"
              disabled={busy !== null || p.lots < 2}
              title={p.lots < 2 ? "Only one lot — use Close" : "Close part of this position"}
              onClick={() => setReduceFor({ id: p.positionId, name: p.displayName, max: p.lots, lots: "1" })}
            >
              Edit
            </DeskButton>
            <DeskButton variant="danger-tonal" onClick={() => exitOne(p)} disabled={busy !== null}>
              Close
            </DeskButton>
          </span>
        ),
    },
  ];

  const orderCols: DeskColumn<Order>[] = [
    { id: "time", header: "Time", sortValue: (o) => o.createdAt, cell: (o) => new Date(o.createdAt).toLocaleString() },
    { id: "name", header: "Contract", cell: (o) => o.displayName },
    { id: "intent", header: "Intent", cell: (o) => <DeskChip label={o.intent} tone="default" /> },
    { id: "side", header: "Side", cell: (o) => <DeskChip label={o.transactionType} tone={o.transactionType === "BUY" ? "success" : "danger"} /> },
    { id: "lots", header: "Lots", align: "right", cell: (o) => o.lots },
    { id: "fill", header: "Fill", align: "right", cell: (o) => usd(o.fillPrice, 4) },
    { id: "status", header: "Status", cell: (o) => <DeskChip label={o.status} tone={o.status === "FILLED" ? "success" : "danger"} /> },
    { id: "why", header: "Note", cell: (o) => <span style={{ opacity: 0.7, fontSize: 12 }}>{o.rejectReason ?? "—"}</span> },
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
    { id: "turnover", header: "24h turnover", align: "right", sortValue: (i) => i.turnoverUsd ?? 0, cell: (i) => compact(i.turnoverUsd) },
    {
      id: "act",
      header: "",
      align: "right",
      cell: (i) => (
        <span style={{ display: "inline-flex", gap: 6 }}>
          <DeskButton variant="tonal" onClick={() => addLeg(i.symbol, `${i.symbol} PERP`, "BUY")}>Buy</DeskButton>
          <DeskButton variant="outlined" onClick={() => addLeg(i.symbol, `${i.symbol} PERP`, "SELL")}>Sell</DeskButton>
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
    { id: "to", header: "Turnover", align: "right", sortValue: (m) => m.turnoverUsd ?? 0, cell: (m) => compact(m.turnoverUsd) },
    {
      id: "act",
      header: "",
      align: "right",
      cell: (m) => (
        <span style={{ display: "inline-flex", gap: 6 }}>
          <DeskButton variant="tonal" onClick={() => addLeg(m.symbol, m.displayName, "BUY")}>Buy</DeskButton>
          <DeskButton variant="outlined" onClick={() => addLeg(m.symbol, m.displayName, "SELL")}>Sell</DeskButton>
        </span>
      ),
    },
  ];

  const specCols: DeskColumn<ContractSpec>[] = [
    { id: "u", header: "Underlying", cell: (s) => <strong>{s.underlying}</strong> },
    { id: "cv", header: "1 contract =", align: "right", cell: (s) => `${s.contractValue} ${s.underlying}` },
    { id: "unit", header: "Quoted", cell: (s) => s.priceUnit },
    { id: "spot", header: "Spot", align: "right", cell: (s) => usd(s.spot) },
    {
      id: "val",
      header: "Value per contract",
      align: "right",
      sortValue: (s) => s.contractValueUsd ?? 0,
      cell: (s) => usd(s.contractValueUsd),
    },
    { id: "tick", header: "Tick", align: "right", cell: (s) => (s.tickSize === null ? "—" : String(s.tickSize)) },
    { id: "exp", header: "Expiries", align: "right", cell: (s) => s.expiryCount },
    { id: "o", header: "Options", align: "right", cell: (s) => s.optionCount.toLocaleString() },
    { id: "p", header: "Perps", align: "right", cell: (s) => s.perpetualCount },
  ];

  const historyCols: DeskColumn<LivePosition>[] = [
    { id: "closed", header: "Closed", sortValue: (p) => p.closedAt ?? 0, cell: (p) => (p.closedAt ? new Date(p.closedAt).toLocaleString() : "—") },
    { id: "name", header: "Contract", cell: (p) => p.displayName },
    { id: "side", header: "Side", cell: (p) => <DeskChip label={p.side} tone={p.side === "BUY" ? "success" : "danger"} /> },
    { id: "lots", header: "Lots", align: "right", cell: (p) => p.lots },
    { id: "entry", header: "Entry", align: "right", cell: (p) => usd(p.entryPrice, 4) },
    { id: "exit", header: "Exit", align: "right", cell: (p) => usd(p.exitPrice, 4) },
    { id: "fees", header: "Fees", align: "right", cell: (p) => usd(p.feesUsd) },
    {
      id: "pnl",
      header: "Realised",
      align: "right",
      sortValue: (p) => p.realizedPnl,
      cell: (p) => <span style={{ color: tone(p.realizedPnl), fontWeight: 600 }}>{usd(p.realizedPnl)}</span>,
    },
  ];

  const sel = { padding: "7px 10px", borderRadius: 10, border: "1px solid var(--desk-outline-variant)", background: "var(--desk-surface)", color: "inherit" } as const;

  return (
    <DeskShell
      loading={busy !== null}
      appBar={
        <DeskAppBar
          title="Crypto Positions"
          status={chain || perps.length > 0 ? "live" : "syncing"}
          subtitle="Live Delta option chains and perpetuals — buy or sell at real quoted premiums with hedge-aware portfolio margin, across paper accounts with their own editable balance. Not investment advice."
          equity={summary?.equity}
          equityCurrency="USD"
          equityLabel="Paper equity"
          equityDetail={summary ? `${pct(summary.roiPct)} on ${usd(summary.initialCapital, 0)}` : undefined}
        />
      }
    >
      {/* ── account bar ──────────────────────────────────────────────────── */}
      <DeskCard padding="md">
        <DeskSectionHeader
          title="Paper account"
          subtitle={`${accounts.length} book${accounts.length === 1 ? "" : "s"} · settles in USD · no broker path`}
          actions={
            <span style={{ display: "inline-flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
              <select value={accountId ?? ""} onChange={(e) => setAccountId(e.target.value)} style={sel}>
                {accounts.map((a) => (
                  <option key={a.accountId} value={a.accountId}>
                    {a.name} — starting {usd(a.initialCapital, 0)}
                  </option>
                ))}
              </select>
              <DeskButton variant="outlined" onClick={loadBook} disabled={busy !== null}>Refresh</DeskButton>
              <DeskButton variant="tonal" onClick={() => setAccountForm({ mode: "create", name: "", capital: "100000" })}>+ New account</DeskButton>
              <DeskButton variant="outlined" disabled={!active} onClick={() => active && setAccountForm({ mode: "edit", name: active.name, capital: String(active.initialCapital) })}>Edit</DeskButton>
              <DeskButton variant="outlined" onClick={reset} disabled={busy !== null || !accountId}>Reset</DeskButton>
              <DeskButton variant="danger-tonal" onClick={del} disabled={busy !== null || !accountId}>Delete</DeskButton>
            </span>
          }
        />
        {accountForm && (
          <DeskCard variant="outlined" padding="md" style={{ marginTop: 12 }}>
            <div style={{ display: "flex", gap: 12, flexWrap: "wrap", alignItems: "flex-end" }}>
              <label style={{ display: "grid", gap: 4 }}>
                <span style={{ fontSize: 12, opacity: 0.7 }}>Name</span>
                <input value={accountForm.name} onChange={(e) => setAccountForm({ ...accountForm, name: e.target.value })} style={sel} />
              </label>
              <label style={{ display: "grid", gap: 4 }}>
                <span style={{ fontSize: 12, opacity: 0.7 }}>Balance (USD)</span>
                <input value={accountForm.capital} inputMode="decimal" onChange={(e) => setAccountForm({ ...accountForm, capital: e.target.value })} style={sel} />
              </label>
              <DeskButton variant="filled" onClick={submitAccount} disabled={busy !== null}>{accountForm.mode === "create" ? "Create" : "Save"}</DeskButton>
              <DeskButton variant="text" onClick={() => setAccountForm(null)}>Cancel</DeskButton>
            </div>
            <p style={{ marginTop: 8, fontSize: 12, opacity: 0.7 }}>
              Changing the balance moves the base capital only. Open positions and realised P&amp;L are untouched, so ROI
              is restated against the new base rather than recalculated from it.
            </p>
          </DeskCard>
        )}
      </DeskCard>

      {/* ── tiles ────────────────────────────────────────────────────────── */}
      {summary && (
        <div style={{ display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit, minmax(148px, 1fr))", marginTop: 12 }}>
          <DeskMetricTile label="Equity" value={usd(summary.equity)} sub={`started at ${usd(summary.initialCapital, 0)}`} />
          <DeskMetricTile label="Available cash" value={usd(summary.availableCash)} sub={`${usd(summary.deployedMargin)} margin blocked`} />
          <DeskMetricTile label="Contract exposure" value={compact(summary.contractExposureUsd)} sub="full notional of the open book" />
          <DeskMetricTile label="Unrealised" value={usd(summary.unrealizedPnl)} subColor={summary.unrealizedPnl >= 0 ? "profit" : "loss"} sub={`${summary.openPositions} open`} />
          <DeskMetricTile label="Realised" value={usd(summary.realizedPnl)} subColor={summary.realizedPnl >= 0 ? "profit" : "loss"} sub={`${summary.closedPositions} closed`} />
          <DeskMetricTile label="Win %" value={summary.winPct === null ? "—" : `${summary.winPct.toFixed(1)}%`} sub={summary.closedPositions === 0 ? "nothing closed yet" : undefined} />
          <DeskMetricTile
            label="Margin level"
            value={summary.marginLevelPct === null ? "—" : `${summary.marginLevelPct.toFixed(0)}%`}
            sub={summary.marginLevelPct === null ? "nothing posted" : `maintenance ${usd(summary.maintenanceMarginUsd)}`}
            subColor={summary.marginLevelPct !== null && summary.marginLevelPct < 150 ? "loss" : "default"}
          />
          <DeskMetricTile
            label="Account leverage"
            value={summary.accountLeverage === null ? "—" : `${summary.accountLeverage.toFixed(2)}x`}
            sub={`${summary.liquidatablePositions} liquidatable`}
          />
          {summary.marginBenefit > 0 && <DeskMetricTile label="Hedge benefit" value={`−${usd(summary.marginBenefit)}`} subColor="profit" sub="saved by offsets" />}
          <DeskMetricTile label="Underlyings" value={summary.underlyingsOpen} sub={`${specs.length} tradable`} />
        </div>
      )}

      {summary && summary.marginLevelPct !== null && summary.marginLevelPct < 150 && (
        <div style={{ marginTop: 12 }}>
          <DeskBanner variant="warning" title="Margin level is low">
            Equity is {summary.marginLevelPct.toFixed(0)}% of the {usd(summary.maintenanceMarginUsd)} maintenance
            requirement. At 100% the venue starts closing positions, and it charges a liquidation penalty on top of the
            loss — closing or hedging voluntarily is cheaper than being closed.
          </DeskBanner>
        </div>
      )}

      {error && <div style={{ marginTop: 12 }}><DeskBanner variant="error" title="That did not go through">{error}</DeskBanner></div>}
      {notice && <div style={{ marginTop: 12 }}><DeskBanner variant="success">{notice}</DeskBanner></div>}

      {/* ── order ticket ─────────────────────────────────────────────────── */}
      <DeskCard padding="md" style={{ marginTop: 12 }}>
        <DeskSectionHeader title="Order ticket" subtitle="applies to every Buy / Sell button below" />
        <div style={{ display: "flex", gap: 16, flexWrap: "wrap", alignItems: "flex-end", marginTop: 10 }}>
          <label style={{ display: "grid", gap: 4 }}>
            <span style={{ fontSize: 12, opacity: 0.7 }}>Underlying</span>
            <select value={underlying} onChange={(e) => setUnderlying(e.target.value)} style={sel}>
              {underlyings.map((u) => (
                <option key={u.symbol} value={u.symbol}>{u.symbol}</option>
              ))}
            </select>
          </label>
          <label style={{ display: "grid", gap: 4 }}>
            <span style={{ fontSize: 12, opacity: 0.7 }}>Lots</span>
            <input
              value={lots}
              inputMode="numeric"
              onChange={(e) => {
                const n = Math.floor(Number(e.target.value));
                setLots(Number.isFinite(n) && n > 0 ? n : 1);
              }}
              style={{ ...sel, width: 90 }}
            />
          </label>
          <label style={{ display: "grid", gap: 4 }}>
            <span style={{ fontSize: 12, opacity: 0.7 }}>
              Leverage{spec ? ` (max ${spec.maxLeverage}x)` : ""}
            </span>
            <select
              value={leverage === null ? "" : String(leverage)}
              onChange={(e) => setLeverage(e.target.value === "" ? null : Number(e.target.value))}
              style={sel}
            >
              <option value="">Venue default{spec ? ` (${spec.defaultLeverage}x)` : ""}</option>
              {[1, 2, 3, 5, 10, 20, 25, 50, 75, 100, 150, 200]
                .filter((x) => !spec || x <= spec.maxLeverage)
                .map((x) => (
                  <option key={x} value={x}>{x}x</option>
                ))}
            </select>
          </label>
          <label style={{ display: "grid", gap: 4 }}>
            <span style={{ fontSize: 12, opacity: 0.7 }}>Expiry</span>
            <select value={expiry} onChange={(e) => setExpiry(e.target.value)} style={sel}>
              {expiries.map((e) => (
                <option key={e} value={e}>{formatExpiry(e)}</option>
              ))}
            </select>
          </label>
          {spec && (
            <div style={{ fontSize: 12, lineHeight: 1.6, opacity: 0.85, borderLeft: "2px solid var(--desk-outline-variant)", paddingLeft: 12 }}>
              <div><b>1 contract = {spec.contractValue} {spec.underlying}</b> · quoted {spec.priceUnit}</div>
              <div style={{ opacity: 0.7 }}>
                one contract is worth {usd(spec.contractValueUsd)} at spot · {spec.optionCount.toLocaleString()} options
                across {spec.expiryCount} expiries · {spec.perpetualCount} perpetual{spec.perpetualCount === 1 ? "" : "s"}
              </div>
              <div style={{ opacity: 0.7 }}>
                {lots} lot{lots === 1 ? "" : "s"} controls {(lots * spec.contractValue).toLocaleString("en-US", { maximumFractionDigits: 6 })} {spec.underlying}
                {spec.contractValueUsd !== null ? ` — about ${usd(lots * spec.contractValueUsd)}` : ""}
              </div>
              <div style={{ opacity: 0.7 }}>
                venue margin {spec.initialMarginPct}% initial / {spec.maintenanceMarginPct}% maintenance — the
                initial rate is what caps leverage at {spec.maxLeverage}x, and Delta raises both as a position grows
              </div>
              <div style={{ opacity: 0.7 }}>
                A BOUGHT option ignores this: the premium is paid in full, so it is always 1x with no liquidation.
              </div>
            </div>
          )}
        </div>
      </DeskCard>

      {/* ── basket ───────────────────────────────────────────────────────── */}
      {basket.length > 0 && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader
            title={preview?.label ?? "Basket"}
            subtitle={`${basket.length} leg${basket.length === 1 ? "" : "s"} · margin is charged on what this ADDS to the book, so a hedge can cost less than it would alone`}
            actions={
              <span style={{ display: "inline-flex", gap: 8 }}>
                <DeskButton variant="text" onClick={() => setBasket([])}>Clear</DeskButton>
                <DeskButton variant="filled" onClick={execute} disabled={busy !== null || !preview?.affordable}>Execute</DeskButton>
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
                    const n = Math.floor(Number(e.target.value));
                    setBasket((cur) => cur.map((x, i) => (i === idx ? { ...x, lots: Number.isFinite(n) && n > 0 ? n : 1 } : x)));
                  }}
                  style={{ ...sel, width: 76 }}
                />
                <DeskButton variant="text" onClick={() => setBasket((cur) => cur.filter((_, i) => i !== idx))}>Remove</DeskButton>
              </div>
            ))}
          </div>
          {preview && preview.legs.some((l) => l.liquidation) && (
            <div style={{ marginTop: 10, fontSize: 12, opacity: 0.85, display: "grid", gap: 4 }}>
              {preview.legs.map((l) => (
                <div key={`${l.symbol}-${l.side}`}>
                  <b>{l.displayName}</b> — {l.leverage.toFixed(l.leverage < 10 ? 1 : 0)}x of max {l.maxLeverage}x
                  {l.liquidation
                    ? ` · liquidation ${usd(l.liquidation.price, 4)} (${pct(l.liquidation.distancePct, 1)} away) · initial margin ${usd(l.liquidation.initialMarginUsd)}`
                    : " · bought outright, no liquidation"}
                </div>
              ))}
            </div>
          )}
          {preview && (
            <div style={{ display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))", marginTop: 12 }}>
              <DeskMetricTile label="Margin required" value={usd(preview.marginRequired)} />
              <DeskMetricTile label="Net premium" value={`${usd(Math.abs(preview.netPremium))} ${preview.netPremium >= 0 ? "credit" : "debit"}`} subColor={preview.netPremium >= 0 ? "profit" : "loss"} />
              <DeskMetricTile label="Worst case" value={usd(preview.worstCaseLossUsd)} sub="over ±20% spot" />
              {preview.marginBenefit > 0 && <DeskMetricTile label="Hedge benefit" value={`−${usd(preview.marginBenefit)}`} subColor="profit" />}
              <DeskMetricTile label="Fees" value={usd(preview.feesUsd)} />
              <DeskMetricTile label="Available" value={usd(preview.availableCash)} />
            </div>
          )}
          {preview && !preview.affordable && (
            <div style={{ marginTop: 10 }}>
              <DeskBanner variant="warning" title="Not enough margin">
                This basket needs {usd(preview.marginRequired)} but only {usd(preview.availableCash)} is available. Reduce
                lots, add a hedge that offsets it, or raise the account balance.
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
            { key: "specs", label: "Contract Specs" },
            { key: "history", label: "History" },
          ]}
          active={tab}
          onChange={(k) => setTab(k as Tab)}
        />
      </div>

      {/* ── chain ────────────────────────────────────────────────────────── */}
      {tab === "chain" && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader
            title="Option chain"
            subtitle={chain ? `${chain.rows.length} strikes · spot ${usd(chain.spot)} · buying ${lots} lot${lots === 1 ? "" : "s"} per click` : "Loading the live chain…"}
            actions={<DeskChip label={`${underlying} · ${expiry ? formatExpiry(expiry) : "—"}`} tone="primary" />}
          />
          {chain && chain.rows.length > 0 ? (
            <div style={{ overflowX: "auto", marginTop: 10 }}>
              <AutoSortTable>
                <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13, minWidth: 940 }}>
                  <thead>
                    <tr>
                      <th colSpan={5} style={{ textAlign: "center", padding: 6, opacity: 0.65, letterSpacing: 1 }}>CALLS</th>
                      <th style={{ padding: 6 }}>Strike</th>
                      <th colSpan={5} style={{ textAlign: "center", padding: 6, opacity: 0.65, letterSpacing: 1 }}>PUTS</th>
                    </tr>
                    <tr style={{ opacity: 0.65, fontSize: 11 }}>
                      <th style={{ padding: 4 }}>OI</th><th style={{ padding: 4 }}>IV</th><th style={{ padding: 4 }}>Δ</th><th style={{ padding: 4 }}>Mark</th><th />
                      <th />
                      <th /><th style={{ padding: 4 }}>Mark</th><th style={{ padding: 4 }}>Δ</th><th style={{ padding: 4 }}>IV</th><th style={{ padding: 4 }}>OI</th>
                    </tr>
                  </thead>
                  <tbody>
                    {chain.rows.map((r) => {
                      const atm = r.strike === chain.atmStrike;
                      const label = (t: "CALL" | "PUT") => `${chain.underlying} ${formatExpiry(chain.expiry)} ${r.strike} ${t}`;
                      return (
                        <tr key={r.strike} style={{ borderTop: "1px solid var(--desk-outline-variant)", background: atm ? "var(--desk-primary-container)" : undefined }}>
                          <td style={{ padding: 4, textAlign: "right" }}>{r.call?.openInterest?.toLocaleString() ?? "—"}</td>
                          <td style={{ padding: 4, textAlign: "right" }}>{r.call?.ivPct ? `${r.call.ivPct.toFixed(1)}%` : "—"}</td>
                          <td style={{ padding: 4, textAlign: "right" }}>{r.call?.greeks ? r.call.greeks.delta.toFixed(3) : "—"}</td>
                          <td style={{ padding: 4, textAlign: "right", fontWeight: 600 }}>{r.call ? usd(r.call.markPrice, 2) : "—"}</td>
                          <td style={{ padding: 4, whiteSpace: "nowrap" }}>
                            {r.call && (
                              <span style={{ display: "inline-flex", gap: 4 }}>
                                <DeskButton variant="tonal" onClick={() => addLeg(r.call!.symbol, label("CALL"), "BUY")}>B</DeskButton>
                                <DeskButton variant="outlined" onClick={() => addLeg(r.call!.symbol, label("CALL"), "SELL")}>S</DeskButton>
                              </span>
                            )}
                          </td>
                          <td style={{ padding: 4, textAlign: "center", fontWeight: 700 }}>{r.strike.toLocaleString()}</td>
                          <td style={{ padding: 4, whiteSpace: "nowrap" }}>
                            {r.put && (
                              <span style={{ display: "inline-flex", gap: 4 }}>
                                <DeskButton variant="tonal" onClick={() => addLeg(r.put!.symbol, label("PUT"), "BUY")}>B</DeskButton>
                                <DeskButton variant="outlined" onClick={() => addLeg(r.put!.symbol, label("PUT"), "SELL")}>S</DeskButton>
                              </span>
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

      {tab === "perps" && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader title="Perpetuals" subtitle={`${perps.length} contracts · Delta India lists no dated futures, so these never expire and pay funding instead`} />
          <DeskDataTable columns={perpCols} rows={perps} getRowKey={(i) => i.symbol} empty={<DeskEmptyState title="No perpetuals" subtitle="Waiting for the venue." />} />
        </DeskCard>
      )}

      {tab === "movers" && (
        <div style={{ display: "grid", gap: 12, marginTop: 12 }}>
          <DeskCard padding="md">
            <DeskSectionHeader title="Top calls" subtitle="Ranked by traded value, not percent change — a 200% move on a $0.05 option is not a mover" />
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
            subtitle={`${positions.length} in ${active?.name ?? "—"} · marked to the live Delta price`}
            actions={
              <span style={{ display: "inline-flex", gap: 6 }}>
                <DeskButton variant={posFilter === "OPEN" ? "filled" : "outlined"} onClick={() => setPosFilter("OPEN")}>Open</DeskButton>
                <DeskButton variant={posFilter === "all" ? "filled" : "outlined"} onClick={() => setPosFilter("all")}>All</DeskButton>
              </span>
            }
          />

          {reduceFor && (
            <DeskCard variant="outlined" padding="md" style={{ marginTop: 10 }}>
              <div style={{ display: "flex", gap: 12, flexWrap: "wrap", alignItems: "flex-end" }}>
                <div style={{ flex: 1, minWidth: 220 }}>
                  <b>{reduceFor.name}</b>
                  <div style={{ fontSize: 12, opacity: 0.7 }}>
                    Holds {reduceFor.max} lots. Closing part of it leaves the rest open at the ORIGINAL entry — the
                    remainder is not re-based to the current mark, so its unrealised P&amp;L is not quietly booked.
                  </div>
                </div>
                <label style={{ display: "grid", gap: 4 }}>
                  <span style={{ fontSize: 12, opacity: 0.7 }}>Lots to close</span>
                  <input value={reduceFor.lots} inputMode="numeric" onChange={(e) => setReduceFor({ ...reduceFor, lots: e.target.value })} style={{ ...sel, width: 100 }} />
                </label>
                <DeskButton variant="filled" onClick={submitReduce} disabled={busy !== null}>Close part</DeskButton>
                <DeskButton variant="text" onClick={() => setReduceFor(null)}>Cancel</DeskButton>
              </div>
            </DeskCard>
          )}

          {openRows.length > 0 && (
            <DeskCard variant="outlined" padding="md" style={{ marginTop: 10 }}>
              <div style={{ display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
                <div style={{ flex: 1, minWidth: 260, fontSize: 13, lineHeight: 1.6 }}>
                  {selected.length ? (
                    <>
                      <b>{selected.length} of {openRows.length} selected.</b> Roll applies to the{" "}
                      {selectedRollable.length} option leg{selectedRollable.length === 1 ? "" : "s"} among them —
                      perpetuals have no strike and are left alone.
                    </>
                  ) : (
                    <>
                      <b>Roll the book to the money.</b> Every option leg is closed at the live price and re-opened at
                      the strike nearest its own spot — same side, same lots. Checked against your margin before
                      anything is closed, and done per expiry group so a straddle never sits half-rolled. Tick rows to
                      roll only some of them.
                    </>
                  )}
                </div>
                <span style={{ display: "inline-flex", gap: 8, flexWrap: "wrap" }}>
                  {selected.length > 0 && (
                    <DeskButton variant="danger-tonal" onClick={closeSelected} disabled={busy !== null}>
                      {busy === "close-many" ? "Closing…" : `Close ${selected.length} selected`}
                    </DeskButton>
                  )}
                  <DeskButton
                    variant="tonal"
                    disabled={busy !== null || (selected.length ? selectedRollable.length === 0 : rollable.length === 0)}
                    onClick={() => roll(selected.length ? selectedRollable.map((p) => p.positionId) : undefined)}
                  >
                    {busy === "roll"
                      ? "Rolling…"
                      : selected.length
                        ? `Re-add ATM · ${selectedRollable.length} selected`
                        : `Re-add ATM · all ${rollable.length} leg${rollable.length === 1 ? "" : "s"}`}
                  </DeskButton>
                </span>
              </div>
            </DeskCard>
          )}

          <div style={{ marginTop: 10 }}>
            <DeskDataTable
              columns={positionCols}
              rows={positions}
              getRowKey={(p) => p.positionId}
              empty={<DeskEmptyState title="No positions" subtitle="Build a basket from the chain or the perpetuals list." />}
            />
          </div>
        </DeskCard>
      )}

      {tab === "orders" && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader title="Orders" subtitle="Every fill and every rejection, with the reason it was refused" />
          <DeskDataTable columns={orderCols} rows={orders} getRowKey={(o) => o.orderId} empty={<DeskEmptyState title="No orders yet" subtitle="Nothing has been placed on this account." />} />
        </DeskCard>
      )}

      {tab === "specs" && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader
            title="Contract specs"
            subtitle="What one contract actually controls. A Delta option is quoted per whole unit of the underlying, so a $2,000 BTC call costs $2 for one 0.001 contract."
          />
          <DeskDataTable columns={specCols} rows={specs} getRowKey={(s) => s.underlying} empty={<DeskEmptyState title="Loading specs" />} />
        </DeskCard>
      )}

      {tab === "history" && (
        <DeskCard padding="md" style={{ marginTop: 12 }}>
          <DeskSectionHeader title="History" subtitle={`${closed.length} closed position${closed.length === 1 ? "" : "s"} in ${active?.name ?? "—"}`} />
          <DeskDataTable columns={historyCols} rows={closed} getRowKey={(p) => p.positionId} empty={<DeskEmptyState title="Nothing closed yet" subtitle="Closed positions and rolls land here." />} />
        </DeskCard>
      )}

      <p style={{ marginTop: 16, fontSize: 12, opacity: 0.65, lineHeight: 1.6 }}>
        Paper money, real prices. This desk holds no API key, signs no request and has no order-routing path — nothing
        here can reach a broker. Fills cross the spread: a buy pays the ask and a sell hits the bid, from Delta&apos;s
        published quotes, and where a contract has no quote the fill says it used the mark. Margin is scenario-based —
        the whole book is revalued across a ±20% shock in spot and the worst loss is what gets blocked, so a bought
        option that caps a sold one genuinely lowers the requirement. That model is ours, not the venue&apos;s; a real
        Delta account would be charged something different.
      </p>
    </DeskShell>
  );
}
