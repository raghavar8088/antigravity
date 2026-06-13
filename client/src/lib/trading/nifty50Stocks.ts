export type Nifty50Stock = {
  symbol: string;
  name: string;
  token: string; // Angel One NSE exchange token
  lotSize: number; // NSE F&O lot size (options)
  sector: string;
};

/**
 * All 50 NIFTY 50 constituent stocks with Angel One SmartAPI exchange tokens.
 * Lot sizes are standard NSE F&O lot sizes (may change each expiry series revision).
 */
export const NIFTY50_STOCKS: Nifty50Stock[] = [
  // ── Banking & Finance ──────────────────────────────────────────────────────
  { symbol: "HDFCBANK",    name: "HDFC Bank",                    token: "1333",  lotSize: 550,  sector: "Banking" },
  { symbol: "ICICIBANK",   name: "ICICI Bank",                   token: "4963",  lotSize: 1375, sector: "Banking" },
  { symbol: "KOTAKBANK",   name: "Kotak Mahindra Bank",          token: "1922",  lotSize: 400,  sector: "Banking" },
  { symbol: "SBIN",        name: "State Bank of India",          token: "3045",  lotSize: 1500, sector: "Banking" },
  { symbol: "AXISBANK",    name: "Axis Bank",                    token: "5900",  lotSize: 1200, sector: "Banking" },
  { symbol: "INDUSINDBK",  name: "IndusInd Bank",                token: "5258",  lotSize: 300,  sector: "Banking" },
  { symbol: "BAJFINANCE",  name: "Bajaj Finance",                token: "317",   lotSize: 125,  sector: "Finance" },
  { symbol: "BAJAJFINSV",  name: "Bajaj Finserv",                token: "16669", lotSize: 200,  sector: "Finance" },
  { symbol: "SHRIRAMFIN",  name: "Shriram Finance",              token: "4306",  lotSize: 400,  sector: "Finance" },
  { symbol: "SBILIFE",     name: "SBI Life Insurance",           token: "21808", lotSize: 750,  sector: "Insurance" },
  { symbol: "HDFCLIFE",    name: "HDFC Life Insurance",          token: "467",   lotSize: 1100, sector: "Insurance" },

  // ── Information Technology ─────────────────────────────────────────────────
  { symbol: "TCS",         name: "Tata Consultancy Services",    token: "11536", lotSize: 150,  sector: "IT" },
  { symbol: "INFY",        name: "Infosys",                      token: "1594",  lotSize: 300,  sector: "IT" },
  { symbol: "HCLTECH",     name: "HCL Technologies",             token: "7229",  lotSize: 350,  sector: "IT" },
  { symbol: "WIPRO",       name: "Wipro",                        token: "3787",  lotSize: 1500, sector: "IT" },
  { symbol: "TECHM",       name: "Tech Mahindra",                token: "13538", lotSize: 600,  sector: "IT" },

  // ── Energy & Resources ─────────────────────────────────────────────────────
  { symbol: "RELIANCE",    name: "Reliance Industries",          token: "2885",  lotSize: 250,  sector: "Energy" },
  { symbol: "ONGC",        name: "Oil & Natural Gas Corp",       token: "2475",  lotSize: 1925, sector: "Energy" },
  { symbol: "BPCL",        name: "Bharat Petroleum Corp",        token: "526",   lotSize: 1800, sector: "Energy" },
  { symbol: "NTPC",        name: "NTPC",                         token: "11630", lotSize: 3000, sector: "Power" },
  { symbol: "POWERGRID",   name: "Power Grid Corp",              token: "14977", lotSize: 2700, sector: "Power" },
  { symbol: "COALINDIA",   name: "Coal India",                   token: "20374", lotSize: 2700, sector: "Mining" },
  { symbol: "ADANIENT",    name: "Adani Enterprises",            token: "25",    lotSize: 500,  sector: "Conglomerate" },
  { symbol: "ADANIPORTS",  name: "Adani Ports & SEZ",            token: "15083", lotSize: 1250, sector: "Infrastructure" },

  // ── Consumer & FMCG ───────────────────────────────────────────────────────
  { symbol: "HINDUNILVR",  name: "Hindustan Unilever",           token: "1394",  lotSize: 300,  sector: "FMCG" },
  { symbol: "ITC",         name: "ITC",                          token: "1660",  lotSize: 3200, sector: "FMCG" },
  { symbol: "NESTLEIND",   name: "Nestle India",                 token: "17963", lotSize: 40,   sector: "FMCG" },
  { symbol: "BRITANNIA",   name: "Britannia Industries",         token: "547",   lotSize: 200,  sector: "FMCG" },
  { symbol: "TATACONSUM",  name: "Tata Consumer Products",       token: "3432",  lotSize: 900,  sector: "FMCG" },
  { symbol: "TITAN",       name: "Titan Company",                token: "3506",  lotSize: 375,  sector: "Consumer" },
  { symbol: "TRENT",       name: "Trent",                        token: "3721",  lotSize: 375,  sector: "Retail" },

  // ── Automobiles ────────────────────────────────────────────────────────────
  { symbol: "MARUTI",      name: "Maruti Suzuki",                token: "10999", lotSize: 100,  sector: "Auto" },
  { symbol: "TATAMOTORS",  name: "Tata Motors",                  token: "3456",  lotSize: 1400, sector: "Auto" },
  { symbol: "M&M",         name: "Mahindra & Mahindra",          token: "2031",  lotSize: 700,  sector: "Auto" },
  { symbol: "HEROMOTOCO",  name: "Hero MotoCorp",                token: "1348",  lotSize: 300,  sector: "Auto" },
  { symbol: "EICHERMOT",   name: "Eicher Motors",                token: "910",   lotSize: 175,  sector: "Auto" },
  { symbol: "BAJAJ-AUTO",  name: "Bajaj Auto",                   token: "16675", lotSize: 75,   sector: "Auto" },

  // ── Metals & Materials ─────────────────────────────────────────────────────
  { symbol: "TATASTEEL",   name: "Tata Steel",                   token: "3499",  lotSize: 5500, sector: "Steel" },
  { symbol: "JSWSTEEL",    name: "JSW Steel",                    token: "11723", lotSize: 600,  sector: "Steel" },
  { symbol: "HINDALCO",    name: "Hindalco Industries",          token: "1363",  lotSize: 2150, sector: "Metals" },
  { symbol: "GRASIM",      name: "Grasim Industries",            token: "1232",  lotSize: 475,  sector: "Cement" },
  { symbol: "ULTRACEMCO",  name: "UltraTech Cement",             token: "11532", lotSize: 100,  sector: "Cement" },
  { symbol: "ASIANPAINT",  name: "Asian Paints",                 token: "236",   lotSize: 200,  sector: "Paints" },

  // ── Pharma & Healthcare ────────────────────────────────────────────────────
  { symbol: "SUNPHARMA",   name: "Sun Pharmaceutical",           token: "3351",  lotSize: 350,  sector: "Pharma" },
  { symbol: "CIPLA",       name: "Cipla",                        token: "694",   lotSize: 650,  sector: "Pharma" },
  { symbol: "DRREDDY",     name: "Dr Reddy's Laboratories",      token: "881",   lotSize: 125,  sector: "Pharma" },
  { symbol: "DIVISLAB",    name: "Divi's Laboratories",          token: "10940", lotSize: 200,  sector: "Pharma" },
  { symbol: "APOLLOHOSP",  name: "Apollo Hospitals",             token: "157",   lotSize: 125,  sector: "Healthcare" },

  // ── Infrastructure & Telecom ───────────────────────────────────────────────
  { symbol: "LT",          name: "Larsen & Toubro",              token: "11483", lotSize: 25,   sector: "Infrastructure" },
  { symbol: "BHARTIARTL",  name: "Bharti Airtel",                token: "10604", lotSize: 1851, sector: "Telecom" },
];

/** Quick lookup by symbol */
export const STOCK_BY_SYMBOL: Record<string, Nifty50Stock> = Object.fromEntries(
  NIFTY50_STOCKS.map((s) => [s.symbol, s]),
);

/** Quick lookup by token */
export const STOCK_BY_TOKEN: Record<string, Nifty50Stock> = Object.fromEntries(
  NIFTY50_STOCKS.map((s) => [s.token, s]),
);

/** All Angel One NSE tokens as strings (for batch API) */
export const ALL_TOKENS = NIFTY50_STOCKS.map((s) => s.token);
