/**
 * Perpetual futures watchlist + canonical symbol list for multi-asset paper trading.
 * Keep TRADING_SYMBOLS in sync with Delta India contract symbols.
 */

export type FuturesWatchItem = {
  symbol: string;
  name: string;
  leverage: string;
  lastPrice: string;
  change24h: string;
  volume24h: string;
};

export const FUTURES_WATCHLIST: FuturesWatchItem[] = [
  { symbol: "BTCUSD", name: "Bitcoin Perpetual", leverage: "200x", lastPrice: "$80845", change24h: "0.68%", volume24h: "$374.18M" },
  { symbol: "ETHUSD", name: "Ethereum Perpetual", leverage: "200x", lastPrice: "$2329.5", change24h: "0.94%", volume24h: "$267.65M" },
  { symbol: "SOLUSD", name: "Solana Perpetual", leverage: "100x", lastPrice: "$93.926", change24h: "0.99%", volume24h: "$92.70M" },
  { symbol: "BNBUSD", name: "BNB Perpetual", leverage: "125x", lastPrice: "$592.40", change24h: "1.12%", volume24h: "$128.44M" },
  { symbol: "ADAUSD", name: "Cardano Perpetual", leverage: "100x", lastPrice: "$0.4621", change24h: "2.06%", volume24h: "$64.81M" },
  { symbol: "DOGEUSD", name: "Dogecoin Perpetual", leverage: "100x", lastPrice: "$0.11102", change24h: "1.34%", volume24h: "$3.17M" },
  { symbol: "UNIUSD", name: "Uniswap Perpetual", leverage: "100x", lastPrice: "$4.04", change24h: "8.98%", volume24h: "$9.28M" },
  { symbol: "RAVEUSD", name: "RaveDAO Perpetual", leverage: "20x", lastPrice: "$0.7511", change24h: "-15.03%", volume24h: "$3.15M" },
  { symbol: "PAXGUSD", name: "PAX Gold Perpetual", leverage: "5x", lastPrice: "$4717.8", change24h: "0.19%", volume24h: "$6.64M" },
  { symbol: "XAUTUSD", name: "Tether Gold Perpetual", leverage: "100x", lastPrice: "$4713.91", change24h: "0.18%", volume24h: "$3.72M" },
  { symbol: "BIOUSD", name: "Bio Protocol Perpetual", leverage: "20x", lastPrice: "$0.05304", change24h: "-5.84%", volume24h: "$2.45M" },
  { symbol: "HUSD", name: "Humanity Protocol Perpetual", leverage: "20x", lastPrice: "$0.20211", change24h: "-1.80%", volume24h: "$1.07M" },
  { symbol: "TSTUSD", name: "Test Token Perpetual", leverage: "20x", lastPrice: "$0.02177", change24h: "-10.26%", volume24h: "$1.22M" },
  { symbol: "RIVERUSD", name: "River Perpetual", leverage: "20x", lastPrice: "$6.474", change24h: "-1.05%", volume24h: "$1.62M" },
  { symbol: "JASMYUSD", name: "JasmyCoin Perpetual", leverage: "20x", lastPrice: "$0.007239", change24h: "2.03%", volume24h: "$1.81M" },
  { symbol: "AIOTUSD", name: "OKZOO Perpetual", leverage: "20x", lastPrice: "$0.08904", change24h: "3.29%", volume24h: "$995.53K" },
  { symbol: "MUSD", name: "MemeCore Perpetual", leverage: "20x", lastPrice: "$3.3034", change24h: "-4.05%", volume24h: "$1.55M" },
  { symbol: "LABUSD", name: "LAB Perpetual", leverage: "20x", lastPrice: "$4.789", change24h: "16.01%", volume24h: "$24.26M" },
  { symbol: "SIRENUSD", name: "Siren Perpetual", leverage: "20x", lastPrice: "$1.17798", change24h: "-7.76%", volume24h: "$13.22M" },
  { symbol: "LAYERUSD", name: "Solayer Perpetual", leverage: "20x", lastPrice: "$0.1297", change24h: "37.39%", volume24h: "$12.67M" },
  { symbol: "SAHARAUSD", name: "Sahara AI Perpetual", leverage: "20x", lastPrice: "$0.03567", change24h: "-8.35%", volume24h: "$9.10M" },
  { symbol: "ZECUSD", name: "Zcash Perpetual", leverage: "20x", lastPrice: "$599.94", change24h: "2.16%", volume24h: "$8.41M" },
  { symbol: "AINUSD", name: "Ainfinity Ground Perpetual", leverage: "20x", lastPrice: "$0.0956", change24h: "-4.12%", volume24h: "$8.04M" },
  { symbol: "VVVUSD", name: "Venice Token Perpetual", leverage: "20x", lastPrice: "$15.149", change24h: "-1.41%", volume24h: "$7.31M" },
  { symbol: "SKYAIUSD", name: "SkyAI Perpetual", leverage: "20x", lastPrice: "$0.54383", change24h: "-5.67%", volume24h: "$7.30M" },
  { symbol: "XRPUSD", name: "Ripple Perpetual", leverage: "100x", lastPrice: "$1.4345", change24h: "1.29%", volume24h: "$6.10M" },
];

/** Symbols used for multi-asset strategy engine (same set as watchlist). */
export const TRADING_SYMBOLS: readonly string[] = FUTURES_WATCHLIST.map((r) => r.symbol);

export const PRIMARY_QUOTE_SYMBOL = "BTCUSD";
