/**
 * /api/delta/mirror — Places a real Delta Exchange order when called.
 * Used by the frontend to mirror BTC Option Selling paper signals to real orders.
 *
 * POST body:
 *   { action: "open", optionType: "CALL"|"PUT", strike: number, expiryMinutes: number, premiumUsd: number }
 *   { action: "close", orderId: string, productId: number, contracts: number }
 */

import { NextRequest, NextResponse } from "next/server";

const BASE = "https://api.india.delta.exchange";
const TESTNET = "https://testnet-api.india.delta.exchange";
function getBase() { return process.env.DELTA_TESTNET === "true" ? TESTNET : BASE; }

async function sign(method: string, path: string, body: string, ts: string, secret: string): Promise<string> {
  const enc = new TextEncoder();
  const key = await crypto.subtle.importKey("raw", enc.encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const sig = await crypto.subtle.sign("HMAC", key, enc.encode(method + ts + path + body));
  return Array.from(new Uint8Array(sig)).map((b) => b.toString(16).padStart(2, "0")).join("");
}

async function deltaPost(path: string, body: unknown): Promise<{ ok: boolean; data: unknown; status: number }> {
  const apiKey = process.env.DELTA_API_KEY ?? "";
  const apiSecret = process.env.DELTA_API_SECRET ?? "";
  if (!apiKey || !apiSecret) return { ok: false, data: { error: "keys not set" }, status: 500 };

  const bodyStr = JSON.stringify(body);
  const ts = String(Math.floor(Date.now() / 1000));
  const sig = await sign("POST", path, bodyStr, ts, apiSecret);

  const res = await fetch(getBase() + path, {
    method: "POST",
    headers: { "api-key": apiKey, "timestamp": ts, "signature": sig, "Content-Type": "application/json", "Accept": "application/json" },
    body: bodyStr,
    cache: "no-store",
  });
  let data: unknown;
  try { data = await res.json(); } catch { data = {}; }
  return { ok: res.ok, data, status: res.status };
}

async function deltaGet(path: string): Promise<{ ok: boolean; data: unknown }> {
  const apiKey = process.env.DELTA_API_KEY ?? "";
  const apiSecret = process.env.DELTA_API_SECRET ?? "";
  if (!apiKey || !apiSecret) return { ok: false, data: {} };
  const ts = String(Math.floor(Date.now() / 1000));
  const sig = await sign("GET", path, "", ts, apiSecret);
  const res = await fetch(getBase() + path, {
    headers: { "api-key": apiKey, "timestamp": ts, "signature": sig, "Accept": "application/json" },
    cache: "no-store",
  });
  let data: unknown;
  try { data = await res.json(); } catch { data = {}; }
  return { ok: res.ok, data };
}

// Find closest BTC option product on Delta Exchange
async function findOptionProduct(strike: number, optionType: string): Promise<{ productId: number; symbol: string } | null> {
  const contractType = optionType === "CALL" ? "call_options" : "put_options";
  const { ok, data } = await deltaGet(`/v2/products?contract_types=${contractType}&page_size=300`);
  if (!ok) return null;

  type ProductRaw = { id?: number; symbol?: string; strike_price?: string; contract_type?: string };
  const d = data as { result?: ProductRaw[] };
  let bestId = 0;
  let bestSymbol = "";
  let bestDiff = 1e18;

  for (const p of d.result ?? []) {
    const s = parseFloat(p.strike_price ?? "0");
    const diff = Math.abs(s - strike);
    if (diff < bestDiff) {
      bestDiff = diff;
      bestId = p.id ?? 0;
      bestSymbol = p.symbol ?? "";
    }
  }
  return bestId ? { productId: bestId, symbol: bestSymbol } : null;
}

export const runtime = "edge";

export async function POST(req: NextRequest) {
  try {
    const body = await req.json() as {
      action: "open" | "close";
      optionType?: string;
      strike?: number;
      expiryMinutes?: number;
      premiumUsd?: number;
      orderId?: string;
      productId?: number;
      contracts?: number;
    };

    if (body.action === "open") {
      const { optionType = "CALL", strike = 0, premiumUsd = 100 } = body;

      // Find matching option product
      const product = await findOptionProduct(strike, optionType);
      if (!product) {
        return NextResponse.json({ ok: false, error: `No ${optionType} option found near strike $${strike}` });
      }

      // Calculate contracts (min 1)
      const contracts = Math.max(1, Math.floor(premiumUsd / 100));

      // Place sell order (selling options = collecting premium)
      const result = await deltaPost("/v2/orders", {
        product_id: product.productId,
        size: contracts,
        side: "sell",
        order_type: "market_order",
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
        fillPrice: parseFloat(r.result?.average_fill_price ?? "0"),
        state: r.result?.state ?? "open",
      });
    }

    if (body.action === "close") {
      const { productId = 0, contracts = 1 } = body;
      if (!productId) return NextResponse.json({ ok: false, error: "productId required" });

      // Buy to close the short option position
      const result = await deltaPost("/v2/orders", {
        product_id: productId,
        size: contracts,
        side: "buy",
        order_type: "market_order",
      });

      if (!result.ok) {
        const err = result.data as { error?: { code?: string; message?: string } };
        return NextResponse.json({ ok: false, error: err?.error?.message ?? err?.error?.code ?? `HTTP ${result.status}` });
      }

      const r = result.data as { result?: { id?: number; average_fill_price?: string; state?: string } };
      return NextResponse.json({
        ok: true,
        closeOrderId: String(r.result?.id ?? ""),
        closeFillPrice: parseFloat(r.result?.average_fill_price ?? "0"),
        state: r.result?.state ?? "open",
      });
    }

    return NextResponse.json({ ok: false, error: "Unknown action" }, { status: 400 });

  } catch (err) {
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}
