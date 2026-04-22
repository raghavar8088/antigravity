"use client";

type Props = {
  /** Last BTCUSDT spot price from workspace feed (0 = unknown). */
  btcSpotUsd?: number;
  /** 24h change in percent points (e.g. 1.25 = +1.25%). */
  btcChange24hPct?: number;
};

function fmtUsdPlain(n: number) {
  return n.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export default function BtcSpotStrip({ btcSpotUsd = 0, btcChange24hPct }: Props) {
  const hasPrice = btcSpotUsd > 0;
  const ch = btcChange24hPct;
  const hasCh = ch != null && Number.isFinite(ch);

  if (!hasPrice) {
    return (
      <div className="mt-1.5 px-0.5 text-xs" style={{ color: "var(--text-muted)" }}>
        <span className="text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-500">BTC / USDT</span>
        {" "}
        Live spot loading…
      </div>
    );
  }

  const positive = (ch ?? 0) >= 0;
  const pctLabel = hasCh ? `${positive ? "+" : "-"}${Math.abs(ch as number).toFixed(2)}%` : "—";

  return (
    <div className="mt-1.5 flex flex-wrap items-baseline gap-x-2 gap-y-0.5 px-0.5">
      <span className="text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-500">BTC / USDT</span>
      <span className="font-mono text-sm font-semibold tabular-nums" style={{ color: "var(--text-primary)" }}>
        ${fmtUsdPlain(btcSpotUsd)}
      </span>
      {hasCh ? (
        <span className={`text-xs font-semibold tabular-nums ${positive ? "text-emerald-600" : "text-rose-600"}`}>
          24h {pctLabel}
        </span>
      ) : null}
    </div>
  );
}
