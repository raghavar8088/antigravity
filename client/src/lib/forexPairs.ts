export type ForexPair = {
  id: string;
  symbol: string;      // display e.g. "EUR/USD"
  yahooSymbol: string; // e.g. "EURUSD=X"
  base: string;
  quote: string;
  category: string;
  pipSize: number;     // smallest price movement (0.0001 for most, 0.01 for JPY)
};

export const FOREX_PAIRS: ForexPair[] = [
  { id: "eurusd",  symbol: "EUR/USD", yahooSymbol: "EURUSD=X",  base: "EUR", quote: "USD", category: "Major",    pipSize: 0.0001 },
  { id: "gbpusd",  symbol: "GBP/USD", yahooSymbol: "GBPUSD=X",  base: "GBP", quote: "USD", category: "Major",    pipSize: 0.0001 },
  { id: "usdjpy",  symbol: "USD/JPY", yahooSymbol: "USDJPY=X",  base: "USD", quote: "JPY", category: "Major",    pipSize: 0.01   },
  { id: "usdchf",  symbol: "USD/CHF", yahooSymbol: "USDCHF=X",  base: "USD", quote: "CHF", category: "Major",    pipSize: 0.0001 },
  { id: "audusd",  symbol: "AUD/USD", yahooSymbol: "AUDUSD=X",  base: "AUD", quote: "USD", category: "Major",    pipSize: 0.0001 },
  { id: "usdcad",  symbol: "USD/CAD", yahooSymbol: "USDCAD=X",  base: "USD", quote: "CAD", category: "Major",    pipSize: 0.0001 },
  { id: "nzdusd",  symbol: "NZD/USD", yahooSymbol: "NZDUSD=X",  base: "NZD", quote: "USD", category: "Major",    pipSize: 0.0001 },
  { id: "eurgbp",  symbol: "EUR/GBP", yahooSymbol: "EURGBP=X",  base: "EUR", quote: "GBP", category: "Cross",    pipSize: 0.0001 },
  { id: "eurjpy",  symbol: "EUR/JPY", yahooSymbol: "EURJPY=X",  base: "EUR", quote: "JPY", category: "Cross",    pipSize: 0.01   },
  { id: "gbpjpy",  symbol: "GBP/JPY", yahooSymbol: "GBPJPY=X",  base: "GBP", quote: "JPY", category: "Cross",    pipSize: 0.01   },
  { id: "usdinr",  symbol: "USD/INR", yahooSymbol: "USDINR=X",  base: "USD", quote: "INR", category: "Emerging", pipSize: 0.01   },
  { id: "usdsgd",  symbol: "USD/SGD", yahooSymbol: "USDSGD=X",  base: "USD", quote: "SGD", category: "Emerging", pipSize: 0.0001 },
];
