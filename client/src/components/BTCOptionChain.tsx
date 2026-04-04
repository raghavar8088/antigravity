"use client";
import { useRef } from "react";
import useOptionChain, { ChainRow, ChainLeg } from "@/hooks/useOptionChain";

// ── Formatters ────────────────────────────────────────────────────────────────

const f2 = (n: number) => n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const f4 = (n: number) => n.toLocaleString(undefined, { minimumFractionDigits: 4, maximumFractionDigits: 4 });
const fInt = (n: number) => Math.round(n).toLocaleString();
const fIV = (n: number) => n.toFixed(1) + "%";
const fDelta = (n: number) => (n >= 0 ? "+" : "") + n.toFixed(3);
const fStrike = (n: number) => "$" + fInt(n);
const fMark = (n: number) => "$" + f2(n);

// ── Sub-components ────────────────────────────────────────────────────────────

function CallCell({ value, accent }: { value: string; accent?: string }) {
  return (
    <td className={`px-2 py-[7px] text-right font-mono text-xs tabular-nums ${accent ?? "text-zinc-300"}`}>
      {value}
    </td>
  );
}

function PutCell({ value, accent }: { value: string; accent?: string }) {
  return (
    <td className={`px-2 py-[7px] text-left font-mono text-xs tabular-nums ${accent ?? "text-zinc-300"}`}>
      {value}
    </td>
  );
}

function IVCell({ iv, side }: { iv: number; side: "call" | "put" }) {
  const color = side === "call" ? "text-emerald-400" : "text-rose-400";
  return (
    <td className={`px-2 py-[7px] ${side === "call" ? "text-right" : "text-left"} font-mono text-xs font-semibold ${color}`}>
      {fIV(iv)}
    </td>
  );
}

function ChainRowComponent({ row, atmRef }: { row: ChainRow; atmRef?: React.RefObject<HTMLTableRowElement | null> }) {
  const c = row.call;
  const p = row.put;

  const callBg = row.isAtm
    ? "bg-amber-500/5"
    : c.isItm
    ? "bg-emerald-500/[0.04]"
    : "";
  const putBg = row.isAtm
    ? "bg-amber-500/5"
    : p.isItm
    ? "bg-rose-500/[0.04]"
    : "";
  const atmBorder = row.isAtm ? "border-y border-amber-500/40" : "";

  return (
    <tr
      ref={row.isAtm ? atmRef : undefined}
      className={`group transition-colors hover:bg-white/[0.025] ${atmBorder}`}
    >
      {/* ── CALLS ── */}
      <IVCell iv={c.iv} side="call" />
      <CallCell value={fDelta(c.delta)} accent={c.isItm ? "text-emerald-300" : "text-zinc-400"} />
      <CallCell value={f4(c.gamma)} accent="text-zinc-500" />
      <CallCell value={f2(c.theta)} accent="text-zinc-500" />
      <CallCell value={f2(c.vega)} accent="text-zinc-500" />
      <CallCell value={fInt(c.oi)} accent="text-zinc-400" />
      <CallCell value={fInt(c.volume)} accent="text-zinc-500" />
      <CallCell value={f2(c.bid)} accent="text-sky-400" />
      <CallCell value={f2(c.ask)} accent="text-orange-400" />
      <td className={`px-3 py-[7px] text-right font-mono text-xs font-semibold ${callBg} ${c.isItm ? "text-emerald-300" : "text-zinc-200"}`}>
        {fMark(c.mark)}
      </td>

      {/* ── STRIKE ── */}
      <td className="relative px-0">
        <div className={`flex flex-col items-center justify-center px-3 py-[7px] min-w-[110px] ${row.isAtm ? "bg-amber-500/10" : ""}`}>
          <span className={`font-mono text-sm font-bold tracking-wide ${row.isAtm ? "text-amber-400" : "text-white"}`}>
            {fStrike(row.strike)}
          </span>
          <span className="text-[10px] mt-0.5 font-medium" style={{ color: row.moneynessPC === 0 ? "#f59e0b" : row.moneynessPC > 0 ? "#6ee7b7" : "#fca5a5" }}>
            {row.isAtm ? "ATM" : (row.moneynessPC > 0 ? "+" : "") + row.moneynessPC.toFixed(1) + "%"}
          </span>
        </div>
      </td>

      {/* ── PUTS ── */}
      <td className={`px-3 py-[7px] text-left font-mono text-xs font-semibold ${putBg} ${p.isItm ? "text-rose-300" : "text-zinc-200"}`}>
        {fMark(p.mark)}
      </td>
      <PutCell value={f2(p.bid)} accent="text-sky-400" />
      <PutCell value={f2(p.ask)} accent="text-orange-400" />
      <PutCell value={fInt(p.volume)} accent="text-zinc-500" />
      <PutCell value={fInt(p.oi)} accent="text-zinc-400" />
      <PutCell value={f2(p.vega)} accent="text-zinc-500" />
      <PutCell value={f2(p.theta)} accent="text-zinc-500" />
      <PutCell value={f4(p.gamma)} accent="text-zinc-500" />
      <PutCell value={fDelta(p.delta)} accent={p.isItm ? "text-rose-300" : "text-zinc-400"} />
      <IVCell iv={p.iv} side="put" />
    </tr>
  );
}

