export type CryptoAsset = {
  id: string;
  symbol: string;
  name: string;
  pair: string;
  sector: string;
};

export const CRYPTO_TOP_20: CryptoAsset[] = [
  { id: "btc", symbol: "BTC", name: "Bitcoin", pair: "BTCUSDT", sector: "Store of Value" },
  { id: "eth", symbol: "ETH", name: "Ethereum", pair: "ETHUSDT", sector: "Layer 1" },
  { id: "bnb", symbol: "BNB", name: "BNB", pair: "BNBUSDT", sector: "Exchange" },
  { id: "sol", symbol: "SOL", name: "Solana", pair: "SOLUSDT", sector: "Layer 1" },
  { id: "xrp", symbol: "XRP", name: "XRP", pair: "XRPUSDT", sector: "Payments" },
  { id: "ada", symbol: "ADA", name: "Cardano", pair: "ADAUSDT", sector: "Layer 1" },
  { id: "doge", symbol: "DOGE", name: "Dogecoin", pair: "DOGEUSDT", sector: "Meme" },
  { id: "avax", symbol: "AVAX", name: "Avalanche", pair: "AVAXUSDT", sector: "Layer 1" },
  { id: "trx", symbol: "TRX", name: "TRON", pair: "TRXUSDT", sector: "Payments" },
  { id: "dot", symbol: "DOT", name: "Polkadot", pair: "DOTUSDT", sector: "Interoperability" },
  { id: "link", symbol: "LINK", name: "Chainlink", pair: "LINKUSDT", sector: "Oracle" },
  { id: "matic", symbol: "MATIC", name: "Polygon", pair: "MATICUSDT", sector: "Scaling" },
  { id: "ton", symbol: "TON", name: "Toncoin", pair: "TONUSDT", sector: "Layer 1" },
  { id: "shib", symbol: "SHIB", name: "Shiba Inu", pair: "SHIBUSDT", sector: "Meme" },
  { id: "near", symbol: "NEAR", name: "NEAR Protocol", pair: "NEARUSDT", sector: "Layer 1" },
  { id: "ltc", symbol: "LTC", name: "Litecoin", pair: "LTCUSDT", sector: "Payments" },
  { id: "uni", symbol: "UNI", name: "Uniswap", pair: "UNIUSDT", sector: "DeFi" },
  { id: "atom", symbol: "ATOM", name: "Cosmos", pair: "ATOMUSDT", sector: "Interoperability" },
  { id: "bch", symbol: "BCH", name: "Bitcoin Cash", pair: "BCHUSDT", sector: "Payments" },
  { id: "apt", symbol: "APT", name: "Aptos", pair: "APTUSDT", sector: "Layer 1" },
];
