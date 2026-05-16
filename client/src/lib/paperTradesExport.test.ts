import { describe, expect, it } from "vitest";
import {
  PAPER_TRADES_CSV_HEADERS,
  buildPaperTradesCsv,
  formatPaperTradeExportRow,
} from "./paperTradesExport";

describe("paperTradesExport", () => {
  it("uses expected CSV header columns", () => {
    expect(PAPER_TRADES_CSV_HEADERS.join(",")).toBe(
      "closed_at,symbol,strategy_name,side,entry,exit,net_pnl,fees,funding,exit_reason",
    );
  });

  it("formats one export row with quoted strategy name when needed", () => {
    const line = formatPaperTradeExportRow({
      closed_at: "2026-05-16T12:00:00.000Z",
      symbol: "BTCUSD",
      strategy_name: "BB_MeanRev, Long",
      side: "LONG",
      entry_price: 80_000,
      exit_price: 81_000,
      net_pnl: 5.25,
      fees: 1,
      funding_costs: 0.05,
      exit_reason: "TP",
    });
    expect(line).toBe(
      '2026-05-16T12:00:00.000Z,BTCUSD,"BB_MeanRev, Long",LONG,80000,81000,5.25,1,0.05,TP',
    );
  });

  it("buildPaperTradesCsv includes header and trailing newline", () => {
    const csv = buildPaperTradesCsv([
      {
        closed_at: "2026-05-16T12:00:00.000Z",
        symbol: "BTCUSD",
        strategy_name: "Test",
        side: "SHORT",
        entry_price: 1,
        exit_price: 2,
        net_pnl: -0.5,
        fees: 0.1,
        funding_costs: 0,
        exit_reason: "TIME",
      },
    ]);
    expect(csv.startsWith(`${PAPER_TRADES_CSV_HEADERS.join(",")}\n`)).toBe(true);
    expect(csv.endsWith("\n")).toBe(true);
    expect(csv.split("\n").length).toBe(3);
  });
});
