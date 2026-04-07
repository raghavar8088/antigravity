"use client";

import type { ChainData, ChainLeg, ChainRow } from "@/hooks/useOptionChain";

type Props = {
  data: ChainData | null;
  loading: boolean;
  selectedExpiry: string;
  selectExpiry: (value: string) => void;
};

function fmtMoney(value: number, decimals = 2) {
  return `₹${value.toLocaleString("en-IN", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  })}`;
}

function fmtInt(value: number) {
  return Math.round(value).toLocaleString("en-IN");
}

function fmtPercent(value: number, decimals = 1) {
  return `${value.toFixed(decimals)}%`;
}

function LegCell({
  leg,
  side,
  field,
}: {
  leg: ChainLeg;
  side: "call" | "put";
  field: "oi" | "iv" | "bid" | "ask" | "mark";
}) {
  const alignClass = side === "call" ? "text-right" : "text-left";
  const accentClass = {
    oi: "text-zinc-600",
    iv: side === "call" ? "text-emerald-600" : "text-rose-600",
    bid: "text-sky-700",
    ask: "text-orange-700",
    mark: leg.isItm ? (side === "call" ? "text-emerald-700" : "text-rose-700") : "text-zinc-900",
  }[field];

  const value = {
    oi: fmtInt(leg.oi),
    iv: fmtPercent(leg.iv),
    bid: fmtMoney(leg.bid),
    ask: fmtMoney(leg.ask),
    mark: fmtMoney(leg.mark),
  }[field];

  return (
    <td className={`px-3 py-3 font-mono text-xs tabular-nums ${alignClass} ${accentClass}`}>
      {value}
    </td>
  );
}

function MetricCard({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div
      className="rounded-[18px] border px-4 py-4"
      style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}
    >
      <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-500">{label}</div>
      <div className="mt-2 text-xl font-semibold text-zinc-900">{value}</div>
      <div className="mt-1 text-xs text-zinc-500">{detail}</div>
    </div>
  );
}

function findAtmIndex(chain: ChainRow[]) {
  const explicit = chain.findIndex((row) => row.isAtm);
  if (explicit >= 0) {
    return explicit;
  }
  return Math.max(
    0,
    chain.reduce((bestIndex, row, index) => {
      const bestDistance = Math.abs(chain[bestIndex].moneynessPC);
      const currentDistance = Math.abs(row.moneynessPC);
      return currentDistance < bestDistance ? index : bestIndex;
    }, 0),
  );
}

