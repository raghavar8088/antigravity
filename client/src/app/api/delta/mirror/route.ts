import { NextRequest, NextResponse } from "next/server";
import { deltaFetch, deltaPost } from "@/lib/deltaSign";

function pf(v: unknown): number {
  if (typeof v === "number") return v;
  if (typeof v === "string") return parseFloat(v) || 0;
  return 0;
}

async function findOptionProduct(strike: number, optionType: string): Promise<{ productId: number; symbol: string } | null> {
  const contractType = optionType === "CALL" ? "call_options" : "put_options";
  const { ok, data } = await deltaFetch(`/v2/products?contract_types=${contractType}&page_size=300`);
  if (!ok) return null;
  type P = { id?: number; symbol?: string; strike_price?: string };
  let bestId = 0, bestSymbol = "", bestDiff = 1e18;
  for (const p of (data as { result?: P[] }).result ?? []) {
    const diff = Math.abs(parseFloat(p.strike_price ?? "0") - strike);
    if (diff < bestDiff) { bestDiff = diff; bestId = p.id ?? 0; bestSymbol = p.symbol ?? ""; }
  }
  return bestId ? { productId: bestId, symbol: bestSymbol } : null;
}

export async function POST(req: NextRequest) {
  try {
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
      const product = await findOptionProduct(strike, optionType);
      if (!product) {
        return NextResponse.json({ ok: false, error: `No ${optionType} option found near strike $${strike}` });
      }
      const contracts = Math.max(1, Math.floor(premiumUsd / 100));
      const result = await deltaPost("/v2/orders", {
        product_id: product.productId,
        size: contracts,
        side: "sell",
        order_type: "market_order",
        reduce_only: false,
        order_source: "place_order",
      });
      if (!result.ok) {
        const err = result.data as { error?: { code?: string; message?: string } };
        return NextResponse.json({ ok: false, error: err?.error?.message ?? err?.error?.code ?? `HTTP ${result.status}` });
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
      });
      if (!result.ok) {
        const err = result.data as { error?: { code?: string; message?: string } };
        return NextResponse.json({ ok: false, error: err?.error?.message ?? err?.error?.code ?? `HTTP ${result.status}` });
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
