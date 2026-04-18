import { NextRequest, NextResponse } from "next/server";
import { deltaPublicFetch, deltaPost, type DeltaKeyOverride } from "@/lib/deltaSign";

function pf(v: unknown): number {
  if (typeof v === "number") return v;
  if (typeof v === "string") return parseFloat(v) || 0;
  return 0;
}

async function findOptionProduct(
  strike: number,
  optionType: string,
  overrides?: DeltaKeyOverride,
): Promise<{ productId: number; symbol: string; debugInfo?: string } | null> {
  const contractType = optionType === "CALL" ? "call_options" : "put_options";
  // Products is a public endpoint — no auth headers needed (auth causes 403 on IP-whitelisted keys)
  const res = await deltaPublicFetch(
    `/v2/products?contract_types=${contractType}&page_size=300`,
    overrides,
  );
  if (!res.ok) {
    return { productId: 0, symbol: "", debugInfo: `products fetch failed: HTTP ${res.status} — ${JSON.stringify(res.data)}` };
  }
  type P = { id?: number; symbol?: string; strike_price?: string };
  const products = (res.data as { result?: P[] }).result ?? [];
  let bestId = 0, bestSymbol = "", bestDiff = 1e18;
  for (const p of products) {
    const diff = Math.abs(parseFloat(p.strike_price ?? "0") - strike);
    if (diff < bestDiff) { bestDiff = diff; bestId = p.id ?? 0; bestSymbol = p.symbol ?? ""; }
  }
  if (!bestId) {
    const strikes = products.slice(0, 5).map((p) => p.strike_price).join(", ");
    return { productId: 0, symbol: "", debugInfo: `no product found — ${products.length} products returned, sample strikes: [${strikes || "none"}]` };
  }
  return { productId: bestId, symbol: bestSymbol };
}

export async function POST(req: NextRequest) {
  try {
    // Allow UI-supplied keys via request headers (takes priority over env vars)
    const overrides: DeltaKeyOverride = {};
    const hKey = req.headers.get("x-delta-api-key");
    const hSecret = req.headers.get("x-delta-api-secret");
    const hTestnet = req.headers.get("x-delta-testnet");
    if (hKey) overrides.apiKey = hKey;
    if (hSecret) overrides.apiSecret = hSecret;
    if (hTestnet !== null) overrides.testnet = hTestnet === "true";

    const body = await req.json() as {
      action: "open" | "close";
      optionType?: string;
      strike?: number;
      premiumUsd?: number;
      productId?: number;
      contracts?: number;
    };

    if (body.action === "open") {
      const { optionType = "CALL", strike = 0, premiumUsd = 100 } = body;
      const product = await findOptionProduct(strike, optionType, overrides);
      if (!product || !product.productId) {
        return NextResponse.json({
          ok: false,
          error: `No ${optionType} option found near strike $${strike}`,
          debug: product?.debugInfo,
        });
      }
      const contracts = Math.max(1, Math.floor(premiumUsd / 100));
      const result = await deltaPost("/v2/orders", {
        product_id: product.productId,
        size: contracts,
        side: "sell",
        order_type: "market_order",
        reduce_only: false,
        order_source: "place_order",
        source: "desktop",
      }, overrides);
      if (!result.ok) {
        const errAny = result.data as { error?: unknown };
        const errMsg = typeof errAny?.error === "string"
          ? errAny.error
          : (errAny?.error as { message?: string; code?: string } | undefined)?.message
            ?? (errAny?.error as { code?: string } | undefined)?.code
            ?? `HTTP ${result.status}`;
        return NextResponse.json({ ok: false, error: errMsg, debug: JSON.stringify(result.data) });
      }
      const r = result.data as { result?: { id?: number; symbol?: string; state?: string; average_fill_price?: string } };
      return NextResponse.json({
        ok: true,
        orderId: String(r.result?.id ?? ""),
        symbol: r.result?.symbol ?? product.symbol,
        productId: product.productId,
        contracts,
        fillPrice: pf(r.result?.average_fill_price),
        state: r.result?.state ?? "open",
      });
    }

    if (body.action === "close") {
      const { productId = 0, contracts = 1 } = body;
      if (!productId) return NextResponse.json({ ok: false, error: "productId required" });
      const result = await deltaPost("/v2/orders", {
        product_id: productId,
        size: contracts,
        side: "buy",
        order_type: "market_order",
        reduce_only: true,
        cancel_orders_accepted: "true",
        source: "desktop",
      }, overrides);
      if (!result.ok) {
        const errAny2 = result.data as { error?: unknown };
        const errMsg2 = typeof errAny2?.error === "string"
          ? errAny2.error
          : (errAny2?.error as { message?: string; code?: string } | undefined)?.message
            ?? (errAny2?.error as { code?: string } | undefined)?.code
            ?? `HTTP ${result.status}`;
        return NextResponse.json({ ok: false, error: errMsg2, debug: JSON.stringify(result.data) });
      }
      const r = result.data as { result?: { id?: number; average_fill_price?: string; state?: string } };
      return NextResponse.json({
        ok: true,
        closeOrderId: String(r.result?.id ?? ""),
        closeFillPrice: pf(r.result?.average_fill_price),
        state: r.result?.state ?? "open",
      });
    }

    return NextResponse.json({ ok: false, error: "Unknown action" }, { status: 400 });

  } catch (err) {
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}
