import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";

export type PlaceOrderRequest = {
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

export async function POST(request: Request) {
  if (!isAngelConfigured()) {
    return NextResponse.json({
      ok: false,
      error: `Not configured — set these env vars: ${angelMissingEnv()}`,
    });
  }

  let body: PlaceOrderRequest;
  try {
    body = await request.json() as PlaceOrderRequest;
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON body" }, { status: 400 });
  }

  if (!body.tradingsymbol || !body.symboltoken || !body.transactiontype || !body.quantity) {
    return NextResponse.json({ ok: false, error: "Missing required fields" }, { status: 400 });
  }

  try {
    const jwt = await getAngelJWT();

    const orderPayload = {
      variety: body.variety ?? "NORMAL",
      tradingsymbol: body.tradingsymbol,
      symboltoken: body.symboltoken,
      transactiontype: body.transactiontype,
      exchange: body.exchange ?? "NFO",
      ordertype: body.ordertype ?? "MARKET",
      producttype: body.producttype ?? "INTRADAY",
      duration: "DAY",
      price: body.price ?? "0",
      squareoff: "0",
      stoploss: "0",
      quantity: String(body.quantity),
      triggerprice: body.triggerprice ?? "0",
    };

    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/order/v1/placeOrder`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify(orderPayload),
      next: { revalidate: 0 },
    });

    const payload = await res.json() as {
      status?: boolean;
      message?: string;
      errorcode?: string;
      data?: { orderid?: string; uniqueorderid?: string; script?: string };
    };

    if (!res.ok || !payload.status) {
      const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || `Angel One returned ${res.status}`;
      return NextResponse.json({ ok: false, error: msg });
    }

    return NextResponse.json({
      ok: true,
      order_id: payload.data?.orderid ?? payload.data?.uniqueorderid ?? "",
      message: payload.message ?? "Order placed successfully",
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, error: message });
  }
}
