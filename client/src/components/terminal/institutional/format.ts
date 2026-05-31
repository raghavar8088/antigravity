export function usd(value: number, opts: { signed?: boolean; compact?: boolean } = {}) {
  const sign = opts.signed && value > 0 ? "+" : value < 0 ? "-" : "";
  const abs = Math.abs(value);
  if (opts.compact && abs >= 1_000_000) return `${sign}$${(abs / 1_000_000).toFixed(2)}M`;
  if (opts.compact && abs >= 1_000) return `${sign}$${(abs / 1_000).toFixed(1)}K`;
  return `${sign}$${abs.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

export function px(value: number) {
  return value.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export function pct(value: number, digits = 2) {
  return `${value >= 0 ? "+" : ""}${value.toFixed(digits)}%`;
}

export function pnlClass(value: number) {
  return value >= 0 ? "text-emerald-400" : "text-rose-400";
}
