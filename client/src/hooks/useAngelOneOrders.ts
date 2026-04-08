"use client";
import { useState, useEffect, useCallback } from "react";

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

  const placeOrder = async (params: PlaceOrderParams): Promise<PlaceOrderResponse> => {
    try {
      const res = await fetch("/api/angelone/order", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(params),
      });
      const data = await res.json() as PlaceOrderResponse;
      if (data.ok) {
        // Refresh order list after successful placement
        setTimeout(refresh, 1000);
      }
      return data;
    } catch (err) {
      const message = err instanceof Error ? err.message : "unknown error";
      return { ok: false, error: message };
    }
  };

  const cancelOrder = async (orderId: string, variety = "NORMAL"): Promise<CancelOrderResponse> => {
    try {
      const res = await fetch("/api/angelone/cancel-order", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ order_id: orderId, variety }),
      });
      const data = await res.json() as CancelOrderResponse;
      if (data.ok) {
        setTimeout(refresh, 1000);
      }
      return data;
    } catch (err) {
      const message = err instanceof Error ? err.message : "unknown error";
      return { ok: false, error: message };
    }
  };

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 15000);
    return () => clearInterval(id);
  }, [refresh]);

  return { orders, funds, loading, error, refresh, placeOrder, cancelOrder };
}
