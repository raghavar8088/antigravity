export type WorkspaceModuleKey = "btcFutureTrading" | "mockTrading";

export function workspaceModuleDescription(module: string): string {
  if (module === "btcFutureTrading") {
    return "Curated 20-strategy BTC perpetual module (trend, breakout, smart-money, order-flow, MTF). Separate paper state.";
  }
  if (module === "mockTrading") {
    return "Analysis twin of BTC FT — every raised signal becomes a mock trade with blockers recorded but not enforced. No broker orders.";
  }
  return "Select a workspace tab above.";
}
