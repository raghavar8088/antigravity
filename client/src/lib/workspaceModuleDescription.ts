export type WorkspaceModuleKey = "btcFutureTrading";

export function workspaceModuleDescription(module: string): string {
  if (module === "btcFutureTrading") {
    return "Curated 20-strategy BTC perpetual module (trend, breakout, smart-money, order-flow, MTF). Separate paper state.";
  }
  return "Select a workspace tab above.";
}