export default function NiftyOptionChainPanel({
  data,
  loading,
  selectedExpiry,
  selectExpiry,
}: Props) {
  if (loading && !data) {
    return (
      <div className="glass-panel px-6 py-12 text-center text-sm" style={{ color: "var(--text-muted)" }}>
        Loading NIFTY option chain...
      </div>
    );
  }

  if (!data || data.chain.length === 0) {
    return (
      <div className="glass-panel px-6 py-12 text-center text-sm" style={{ color: "var(--text-muted)" }}>
        NIFTY option chain is waiting for the live spot feed.
      </div>
    );
  }

  const atmIndex = findAtmIndex(data.chain);
  const atmRow = data.chain[atmIndex] ?? data.chain[0];
  const visibleRows = data.chain.slice(Math.max(0, atmIndex - 5), Math.min(data.chain.length, atmIndex + 6));

  const totalCallOI = data.chain.reduce((sum, row) => sum + row.call.oi, 0);
  const totalPutOI = data.chain.reduce((sum, row) => sum + row.put.oi, 0);
  const callWall = data.chain.reduce((best, row) => (row.call.oi > best.call.oi ? row : best), data.chain[0]);
  const putFloor = data.chain.reduce((best, row) => (row.put.oi > best.put.oi ? row : best), data.chain[0]);

  const upsideWing = data.chain.find((row) => row.strike > data.underlyingPrice) ?? atmRow;
  const downsideWing = [...data.chain].reverse().find((row) => row.strike < data.underlyingPrice) ?? atmRow;
  const wingSkew = downsideWing.put.iv - upsideWing.call.iv;
  const atmStraddle = atmRow.call.mark + atmRow.put.mark;
  const putCallRatio = totalCallOI > 0 ? totalPutOI / totalCallOI : 0;

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <div className="flex flex-col gap-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
              NIFTY 50 Option Chain
            </div>
            <div className="mt-2 text-2xl font-semibold text-zinc-900">
              {fmtMoney(data.underlyingPrice)}
            </div>
            <div className="mt-1 text-sm" style={{ color: "var(--text-secondary)" }}>
              Angel One live data · weekly NIFTY option chain.
            </div>
          </div>
          <div
            className="rounded-full border px-4 py-2 text-xs font-medium"
            style={{ borderColor: "rgba(245, 158, 11, 0.22)", background: "rgba(245, 158, 11, 0.08)", color: "#b45309" }}
          >
            {data.expiryLabel} · {data.dte}DTE
          </div>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-6">
          <MetricCard label="Spot" value={fmtMoney(data.underlyingPrice)} detail="NSE NIFTY 50 live spot" />
          <MetricCard label="ATM Straddle" value={fmtMoney(atmStraddle)} detail={`Strike ${fmtMoney(atmRow.strike, 0)}`} />
          <MetricCard label="ATM IV" value={fmtPercent(data.baseIv)} detail="Realized-vol anchor" />
          <MetricCard label="Put/Call OI" value={putCallRatio.toFixed(2)} detail={`${fmtInt(totalPutOI)} puts vs ${fmtInt(totalCallOI)} calls`} />
          <MetricCard label="Call Wall" value={fmtMoney(callWall.strike, 0)} detail={`${fmtInt(callWall.call.oi)} call OI`} />
          <MetricCard label="Put Floor" value={fmtMoney(putFloor.strike, 0)} detail={`${fmtInt(putFloor.put.oi)} put OI`} />
        </div>

        <div className="flex flex-wrap gap-2">
          {data.expiries.map((expiry) => {
            const active = selectedExpiry === expiry.value || (!selectedExpiry && expiry.value === data.selectedExpiry);
            return (
              <button
                key={expiry.value}
                type="button"
                onClick={() => selectExpiry(expiry.value)}
                className="rounded-full border px-4 py-2 text-xs font-semibold transition-colors"
                style={{
                  borderColor: active ? "rgba(37, 99, 235, 0.22)" : "var(--border)",
                  background: active ? "rgba(37, 99, 235, 0.08)" : "var(--surface-2)",
                  color: active ? "#1d4ed8" : "var(--text-secondary)",
                }}
              >
                {expiry.label} <span className="opacity-60">{expiry.dte}d</span>
              </button>
            );
          })}
        </div>

        <div className="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_220px]">
          <div className="overflow-x-auto rounded-[22px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full text-sm" style={{ minWidth: 920 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[10px] uppercase tracking-[0.16em]">
                  <th className="px-3 py-3 text-right font-semibold text-emerald-700" colSpan={5}>Calls</th>
                  <th className="px-3 py-3 text-center font-semibold text-zinc-600">Strike</th>
                  <th className="px-3 py-3 text-left font-semibold text-rose-700" colSpan={5}>Puts</th>
                </tr>
                <tr className="text-[10px] uppercase tracking-[0.16em]">
                  <th className="px-3 pb-3 text-right font-semibold">OI</th>
                  <th className="px-3 pb-3 text-right font-semibold">IV</th>
                  <th className="px-3 pb-3 text-right font-semibold">Bid</th>
                  <th className="px-3 pb-3 text-right font-semibold">Ask</th>
                  <th className="px-3 pb-3 text-right font-semibold">Mark</th>
                  <th className="px-3 pb-3 text-center font-semibold">Level</th>
                  <th className="px-3 pb-3 text-left font-semibold">Mark</th>
                  <th className="px-3 pb-3 text-left font-semibold">Bid</th>
                  <th className="px-3 pb-3 text-left font-semibold">Ask</th>
                  <th className="px-3 pb-3 text-left font-semibold">IV</th>
                  <th className="px-3 pb-3 text-left font-semibold">OI</th>
                </tr>
              </thead>
              <tbody className="divide-y" style={{ color: "var(--text-primary)" }}>
                {visibleRows.map((row) => (
                  <tr
                    key={row.strike}
                    style={{
                      background: row.isAtm ? "rgba(245, 158, 11, 0.08)" : "transparent",
                    }}
                  >
                    <LegCell leg={row.call} side="call" field="oi" />
                    <LegCell leg={row.call} side="call" field="iv" />
                    <LegCell leg={row.call} side="call" field="bid" />
                    <LegCell leg={row.call} side="call" field="ask" />
                    <LegCell leg={row.call} side="call" field="mark" />
                    <td className="px-3 py-3 text-center">
                      <div className={`font-mono text-sm font-semibold ${row.isAtm ? "text-amber-600" : "text-zinc-900"}`}>
                        {fmtMoney(row.strike, 0)}
                      </div>
                      <div className="mt-0.5 text-[11px]" style={{ color: row.isAtm ? "#b45309" : "var(--text-muted)" }}>
                        {row.isAtm ? "ATM" : `${row.moneynessPC > 0 ? "+" : ""}${row.moneynessPC.toFixed(1)}%`}
                      </div>
                    </td>
                    <LegCell leg={row.put} side="put" field="mark" />
                    <LegCell leg={row.put} side="put" field="bid" />
                    <LegCell leg={row.put} side="put" field="ask" />
                    <LegCell leg={row.put} side="put" field="iv" />
                    <LegCell leg={row.put} side="put" field="oi" />
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="grid grid-cols-1 gap-3">
            <MetricCard label="Wing Skew" value={fmtPercent(wingSkew)} detail={`${fmtMoney(downsideWing.strike, 0)} put vs ${fmtMoney(upsideWing.strike, 0)} call`} />
            <MetricCard label="Rows Shown" value={`${visibleRows.length}`} detail="Centered around ATM" />
            <MetricCard label="Refresh" value="3s" detail="Auto-updating chain and expiry ladder" />
            <div
              className="rounded-[18px] border px-4 py-4 text-sm"
              style={{ borderColor: "var(--border)", background: "var(--surface-2)", color: "var(--text-secondary)" }}
            >
              Premiums, IV, and liquidity are refreshed against the live NIFTY spot feed using the NIFTY-specific expiry calendar and strike spacing.
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
