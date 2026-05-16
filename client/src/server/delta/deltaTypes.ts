/** Delta Exchange v2 REST — subset used by testnet execution adapter (P3-A). */

export type DeltaOrderSide = "buy" | "sell";

export type DeltaOrderType = "market_order" | "limit_order";

export type DeltaWalletBalance = {
  asset: string;
  balance: number;
  availableBalance: number;
  blockedBalance: number;
  unrealisedPnl: number;
};

export type DeltaMarginedPosition = {
  symbol: string;
  productId: number;
  size: number;
  entryPrice: number;
  markPrice: number;
  unrealisedPnl: number;
  realisedPnl: number;
  margin: number;
  side: "LONG" | "SHORT";
};

export type DeltaOpenOrder = {
  orderId: string;
  symbol: string;
  productId: number;
  side: DeltaOrderSide;
  size: number;
  limitPrice: number | null;
  state: string;
  createdAt: string;
};

export type PlaceOrderParams = {
  productId: number;
  size: number;
  side: DeltaOrderSide;
  orderType: DeltaOrderType;
  /** Required for `limit_order`. */
  limitPrice?: number;
  reduceOnly?: boolean;
};

export type PlaceOrderResult = {
  orderId: string;
  symbol: string;
  state: string;
  averageFillPrice: number | null;
};

export type DeltaApiSuccess<T> = {
  ok: true;
  data: T;
};

export type DeltaApiFailure = {
  ok: false;
  status: number;
  error: string;
  raw?: unknown;
};

export type DeltaApiResult<T> = DeltaApiSuccess<T> | DeltaApiFailure;

export type DeltaBalanceSnippet = {
  asset: string;
  availableBalance: number;
}[];