function ColHeader({ label, align = "right" }: { label: string; align?: "right" | "left" | "center" }) {
  return (
    <th
      className={`px-2 pb-2 pt-3 text-${align} text-[10px] font-bold uppercase tracking-[0.15em] whitespace-nowrap`}
      style={{ color: "var(--text-muted)" }}
    >
      {label}
    </th>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export default function BTCOptionChain() {
  const { data, loading, selectedExpiry, selectExpiry } = useOptionChain();
  const atmRowRef = useRef<HTMLTableRowElement>(null);

  const scrollToATM = () => {
    atmRowRef.current?.scrollIntoView({ block: "center", behavior: "smooth" });
  };

  if (loading) {
    return (
      <div className="glass-panel p-12 text-center" style={{ color: "var(--text-muted)" }}>
        Loading option chain…
      </div>
    );
  }

  if (!data || data.chain.length === 0) {
    return (
      <div className="glass-panel p-12 text-center" style={{ color: "var(--text-muted)" }}>
        Option chain unavailable — waiting for BTC price feed.
      </div>
    );
  }

  return (
    <div className="space-y-4">

      {/* ── Top bar ── */}
      <div className="glass-panel px-5 py-4 flex flex-wrap items-center gap-4 justify-between">
        <div className="flex items-center gap-5">
          {/* BTC Price */}
          <div>
            <div style={{ fontSize: 10, fontWeight: 700, letterSpacing: "0.18em", color: "var(--text-muted)", textTransform: "uppercase" }}>
              BTC / USDT
            </div>
            <div className="mt-0.5 text-2xl font-bold text-white font-mono">
              ${data.underlyingPrice.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
            </div>
          </div>
          {/* Base IV */}
          <div className="border-l pl-5" style={{ borderColor: "var(--border)" }}>
            <div style={{ fontSize: 10, fontWeight: 700, letterSpacing: "0.18em", color: "var(--text-muted)", textTransform: "uppercase" }}>
              ATM IV
            </div>
            <div className="mt-0.5 text-xl font-bold text-amber-400 font-mono">
              {data.baseIv.toFixed(1)}%
            </div>
          </div>
          {/* DTE */}
          <div className="border-l pl-5" style={{ borderColor: "var(--border)" }}>
            <div style={{ fontSize: 10, fontWeight: 700, letterSpacing: "0.18em", color: "var(--text-muted)", textTransform: "uppercase" }}>
              Expiry
            </div>
            <div className="mt-0.5 text-base font-semibold text-white">
              {data.expiryLabel}
              <span className="ml-2 text-xs text-zinc-500">{data.dte}d</span>
            </div>
          </div>
        </div>

        {/* Controls */}
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={scrollToATM}
            className="btn-primary text-xs px-3 py-1.5"
          >
            Jump to ATM
          </button>
        </div>
      </div>

      {/* ── Expiry selector ── */}
      <div className="flex flex-wrap gap-2">
        {data.expiries.map((ex) => (
          <button
            key={ex.value}
            type="button"
            onClick={() => selectExpiry(ex.value)}
            className={`rounded-xl border px-4 py-2 text-xs font-semibold transition-colors ${
              selectedExpiry === ex.value || (!selectedExpiry && ex.value === data.selectedExpiry)
                ? "border-amber-500/40 bg-amber-500/10 text-amber-300"
                : "border-zinc-700/60 bg-zinc-900/60 text-zinc-400 hover:text-zinc-200"
            }`}
          >
            {ex.label}
            <span className="ml-1.5 opacity-60">{ex.dte}d</span>
          </button>
        ))}
      </div>

      {/* ── Legend ── */}
      <div className="flex items-center gap-4 text-[11px]" style={{ color: "var(--text-muted)" }}>
        <span className="flex items-center gap-1.5">
          <span className="inline-block w-3 h-3 rounded-sm bg-emerald-500/20 border border-emerald-500/30" />
          ITM Call
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block w-3 h-3 rounded-sm bg-rose-500/20 border border-rose-500/30" />
          ITM Put
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block w-3 h-3 rounded-sm bg-amber-500/20 border border-amber-500/30" />
          ATM
        </span>
        <span className="ml-4 text-sky-400">Bid</span>
        <span className="text-orange-400">Ask</span>
        <span className="text-emerald-400">Call IV</span>
        <span className="text-rose-400">Put IV</span>
      </div>

      {/* ── Option chain table ── */}
      <div className="glass-panel overflow-x-auto">
        <table className="w-full border-collapse" style={{ minWidth: 1100 }}>
          <thead>
            <tr style={{ background: "var(--surface-2)", borderBottom: "1px solid var(--border)" }}>
              {/* CALLS header */}
              <th colSpan={10} className="px-4 py-2 text-center text-xs font-bold uppercase tracking-widest text-emerald-400 border-r" style={{ borderColor: "var(--border)" }}>
                CALLS
              </th>
              {/* Strike */}
              <th className="px-3 py-2 text-center text-xs font-bold uppercase tracking-widest text-white">
                STRIKE
              </th>
              {/* PUTS header */}
              <th colSpan={10} className="px-4 py-2 text-center text-xs font-bold uppercase tracking-widest text-rose-400 border-l" style={{ borderColor: "var(--border)" }}>
                PUTS
              </th>
            </tr>
            <tr style={{ background: "var(--surface-2)", borderBottom: "1px solid var(--border)" }}>
              {/* Call columns (right-aligned, reversed order — IV first = leftmost) */}
              <ColHeader label="IV" />
              <ColHeader label="Delta" />
              <ColHeader label="Gamma" />
              <ColHeader label="Theta" />
              <ColHeader label="Vega" />
              <ColHeader label="OI" />
              <ColHeader label="Volume" />
              <ColHeader label="Bid" />
              <ColHeader label="Ask" />
              <ColHeader label="Mark" />
              {/* Strike */}
              <th className="px-3 pb-2 pt-3 text-center text-[10px] font-bold uppercase tracking-[0.15em]" style={{ color: "var(--text-muted)" }}>
                Strike / %
              </th>
              {/* Put columns (left-aligned, mirror of calls) */}
              <ColHeader label="Mark" align="left" />
              <ColHeader label="Bid" align="left" />
              <ColHeader label="Ask" align="left" />
              <ColHeader label="Volume" align="left" />
              <ColHeader label="OI" align="left" />
              <ColHeader label="Vega" align="left" />
              <ColHeader label="Theta" align="left" />
              <ColHeader label="Gamma" align="left" />
              <ColHeader label="Delta" align="left" />
              <ColHeader label="IV" align="left" />
            </tr>
          </thead>
          <tbody className="divide-y">
            {data.chain.map((row) => (
              <ChainRowComponent key={row.strike} row={row} atmRef={row.isAtm ? atmRowRef : undefined} />
            ))}
          </tbody>
        </table>
      </div>

      {/* ── Footer note ── */}
      <div className="text-center text-[11px]" style={{ color: "var(--text-muted)" }}>
        Prices calculated via Black-Scholes · IV from BTC realised volatility · OI & Volume simulated · Auto-refresh every 3s
      </div>
    </div>
  );
}
