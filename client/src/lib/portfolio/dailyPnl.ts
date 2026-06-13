export type ClosedTradeLike = {
  exitTime: string;
  netPnl: number;
};

export type DailyPnlRow = {
  dateKey: string;
  label: string;
  trades: number;
  wins: number;
  losses: number;
  pnl: number;
  pnlPct: number;
  startEquity: number;
  endEquity: number;
};

function pad(value: number) {
  return String(value).padStart(2, "0");
}

function buildLocalDateKey(value: string): string | null {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return null;
  }

  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

export function formatDailyLabel(dateKey: string) {
  const parsed = new Date(`${dateKey}T00:00:00`);
  if (Number.isNaN(parsed.getTime())) {
    return dateKey;
  }

  return parsed.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export function buildDailyPnlRows<T extends ClosedTradeLike>(trades: T[], initialEquity: number): DailyPnlRow[] {
  const grouped = new Map<string, { pnl: number; trades: number; wins: number; losses: number }>();

  for (const trade of trades) {
    const dateKey = buildLocalDateKey(trade.exitTime);
    if (!dateKey) {
      continue;
    }

    const current = grouped.get(dateKey) ?? { pnl: 0, trades: 0, wins: 0, losses: 0 };
    current.pnl += trade.netPnl;
    current.trades += 1;
    if (trade.netPnl >= 0) current.wins += 1;
    else current.losses += 1;
    grouped.set(dateKey, current);
  }

  let runningEquity = initialEquity;
  const chronological = [...grouped.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([dateKey, value]) => {
      const startEquity = runningEquity;
      const pnlPct = startEquity > 0 ? (value.pnl / startEquity) * 100 : 0;
      const endEquity = startEquity + value.pnl;
      runningEquity = endEquity;

      return {
        dateKey,
        label: formatDailyLabel(dateKey),
        trades: value.trades,
        wins: value.wins,
        losses: value.losses,
        pnl: value.pnl,
        pnlPct,
        startEquity,
        endEquity,
      };
    });

  return chronological.reverse();
}
