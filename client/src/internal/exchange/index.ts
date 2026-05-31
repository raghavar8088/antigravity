import type { Order } from "@/internal/oms";

export interface ExchangeOrderRequest {
  clientOrderId: string;
  symbol: string;
  side: "BUY" | "SELL";
  type: "MARKET" | "LIMIT";
  quantity: number;
  price?: number;
}

export interface ExchangeOrderAck {
  exchangeOrderId: string;
  status: "SUBMITTED" | "PARTIAL" | "FILLED" | "REJECTED";
  filledQuantity: number;
  averageFillPrice?: number;
  reason?: string;
  receivedAt: number;
}

export interface ExchangePosition {
  symbol: string;
  side: "LONG" | "SHORT" | "FLAT";
  quantity: number;
  entryPrice: number;
  markPrice: number;
  unrealizedPnl: number;
}

export interface ExchangeBalance {
  asset: string;
  available: number;
  equity: number;
}

export interface ExchangeAdapter {
  readonly name: string;
  placeOrder(order: ExchangeOrderRequest): Promise<ExchangeOrderAck>;
  cancelOrder(exchangeOrderId: string): Promise<{ ok: boolean; reason?: string }>;
  getPosition(symbol: string): Promise<ExchangePosition>;
  getBalance(asset?: string): Promise<ExchangeBalance>;
}

export class PaperExchangeAdapter implements ExchangeAdapter {
  readonly name = "paper";

  constructor(private readonly markPriceProvider: (symbol: string) => number = () => 0) {}

  async placeOrder(order: ExchangeOrderRequest): Promise<ExchangeOrderAck> {
    const mark = this.markPriceProvider(order.symbol);
    const fillPrice = order.price ?? (mark > 0 ? mark : 1);
    return {
      exchangeOrderId: `paper_${order.clientOrderId}`,
      status: "FILLED",
      filledQuantity: order.quantity,
      averageFillPrice: fillPrice,
      receivedAt: Date.now(),
    };
  }

  async cancelOrder(exchangeOrderId: string): Promise<{ ok: boolean; reason?: string }> {
    void exchangeOrderId;
    return { ok: true };
  }

  async getPosition(symbol: string): Promise<ExchangePosition> {
    return {
      symbol,
      side: "FLAT",
      quantity: 0,
      entryPrice: 0,
      markPrice: this.markPriceProvider(symbol),
      unrealizedPnl: 0,
    };
  }

  async getBalance(asset = "USD"): Promise<ExchangeBalance> {
    return { asset, available: Number.POSITIVE_INFINITY, equity: Number.POSITIVE_INFINITY };
  }
}

abstract class UnsupportedLiveAdapter implements ExchangeAdapter {
  abstract readonly name: string;

  async placeOrder(order: ExchangeOrderRequest): Promise<ExchangeOrderAck> {
    void order;
    return {
      exchangeOrderId: "",
      status: "REJECTED",
      filledQuantity: 0,
      reason: `${this.name} live adapter is not configured`,
      receivedAt: Date.now(),
    };
  }

  async cancelOrder(exchangeOrderId: string): Promise<{ ok: boolean; reason?: string }> {
    void exchangeOrderId;
    return { ok: false, reason: `${this.name} live adapter is not configured` };
  }

  async getPosition(symbol: string): Promise<ExchangePosition> {
    return { symbol, side: "FLAT", quantity: 0, entryPrice: 0, markPrice: 0, unrealizedPnl: 0 };
  }

  async getBalance(asset = "USD"): Promise<ExchangeBalance> {
    return { asset, available: 0, equity: 0 };
  }
}

export class BinanceExchangeAdapter extends UnsupportedLiveAdapter {
  readonly name = "binance";
}

export class CoinbaseExchangeAdapter extends UnsupportedLiveAdapter {
  readonly name = "coinbase";
}

export class DeltaExchangeAdapter extends UnsupportedLiveAdapter {
  readonly name = "delta";
}

export function orderToExchangeRequest(order: Order): ExchangeOrderRequest {
  return {
    clientOrderId: order.clientOrderId,
    symbol: order.symbol,
    side: order.side,
    type: order.type,
    quantity: order.quantity,
    price: order.price,
  };
}
