"use client";

/**
 * Crypto F&O — paper trading of Delta BTC options with named accounts,
 * Groww-style multi-leg baskets, and hedge-aware portfolio margin.
 *
 * The margin number shown here is the SAME one the engine enforces at execution:
 * preview and execute run one code path, so the ticket cannot disagree with the
 * fill. Selling a call and a put reserves a lot; adding long wings collapses it.
 *
 * Paper only — no order from this page reaches a broker.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskEmptyState,
  DeskMetricTile,
  DeskSectionHeader,
  StatusBadge,
  type DeskColumn,
  type DeskEngineStatus,
} from "@/components/desk/ui";

type Account = {
  id: string;
  name: string;
  initialCapitalUsd: number;
  realisedPnlUsd: number;
  marginUsedUsd: number;
  availableUsd: number;
  unrealisedPnlUsd: number;
  equityUsd: number;
  openBaskets: number;
  openPositions: number;
};

type ChainRow = {
  symbol: string;
  productId: number;
  type: "CALL" | "PUT";
  strike: number;
  expiry: string;
  markPerBtc: number;
  bid: number;
  ask: number;
  iv: number;
  contractValue: number;
};

type Margin = {
  requiredUsd: number;
  worstCaseLossUsd: number;
  worstCaseSpot: number;
  netPremiumUsd: number;
  exposureUsd: number;
  hedgeCreditUsd: number;
  standaloneUsd: number;
  basis: string;
};

type Preview = {
  margin: Margin;
  account: Account;
  label: string;
  spot: number;
  affordable: boolean;
  shortfallUsd: number;
};

type Leg = { symbol: string; side: "buy" | "sell"; lots: number };

type Position = {
  id: string;
  underlying: string;
  label: string;
  status: string;
  legs: { symbol: string; side: string; lots: number; type: string; strike: number }[];
  marginUsd: number;
  netPremiumUsd: number;
  feesUsd: number;
  realisedPnlUsd?: number;
  unrealisedUsd: number;
  openedAt: string;
};

const usd = (v: number) =>
  `${v < 0 ? "-" : ""}$${Math.abs(v).toLocaleString("en-US", { maximumFractionDigits: 2 })}`;

export default function CryptoFnoPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [activeId, setActiveId] = useState<string>("");
  const [chain, setChain] = useState<ChainRow[]>([]);
  const [spot, setSpot] = useState(0);
  const [expiry, setExpiry] = useState<string>("");
  const [basket, setBasket] = useState<Leg[]>([]);
  const [preview, setPreview] = useState<Preview | null>(null);
  const [positions, setPositions] = useState<Position[]>([]);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);

  // account editor
  const [editName, setEditName] = useState("");
  const [editCapital, setEditCapital] = useState("");
  const [newName, setNewName] = useState("");
  const [newCapital, setNewCapital] = useState("");

  const active = accounts.find((a) => a.id === activeId);

  const loadAccounts = useCallback(async () => {
    const r = await fetch("/api/crypto-fno/accounts", { cache: "no-store" });
    if (!r.ok) throw new Error(`accounts HTTP ${r.status}`);
    const list = (await r.json()) as Account[];
    setAccounts(list);
    setActiveId((cur) => (cur && list.some((a) => a.id === cur) ? cur : (list[0]?.id ?? "")));
  }, []);

  const loadChain = useCallback(async () => {
    const r = await fetch("/api/crypto-fno/chain?underlying=BTC", { cache: "no-store" });
    if (!r.ok) throw new Error(`chain HTTP ${r.status}`);
    const d = (await r.json()) as { contracts: ChainRow[]; spot: number };
    setChain(d.contracts ?? []);
    setSpot(d.spot ?? 0);
    setExpiry((cur) => {
      if (cur) return cur;
      const first = [...new Set((d.contracts ?? []).map((c) => c.expiry.slice(0, 10)))].sort()[0];
      return first ?? "";
    });
  }, []);

  const loadPositions = useCallback(async (id: string) => {
    if (!id) return;
    const r = await fetch(`/api/crypto-fno/positions?accountId=${encodeURIComponent(id)}`, {
      cache: "no-store",
    });
    if (!r.ok) return;
    setPositions((await r.json()) as Position[]);
  }, []);

  const refresh = useCallback(async () => {
    try {
      await Promise.all([loadAccounts(), loadChain()]);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "desk unreachable");
    } finally {
      setLoading(false);
    }
  }, [loadAccounts, loadChain]);

  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), 30_000);
    return () => clearInterval(t);
  }, [refresh]);

  useEffect(() => {
    void loadPositions(activeId);
  }, [activeId, loadPositions]);

  useEffect(() => {
    if (active) {
      setEditName(active.name);
      setEditCapital(String(active.initialCapitalUsd));
    }
  }, [active?.id]); // eslint-disable-line react-hooks/exhaustive-deps

  // Re-price the basket whenever it changes. This calls the same margin path the
  // engine enforces, so what is shown is what will be charged.
  useEffect(() => {
    if (!activeId || basket.length === 0) {
      setPreview(null);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const r = await fetch("/api/crypto-fno/preview", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ accountId: activeId, underlying: "BTC", legs: basket }),
        });
        const d = await r.json();
        if (!cancelled) {
          if (r.ok) {
            setPreview(d as Preview);
            setError("");
          } else {
            setPreview(null);
            setError(d?.error ?? `preview HTTP ${r.status}`);
          }
        }
      } catch {
        if (!cancelled) setPreview(null);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [activeId, basket]);

  const expiries = useMemo(
    () => [...new Set(chain.map((c) => c.expiry.slice(0, 10)))].sort(),
    [chain],
  );

  // Strike ladder for the selected expiry, calls and puts side by side.
  const ladder = useMemo(() => {
    const rows = chain.filter((c) => c.expiry.slice(0, 10) === expiry);
    const byStrike = new Map<number, { strike: number; call?: ChainRow; put?: ChainRow }>();
    for (const c of rows) {
      const e = byStrike.get(c.strike) ?? { strike: c.strike };
      if (c.type === "CALL") e.call = c;
      else e.put = c;
      byStrike.set(c.strike, e);
    }
    return [...byStrike.values()].sort((a, b) => a.strike - b.strike);
  }, [chain, expiry]);

  function legFor(symbol: string) {
    return basket.find((l) => l.symbol === symbol);
  }

  function toggleLeg(symbol: string, side: "buy" | "sell") {
    setMsg("");
    setBasket((cur) => {
      const existing = cur.find((l) => l.symbol === symbol);
      if (existing && existing.side === side) return cur.filter((l) => l.symbol !== symbol);
      if (existing) return cur.map((l) => (l.symbol === symbol ? { ...l, side } : l));
      return [...cur, { symbol, side, lots: 1 }];
    });
  }

  function setLots(symbol: string, lots: number) {
    setBasket((cur) => cur.map((l) => (l.symbol === symbol ? { ...l, lots: Math.max(1, lots) } : l)));
  }

  async function post(path: string, body: unknown): Promise<Record<string, unknown>> {
    const r = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const d = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(d?.error ?? `HTTP ${r.status}`);
    return d;
  }

  async function execute() {
    if (!activeId || basket.length === 0) return;
    setBusy(true);
    setError("");
    setMsg("");
    try {
      const pos = await post("/api/crypto-fno/execute", {
        accountId: activeId,
        underlying: "BTC",
        legs: basket,
      });
      const label = typeof pos.label === "string" ? pos.label : "basket";
      const margin = typeof pos.marginUsd === "number" ? pos.marginUsd : 0;
      setMsg(`Executed ${label} — margin ${usd(margin)}`);
      setBasket([]);
      await Promise.all([loadAccounts(), loadPositions(activeId)]);
    } catch (e) {
      setError(e instanceof Error ? e.message : "execution failed");
    } finally {
      setBusy(false);
    }
  }

  const engineStatus: DeskEngineStatus = loading ? "syncing" : error ? "degraded" : "live";

  const positionColumns: DeskColumn<Position>[] = [
    { id: "label", header: "Structure", cell: (p) => <span style={{ fontWeight: 600 }}>{p.label}</span> },
    {
      id: "legs",
      header: "Legs",
      cell: (p) => (
        <span className="desk-mono" style={{ fontSize: 12 }}>
          {p.legs.map((l) => `${l.side === "sell" ? "-" : "+"}${l.lots} ${l.type[0]}${l.strike}`).join("  ")}
        </span>
      ),
    },
    { id: "margin", align: "right", header: "Margin", cell: (p) => usd(p.marginUsd) },
    {
      id: "prem",
      align: "right",
      header: "Net premium",
      cell: (p) => (
        <span className={p.netPremiumUsd >= 0 ? "desk-pnl-positive" : "desk-pnl-negative"}>
          {usd(p.netPremiumUsd)}
        </span>
      ),
    },
    { id: "fees", align: "right", header: "Fees", cell: (p) => usd(p.feesUsd) },
    {
      id: "pnl",
      align: "right",
      header: "P&L",
      cell: (p) => {
        const v = p.status === "OPEN" ? p.unrealisedUsd : (p.realisedPnlUsd ?? 0);
        return (
          <span className={v >= 0 ? "desk-pnl-positive" : "desk-pnl-negative"} style={{ fontWeight: 600 }}>
            {usd(v)}
          </span>
        );
      },
    },
    { id: "status", header: "Status", cell: (p) => <DeskChip label={p.status} /> },
    {
      id: "act",
      align: "right",
      header: "",
      cell: (p) =>
        p.status === "OPEN" ? (
          <DeskButton
            variant="text"
            onClick={async () => {
              try {
                await post("/api/crypto-fno/close", { accountId: activeId, positionId: p.id });
                await Promise.all([loadAccounts(), loadPositions(activeId)]);
              } catch (e) {
                setError(e instanceof Error ? e.message : "close failed");
              }
            }}
          >
            Close
          </DeskButton>
        ) : null,
    },
  ];

  return (
    <div className="desk-root">
      <main style={{ display: "flex", flexDirection: "column", gap: 20, padding: 20 }}>
        <div>
          <Link href="/" className="desk-label-md">
            Home
          </Link>{" "}
          <span className="desk-label-md">› Crypto F&O</span>
          <h1 style={{ display: "flex", alignItems: "center", gap: 12, margin: "6px 0 0" }}>
            Crypto F&O
            <StatusBadge status={engineStatus} />
            <DeskChip label="PAPER" />
          </h1>
          <p className="desk-label-md" style={{ maxWidth: 900 }}>
            Multi-leg paper trading on the live Delta BTC option chain. Margin is computed on the
            whole basket, so hedges earn credit — selling a call and a put reserves a lot, and
            buying wings against them collapses the requirement. No order reaches a broker.
          </p>
        </div>

        {error && <DeskBanner variant="error">{error}</DeskBanner>}
        {msg && <DeskBanner variant="success">{msg}</DeskBanner>}

        {/* Accounts */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Accounts"
            subtitle={`${accounts.length} book${accounts.length === 1 ? "" : "s"} · BTC spot ${usd(spot)}`}
          />
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 16 }}>
            {accounts.map((a) => (
              <button
                key={a.id}
                onClick={() => setActiveId(a.id)}
                style={{
                  padding: "8px 14px",
                  borderRadius: 8,
                  border: `1px solid ${a.id === activeId ? "var(--desk-primary)" : "var(--desk-outline)"}`,
                  background: a.id === activeId ? "var(--desk-primary-container)" : "transparent",
                  color: "var(--desk-on-surface)",
                  cursor: "pointer",
                  fontWeight: a.id === activeId ? 700 : 400,
                }}
              >
                {a.name}
                <span className="desk-mono" style={{ opacity: 0.7, marginLeft: 8, fontSize: 12 }}>
                  {usd(a.equityUsd)}
                </span>
              </button>
            ))}
          </div>

          {active && (
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(150px,1fr))", gap: 12 }}>
              <DeskMetricTile label="Equity" value={usd(active.equityUsd)} />
              <DeskMetricTile label="Available" value={usd(active.availableUsd)} detail="free for new baskets" />
              <DeskMetricTile label="Margin used" value={usd(active.marginUsedUsd)} />
              <DeskMetricTile label="Realised" value={usd(active.realisedPnlUsd)} />
              <DeskMetricTile label="Unrealised" value={usd(active.unrealisedPnlUsd)} />
              <DeskMetricTile label="Open baskets" value={String(active.openBaskets)} />
            </div>
          )}

          {/* Edit + create */}
          <div style={{ display: "flex", gap: 24, flexWrap: "wrap", marginTop: 20 }}>
            <div style={{ display: "flex", gap: 8, alignItems: "flex-end", flexWrap: "wrap" }}>
              <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <span className="desk-label-sm">Account name</span>
                <input
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  style={inputStyle}
                />
              </label>
              <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <span className="desk-label-sm">Balance (USD)</span>
                <input
                  type="number"
                  value={editCapital}
                  onChange={(e) => setEditCapital(e.target.value)}
                  className="desk-mono"
                  style={inputStyle}
                />
              </label>
              <DeskButton
                variant="outlined"
                disabled={!activeId || busy}
                onClick={async () => {
                  try {
                    await post("/api/crypto-fno/accounts/edit", {
                      accountId: activeId,
                      name: editName,
                      capitalUsd: Number(editCapital) || 0,
                    });
                    setMsg("Account updated");
                    await loadAccounts();
                  } catch (e) {
                    setError(e instanceof Error ? e.message : "edit failed");
                  }
                }}
              >
                Save changes
              </DeskButton>
            </div>

            <div style={{ display: "flex", gap: 8, alignItems: "flex-end", flexWrap: "wrap" }}>
              <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <span className="desk-label-sm">New account</span>
                <input
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="name"
                  style={inputStyle}
                />
              </label>
              <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <span className="desk-label-sm">Capital (USD)</span>
                <input
                  type="number"
                  value={newCapital}
                  onChange={(e) => setNewCapital(e.target.value)}
                  placeholder="100000"
                  className="desk-mono"
                  style={inputStyle}
                />
              </label>
              <DeskButton
                variant="filled"
                disabled={!newName.trim() || busy}
                onClick={async () => {
                  try {
                    await post("/api/crypto-fno/accounts", {
                      name: newName,
                      capitalUsd: Number(newCapital) || 0,
                    });
                    setNewName("");
                    setNewCapital("");
                    setMsg("Account created");
                    await loadAccounts();
                  } catch (e) {
                    setError(e instanceof Error ? e.message : "create failed");
                  }
                }}
              >
                Add account
              </DeskButton>
            </div>
          </div>
        </DeskCard>

        {/* Basket ticket */}
        {basket.length > 0 && (
          <DeskCard padding="md" variant="outlined">
            <DeskSectionHeader
              title={preview?.label ?? "Basket"}
              subtitle={`${basket.length} leg${basket.length === 1 ? "" : "s"}`}
              actions={
                <DeskButton variant="text" onClick={() => setBasket([])}>
                  Clear
                </DeskButton>
              }
            />
            <div style={{ display: "flex", flexDirection: "column", gap: 6, marginBottom: 14 }}>
              {basket.map((l) => (
                <div key={l.symbol} style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <DeskChip label={l.side === "sell" ? "SELL" : "BUY"} tone={l.side === "sell" ? "error" : "success"} />
                  <span className="desk-mono" style={{ flex: 1 }}>{l.symbol}</span>
                  <input
                    type="number"
                    min={1}
                    value={l.lots}
                    onChange={(e) => setLots(l.symbol, Number(e.target.value))}
                    className="desk-mono"
                    style={{ ...inputStyle, width: 80 }}
                  />
                  <DeskButton variant="text" onClick={() => toggleLeg(l.symbol, l.side)}>
                    Remove
                  </DeskButton>
                </div>
              ))}
            </div>

            {preview && (
              <>
                <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(150px,1fr))", gap: 12 }}>
                  <DeskMetricTile
                    label="Margin required"
                    value={usd(preview.margin.requiredUsd)}
                    detail={preview.margin.basis}
                  />
                  <DeskMetricTile
                    label="Hedge credit"
                    value={usd(preview.margin.hedgeCreditUsd)}
                    detail={`vs ${usd(preview.margin.standaloneUsd)} leg-by-leg`}
                  />
                  <DeskMetricTile
                    label="Net premium"
                    value={usd(preview.margin.netPremiumUsd)}
                    detail={preview.margin.netPremiumUsd >= 0 ? "credit received" : "debit paid"}
                  />
                  <DeskMetricTile
                    label="Worst case"
                    value={usd(-preview.margin.worstCaseLossUsd)}
                    detail={`at spot ${usd(preview.margin.worstCaseSpot)}`}
                  />
                  <DeskMetricTile label="Available" value={usd(preview.account.availableUsd)} />
                </div>

                {!preview.affordable && (
                  <DeskBanner variant="error">
                    Not enough capital — this basket needs {usd(preview.margin.requiredUsd)} but only{" "}
                    {usd(preview.account.availableUsd)} is available (short {usd(preview.shortfallUsd)}).
                    Add a hedge to reduce the requirement, cut lots, or raise the account balance.
                  </DeskBanner>
                )}

                <div style={{ marginTop: 14 }}>
                  <DeskButton variant="filled" disabled={busy || !preview.affordable} onClick={() => void execute()}>
                    {busy ? "Executing…" : `Execute ${preview.label}`}
                  </DeskButton>
                </div>
              </>
            )}
          </DeskCard>
        )}

        {/* Option chain */}
        <DeskCard padding="md">
          <DeskSectionHeader
            title="Option chain"
            subtitle={`${ladder.length} strikes · click BUY or SELL to build a basket`}
            actions={
              <select
                value={expiry}
                onChange={(e) => setExpiry(e.target.value)}
                style={inputStyle}
              >
                {expiries.map((x) => (
                  <option key={x} value={x}>
                    {x}
                  </option>
                ))}
              </select>
            }
          />
          <div style={{ overflowX: "auto" }}>
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead>
                <tr className="desk-label-sm">
                  <th style={th}>Call</th>
                  <th style={{ ...th, textAlign: "right" }}>Bid</th>
                  <th style={{ ...th, textAlign: "right" }}>Ask</th>
                  <th style={{ ...th, textAlign: "center" }}>Strike</th>
                  <th style={{ ...th, textAlign: "right" }}>Bid</th>
                  <th style={{ ...th, textAlign: "right" }}>Ask</th>
                  <th style={th}>Put</th>
                </tr>
              </thead>
              <tbody>
                {ladder.map((row) => {
                  const atm = Math.abs(row.strike - spot) < 500;
                  return (
                    <tr
                      key={row.strike}
                      style={{
                        borderTop: "1px solid var(--desk-outline)",
                        background: atm ? "var(--desk-primary-container)" : undefined,
                      }}
                    >
                      <td style={td}>{row.call && <LegButtons row={row.call} leg={legFor(row.call.symbol)} onPick={toggleLeg} />}</td>
                      <td style={{ ...td, textAlign: "right" }} className="desk-mono">
                        {row.call?.bid ? row.call.bid.toFixed(0) : "—"}
                      </td>
                      <td style={{ ...td, textAlign: "right" }} className="desk-mono">
                        {row.call?.ask ? row.call.ask.toFixed(0) : "—"}
                      </td>
                      <td style={{ ...td, textAlign: "center", fontWeight: 700 }} className="desk-mono">
                        {row.strike.toFixed(0)}
                      </td>
                      <td style={{ ...td, textAlign: "right" }} className="desk-mono">
                        {row.put?.bid ? row.put.bid.toFixed(0) : "—"}
                      </td>
                      <td style={{ ...td, textAlign: "right" }} className="desk-mono">
                        {row.put?.ask ? row.put.ask.toFixed(0) : "—"}
                      </td>
                      <td style={td}>{row.put && <LegButtons row={row.put} leg={legFor(row.put.symbol)} onPick={toggleLeg} />}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            {ladder.length === 0 && (
              <DeskEmptyState title="No chain" subtitle="Waiting for the live Delta option chain." />
            )}
          </div>
        </DeskCard>

        {/* Positions */}
        <DeskCard padding="md">
          <DeskSectionHeader title="Positions" subtitle={`${positions.length} in ${active?.name ?? "—"}`} />
          <DeskDataTable
            columns={positionColumns}
            rows={positions}
            getRowKey={(p) => p.id}
            minWidth={900}
            empty={<DeskEmptyState title="No positions" subtitle="Build a basket from the chain above." />}
          />
        </DeskCard>
      </main>
    </div>
  );
}

function LegButtons({
  row,
  leg,
  onPick,
}: {
  row: ChainRow;
  leg?: Leg;
  onPick: (symbol: string, side: "buy" | "sell") => void;
}) {
  return (
    <span style={{ display: "inline-flex", gap: 4 }}>
      <button
        onClick={() => onPick(row.symbol, "buy")}
        style={pill(leg?.side === "buy", "var(--desk-success, #16a34a)")}
        title={`Buy ${row.symbol}`}
      >
        B
      </button>
      <button
        onClick={() => onPick(row.symbol, "sell")}
        style={pill(leg?.side === "sell", "var(--desk-error, #dc2626)")}
        title={`Sell ${row.symbol}`}
      >
        S
      </button>
    </span>
  );
}

function pill(active: boolean, colour: string): React.CSSProperties {
  return {
    width: 26,
    height: 24,
    borderRadius: 6,
    border: `1px solid ${active ? colour : "var(--desk-outline)"}`,
    background: active ? colour : "transparent",
    color: active ? "#fff" : "var(--desk-on-surface-variant)",
    fontWeight: 700,
    fontSize: 11,
    cursor: "pointer",
  };
}

const inputStyle: React.CSSProperties = {
  padding: "8px 10px",
  borderRadius: 8,
  border: "1px solid var(--desk-outline)",
  background: "var(--desk-surface)",
  color: "var(--desk-on-surface)",
  fontSize: 14,
};

const th: React.CSSProperties = { padding: "8px 10px", textAlign: "left" };
const td: React.CSSProperties = { padding: "6px 10px" };
