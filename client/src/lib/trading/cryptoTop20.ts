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
  { id: "sui", symbol: "SUI", name: "Sui", pair: "SUIUSDT", sector: "Layer 1" },
  { id: "xlm", symbol: "XLM", name: "Stellar", pair: "XLMUSDT", sector: "Payments" },
  { id: "hbar", symbol: "HBAR", name: "Hedera", pair: "HBARUSDT", sector: "Enterprise" },
  { id: "fil", symbol: "FIL", name: "Filecoin", pair: "FILUSDT", sector: "Storage" },
  { id: "etc", symbol: "ETC", name: "Ethereum Classic", pair: "ETCUSDT", sector: "Layer 1" },
  { id: "arb", symbol: "ARB", name: "Arbitrum", pair: "ARBUSDT", sector: "Layer 2" },
  { id: "op", symbol: "OP", name: "Optimism", pair: "OPUSDT", sector: "Layer 2" },
  { id: "inj", symbol: "INJ", name: "Injective", pair: "INJUSDT", sector: "DeFi" },
  { id: "aave", symbol: "AAVE", name: "Aave", pair: "AAVEUSDT", sector: "DeFi" },
  { id: "algo", symbol: "ALGO", name: "Algorand", pair: "ALGOUSDT", sector: "Layer 1" },
  { id: "vet", symbol: "VET", name: "VeChain", pair: "VETUSDT", sector: "Enterprise" },
  { id: "render", symbol: "RENDER", name: "Render", pair: "RENDERUSDT", sector: "AI" },
  { id: "sei", symbol: "SEI", name: "Sei", pair: "SEIUSDT", sector: "Layer 1" },
  { id: "gala", symbol: "GALA", name: "Gala", pair: "GALAUSDT", sector: "Gaming" },
  { id: "sand", symbol: "SAND", name: "The Sandbox", pair: "SANDUSDT", sector: "Metaverse" },
  { id: "mana", symbol: "MANA", name: "Decentraland", pair: "MANAUSDT", sector: "Metaverse" },
  { id: "jasmy", symbol: "JASMY", name: "JasmyCoin", pair: "JASMYUSDT", sector: "Data" },
  { id: "flow", symbol: "FLOW", name: "Flow", pair: "FLOWUSDT", sector: "Layer 1" },
  { id: "eos", symbol: "EOS", name: "EOS", pair: "EOSUSDT", sector: "Layer 1" },
  { id: "theta", symbol: "THETA", name: "Theta Network", pair: "THETAUSDT", sector: "Media" },
];
