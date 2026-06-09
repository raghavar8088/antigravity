import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";

export type AngelOrderRaw = {
  orderid?: string;
  uniqueorderid?: string;
  tradingsymbol?: string;
  transactiontype?: string;
  status?: string;
  quantity?: string | number;
  price?: string | number;
  averageprice?: string | number;
  updatetime?: string;
  exchorderupdatetime?: string;
  ordertype?: string;
  producttype?: string;
  exchange?: string;
};

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  if (!isAngelConfigured()) {
    return NextResponse.json({
      ok: false,
      orders: [],
      error: `Not configured — set these env vars: ${angelMissingEnv()}`,
    });
  }

  try {
    const jwt = await getAngelJWT();

    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/order/v1/book`, {
      method: "GET",
      headers: commonHeaders(jwt),
      next: { revalidate: 0 },
    });

    if (!res.ok) {
      return NextResponse.json({ ok: false, orders: [], error: `Angel One returned ${res.status}` });
    }

    const payload = await res.json() as {
      status?: boolean;
      message?: string;
      errorcode?: string;
      data?: AngelOrderRaw[] | null;
    };

    if (!payload.status) {
      const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || "Order book fetch failed";
      return NextResponse.json({ ok: false, orders: [], error: msg });
    }

    const rawOrders = Array.isArray(payload.data) ? payload.data : [];

    const orders = rawOrders.map((o) => ({
      order_id: o.orderid ?? o.uniqueorderid ?? "",
      tradingsymbol: o.tradingsymbol ?? "",
      transactiontype: o.transactiontype ?? "",
      status: o.status ?? "",
      quantity: Number(o.quantity) || 0,
      price: Number(o.price) || 0,
      average_price: Number(o.averageprice) || 0,
      placed_at: o.updatetime ?? o.exchorderupdatetime ?? "",
      ordertype: o.ordertype ?? "",
      producttype: o.producttype ?? "",
      exchange: o.exchange ?? "",
    }));

    return NextResponse.json({ ok: true, orders });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, orders: [], error: message });
  }
}
