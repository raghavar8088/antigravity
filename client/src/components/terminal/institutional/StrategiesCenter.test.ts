import { describe, expect, it } from "vitest";
import { sortScalerStrategies } from "./StrategiesCenter";

describe("sortScalerStrategies", () => {
  it("puts active strategies before demoted ones", () => {
    const rows = sortScalerStrategies([
      { name: "Demoted_One", total_trades: 30, wins: 5, win_rate: 0.16, active: false, last_pnl: 100 },
      { name: "Active_One", total_trades: 5, wins: 2, win_rate: 0.4, active: true, last_pnl: -10 },
    ]);

    expect(rows[0].name).toBe("Active_One");
    expect(rows[1].name).toBe("Demoted_One");
  });

  it("ranks active strategies by last PnL, then trade count, then name", () => {
    const rows = sortScalerStrategies([
      { name: "Charlie", total_trades: 2, wins: 1, win_rate: 0.5, active: true, last_pnl: 50 },
      { name: "Alpha", total_trades: 2, wins: 1, win_rate: 0.5, active: true, last_pnl: 50 },
      { name: "Bravo", total_trades: 10, wins: 6, win_rate: 0.6, active: true, last_pnl: 50 },
      { name: "Delta", total_trades: 1, wins: 0, win_rate: 0, active: true, last_pnl: 200 },
    ]);

    expect(rows.map((r) => r.name)).toEqual(["Delta", "Bravo", "Alpha", "Charlie"]);
  });

  it("does not mutate the input array", () => {
    const input = [
      { name: "IV_RV_Spread_Reversion", total_trades: 0, wins: 0, win_rate: 0, active: true, last_pnl: 0 },
    ];
    const rows = sortScalerStrategies(input);
    expect(rows).not.toBe(input);
  });
});
