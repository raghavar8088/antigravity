/** CSV columns for `GET /api/paper-trades/export`. */
export const PAPER_TRADES_CSV_HEADERS = [
  "closed_at",
  "symbol",
  "strategy_name",
  "side",
  "entry",
  "exit",
  "net_pnl",
  "fees",
  "funding",
  "exit_reason",
] as const;

export type PaperTradeCsvExportRow = {
  closed_at: string;
  symbol: string;
  strategy_name: string;
  side: string;
  entry_price: number;
  exit_price: number;
  net_pnl: number;
  fees: number;
  funding_costs: number;
  exit_reason: string;
};

export function escapeCsvField(value: string): string {
  if (!/[",\n\r]/.test(value)) return value;
  return `"${value.replace(/"/g, '""')}"`;
}

export function formatPaperTradeExportRow(row: PaperTradeCsvExportRow): string {
  const cells = [
    row.closed_at,
    row.symbol,
    row.strategy_name,
    row.side,
    String(row.entry_price),
    String(row.exit_price),
    String(row.net_pnl),
    String(row.fees),
    String(row.funding_costs),
    row.exit_reason,
  ];
  return cells.map((c) => escapeCsvField(c)).join(",");
}

export function buildPaperTradesCsv(rows: readonly PaperTradeCsvExportRow[]): string {
  const lines = [PAPER_TRADES_CSV_HEADERS.join(",")];
  for (const row of rows) {
    lines.push(formatPaperTradeExportRow(row));
  }
  return `${lines.join("\n")}\n`;
}
