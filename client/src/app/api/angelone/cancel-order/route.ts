import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";

export async function DELETE(request: Request) {
  if (!isAngelConfigured()) {
    return NextResponse.json({
      ok: false,
      error: `Not configured — set these env vars: ${angelMissingEnv()}`,
    });
  }

  let body: { order_id?: string; variety?: string };
  try {
    body = await request.json() as { order_id?: string; variety?: string };
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON body" }, { status: 400 });
  }

  if (!body.order_id) {
    return NextResponse.json({ ok: false, error: "Missing order_id" }, { status: 400 });
  }

  try {
    const jwt = await getAngelJWT();

    const cancelPayload = {
      variety: body.variety ?? "NORMAL",
      orderid: body.order_id,
    };

    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/order/v1/cancelOrder`, {
      method: "DELETE",
      headers: commonHeaders(jwt),
      body: JSON.stringify(cancelPayload),
      next: { revalidate: 0 },
    });

    const payload = await res.json() as {
      status?: boolean;
      message?: string;
      errorcode?: string;
      data?: { orderid?: string };
    };

    if (!res.ok || !payload.status) {
      const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || `Angel One returned ${res.status}`;
      return NextResponse.json({ ok: false, error: msg });
    }

    return NextResponse.json({
      ok: true,
      order_id: payload.data?.orderid ?? body.order_id,
      message: payload.message ?? "Order cancelled successfully",
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, error: message });
  }
}
