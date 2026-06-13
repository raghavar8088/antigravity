"use client";
import { useState, useEffect, useCallback } from "react";
import { submitExecutionRequest, type ExecutionRequestPayload } from "@/lib/trading/executionRequest";

export type AngelOrder = {
  orderId: string;
  tradingSymbol: string;
  transactionType: "BUY" | "SELL";
  status: string;
  quantity: number;
  price: number;
  averagePrice: number;
  placedAt: string;
  orderType?: string;
  productType?: string;
  exchange?: string;
};


export type PlaceOrderParams = {
  tradingsymbol: string;
  symboltoken: string;
  exchange: string;
  transactiontype: "BUY" | "SELL";
  ordertype: "MARKET" | "LIMIT" | "STOPLOSS_MARKET" | "STOPLOSS_LIMIT";
  quantity: number;
  producttype: "INTRADAY" | "CARRYFORWARD" | "DELIVERY" | "MARGIN" | "BO";
  price: string;
  triggerprice: string;
  variety: "NORMAL" | "STOPLOSS" | "AMO";
};

type OrdersResponse = {
  ok: boolean;
  orders?: Array<{
    order_id: string;
    tradingsymbol: string;
    transactiontype: string;
    status: string;
    quantity: number;
    price: number;
    average_price: number;
    placed_at: string;
    ordertype?: string;
    producttype?: string;
    exchange?: string;
  }>;
  error?: string;
};

type FundsResponse = {
  ok: boolean;
  available_cash?: number;
  used_margin?: number;
  total?: number;
  error?: string;
};

type PlaceOrderResponse = {
  ok: boolean;
  order_id?: string;
  message?: string;
  error?: string;
};

type CancelOrderResponse = {
  ok: boolean;
  order_id?: string;
  message?: string;
  error?: string;
};

export default function useAngelOneOrders() {
  const [orders, setOrders] = useState<AngelOrder[]>([]);
  const [funds, setFunds] = useState<{ availableCash: number; usedMargin: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [ordersRes, fundsRes] = await Promise.all([
        fetch("/api/angelone/orders"),
        fetch("/api/angelone/funds"),
      ]);

      if (ordersRes.ok) {
        const data = await ordersRes.json() as OrdersResponse;
        if (data.ok && Array.isArray(data.orders)) {
          setOrders(
            data.orders.map((o) => ({
              orderId: o.order_id,
              tradingSymbol: o.tradingsymbol,
              transactionType: (o.transactiontype === "BUY" ? "BUY" : "SELL") as "BUY" | "SELL",
              status: o.status,
              quantity: o.quantity,
              price: o.price,
              averagePrice: o.average_price,
              placedAt: o.placed_at,
              orderType: o.ordertype,
              productType: o.producttype,
              exchange: o.exchange,
            }))
          );
          setError("");
        } else {
          setError(data.error ?? "Failed to load orders");
        }
      }

      if (fundsRes.ok) {
        const data = await fundsRes.json() as FundsResponse;
        if (data.ok) {
          setFunds({
            availableCash: data.available_cash ?? 0,
            usedMargin: data.used_margin ?? 0,
          });
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  /** @deprecated Direct broker placement disabled — submits an institutional execution request */
  const placeOrder = async (params: PlaceOrderParams): Promise<PlaceOrderResponse> => {
    const payload: ExecutionRequestPayload = {
      venue: "angelone",
      symbol: params.tradingsymbol,
      side: params.transactiontype,
      size: params.quantity,
      strategyName: "ANGELONE_MANUAL",
      reason: "ui_request",
    };
    const result = await submitExecutionRequest(payload);
    return {
      ok: result.ok,
      order_id: result.clientOrderId,
      message: result.message,
      error: result.ok ? undefined : result.message,
    };
  };

  const cancelOrder = async (_orderId: string, _variety = "NORMAL"): Promise<CancelOrderResponse> => {
    return {
      ok: false,
      error: "Direct order cancellation from UI is disabled — use backend OMS cancel when available",
    };
  };

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 15000);
    return () => clearInterval(id);
  }, [refresh]);

  return { orders, funds, loading, error, refresh, placeOrder, cancelOrder };
}
