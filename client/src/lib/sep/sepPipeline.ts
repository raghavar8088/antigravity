import fs from "fs";
import path from "path";

export type SepStrategyRow = {
  strategy_id: string;
  category: string;
  tier: string;
  status: string;
  total_trades: number;
  win_rate: number;
  net_pnl: number;
  profit_factor: number;
  expectancy: number;
  sharpe_ratio: number;
  sortino_ratio: number;
  max_drawdown: number;
  evidence_score: number;
};

const SEP_DIR = process.env.SEP_REPORTS_DIR?.trim() || path.join(process.cwd(), "..", "sep_reports");

export function readSepStrategyEvidence(): SepStrategyRow[] | null {
  try {
    const file = path.join(/* turbopackIgnore: true */ SEP_DIR, "strategy_evidence.json");
    if (!fs.existsSync(file)) return null;
    const raw = JSON.parse(fs.readFileSync(file, "utf8")) as Array<Record<string, unknown>>;
    if (!Array.isArray(raw)) return null;
    return raw.map((row) => ({
      strategy_id: String(row.StrategyID ?? row.strategy_id ?? ""),
      category: String(row.Category ?? row.category ?? ""),
      tier: String(row.Tier ?? row.tier ?? "F"),
      status: String(row.Status ?? row.status ?? "INSUFFICIENT_DATA"),
      total_trades: Number(row.TotalTrades ?? row.total_trades ?? 0),
      win_rate: Number(row.WinRate ?? row.win_rate ?? 0),
      net_pnl: Number(row.NetPnL ?? row.net_pnl ?? 0),
      profit_factor: Number(row.ProfitFactor ?? row.profit_factor ?? 0),
      expectancy: Number(row.Expectancy ?? row.expectancy ?? 0),
      sharpe_ratio: Number(row.SharpeRatio ?? row.sharpe_ratio ?? 0),
      sortino_ratio: Number(row.SortinoRatio ?? row.sortino_ratio ?? 0),
      max_drawdown: Number(row.MaxDrawdown ?? row.max_drawdown ?? 0),
      evidence_score: Number(row.EvidenceScore ?? row.evidence_score ?? 0),
    }));
  } catch {
    return null;
  }
}

export function sepReportsAvailable(): boolean {
  return fs.existsSync(path.join(/* turbopackIgnore: true */ SEP_DIR, "strategy_evidence.json"));
}
