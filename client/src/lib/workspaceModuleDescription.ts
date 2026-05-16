export type WorkspaceModuleKey =
  | "dashboard"
  | "engine"
  | "history"
  | "options"
  | "options-selling"
  | "chain"
  | "nifty"
  | "niftySelling"
  | "niftyBees"
  | "btcFuturesScalper"
  | "btcFutureTrading";

export function workspaceModuleDescription(module: string): string {
  switch (module as WorkspaceModuleKey) {
    case "options":
      return "50 autonomous BTC option scalping strategies. Separate $1M paper account, 1% per trade.";
    case "niftySelling":
      return "NIFTY 50 short-option workspace focused on premium decay.";
    case "nifty":
      return "Nifty 50 option scalper on live NSE data. Separate ₹1M paper account, 1% per trade.";
    case "niftyBees":
      return "Nifty BEES ETF on Angel/Yahoo live NSE prices. 30 strategies, ₹10,000 paper.";
    case "options-selling":
      return "BTC short-premium workspace with theta-style decay and separate paper capital.";
    case "chain":
      return "Live BTC option chain with full Greeks and IV smile. Read-only.";
    case "btcFuturesScalper":
      return "Multi-asset perpetual futures with 25× leverage, liquidation tracking, and funding awareness. $1,000 paper wallet.";
    case "btcFutureTrading":
      return "Curated 20-strategy BTC perpetual module (trend, breakout, smart-money, order-flow, MTF). Separate paper state.";
    default:
      return "Select a workspace tab above.";
  }
}
