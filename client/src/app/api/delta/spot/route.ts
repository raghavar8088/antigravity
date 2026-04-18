import { NextRequest, NextResponse } from "next/server";
import { deltaPublicFetch, deltaPost, type DeltaKeyOverride } from "@/lib/deltaSign";

function pf(v: unknown): number {
  if (typeof v === "number") return v;
  if (typeof v === "string") return parseFloat(v) || 0;
  return 0;
}

async function findBTCSpotProduct(
  overrides?: DeltaKeyOverride,
): Promise<{ productId: number; symbol: string; contractSize: number; debugInfo?: string } | null> {
  const res = await deltaPublicFetch(
    "/v2/products?contract_types=spot&page_size=200",
    overrides,
  );
  if (!res.ok) {
    return { productId: 0, symbol: "", contractSize: 1, debugInfo: `products fetch failed: HTTP ${res.status} — ${JSON.stringify(res.data)}` };
  }
  type P = { id?: number; symbol?: string; contract_type?: string; underlying_asset?: { symbol?: string }; quoting_asset?: { symbol?: string }; contract_size?: string | number };
  const products = (res.data as { result?: P[] }).result ?? [];
  // Find BTC/USDT spot
  const btc = products.find(
    (p) =>
      (p.symbol ?? "").toUpperCase().includes("BTC") &&
      (p.symbol ?? "").toUpperCase().includes("USDT") &&
      p.contract_type === "spot",
  ) ?? products.find(
    (p) =>
      (p.underlying_asset?.symbol ?? "").toUpperCase() === "BTC" &&
      (p.quoting_asset?.symbol ?? "").toUpperCase() === "USDT",
  );
  if (!btc || !btc.id) {
    const sample = products.slice(0, 8).map((p) => p.symbol).join(", ");
    return { productId: 0, symbol: "", contractSize: 1, debugInfo: `BTC/USDT spot not found — ${products.length} spot products, sample: [${sample || "none"}]` };
  }
  return {
    productId: btc.id,
    symbol: btc.symbol ?? "",
    contractSize: pf(btc.contract_size) || 1,
  };
}

export async function GET() {
  // Probe: return available spot products for diagnostics
  const res = await deltaPublicFetch("/v2/products?contract_types=spot&page_size=200");
  type P = { id?: number; symbol?: string; contract_type?: string };
  const products = (res.data as { result?: P[] }).result ?? [];
  const btcProducts = products.filter((p) => (p.symbol ?? "").toUpperCase().includes("BTC"));
  return NextResponse.json({ ok: res.ok, status: res.status, btcProducts, total: products.length });
}

export async function POST(req: NextRequest) {
  try {
    const overrides: DeltaKeyOverride = {};
    const hKey = req.headers.get("x-delta-api-key");
    const hSecret = req.headers.get("x-delta-api-secret");
    const hTestnet = req.headers.get("x-delta-testnet");
    if (hKey) overrides.apiKey = hKey;
    if (hSecret) overrides.apiSecret = hSecret;
    if (hTestnet !== null) overrides.testnet = hTestnet === "true";

    const body = await req.json() as {
      action: "buy" | "sell" | "probe";
      orderType?: "market_order" | "limit_order";
      usdtAmount?: number;
      limitPrice?: number;
      productId?: number;
    };

    if (body.action === "probe") {
      const product = await findBTCSpotProduct(overrides);
      return NextResponse.json({ ok: true, product });
    }

    if (body.action === "buy" || body.action === "sell") {
      const { orderType = "market_order", usdtAmount = 0, limitPrice } = body;

      let productId = body.productId ?? 0;
      let symbol = "";
      let contractSize = 1;

      if (!productId) {
        const product = await findBTCSpotProduct(overrides);
        if (!product || !product.productId) {
          return NextResponse.json({ ok: false, error: "BTC/USDT spot product not found", debug: product?.debugInfo });
        }
        productId = product.productId;
        symbol = product.symbol;
        contractSize = product.contractSize;
      }

      if (usdtAmount <= 0) {
        return NextResponse.json({ ok: false, error: "usdtAmount must be > 0" });
      }

      // For spot: size is in BTC contracts. Each contract = contractSize BTC.
      // We need approximately usdtAmount / currentPrice BTC.
      // Without current price, we use size=1 as minimum and let user pass explicit size via usdtAmount.
      // Delta spot orders accept size in contracts; 1 contract typically = 0.001 BTC on Delta India.
      // We'll pass size as number of contracts: floor(usdtAmount / 100) minimum 1
      // (caller should pass usdtAmount as number of USDT, we compute rough contracts)
      const contracts = Math.max(1, Math.floor(usdtAmount / 100));

      const orderPayload: Record<string, unknown> = {
        product_id: productId,
        size: contracts,
        side: body.action === "buy" ? "buy" : "sell",
        order_type: orderType,
        reduce_only: false,
      };

      if (orderType === "limit_order") {
        if (!limitPrice || limitPrice <= 0) {
          return NextResponse.json({ ok: false, error: "limitPrice required for limit orders" });
        }
        orderPayload.limit_price = String(limitPrice);
      }

      const result = await deltaPost("/v2/orders", orderPayload, overrides);

      if (!result.ok) {
        const errAny = result.data as { error?: unknown };
        const errMsg =
          typeof errAny?.error === "string"
            ? errAny.error
            : (errAny?.error as { message?: string } | undefined)?.message
              ?? (errAny?.error as { code?: string } | undefined)?.code
              ?? `HTTP ${result.status}`;
        return NextResponse.json({ ok: false, error: errMsg, debug: JSON.stringify(result.data) });
      }

      const r = result.data as { result?: { id?: number; symbol?: string; state?: string; average_fill_price?: string; size?: number } };
      return NextResponse.json({
        ok: true,
        orderId: String(r.result?.id ?? ""),
        symbol: r.result?.symbol ?? symbol,
        productId,
        contracts,
        contractSize,
        fillPrice: pf(r.result?.average_fill_price),
        state: r.result?.state ?? "open",
        side: body.action,
        orderType,
      });
    }

    return NextResponse.json({ ok: false, error: "Unknown action" }, { status: 400 });
  } catch (err) {
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}
