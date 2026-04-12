/**
 * /api/delta/account — Fetches live Delta Exchange account data
 * (wallet balances, positions, open orders) using DELTA_API_KEY / DELTA_API_SECRET
 * from Vercel environment variables.
 */

import { NextResponse } from "next/server";
import { createHmac } from "crypto";

const BASE = "https://api.india.delta.exchange";
const TESTNET = "https://testnet-api.india.delta.exchange";

function getBase() {
  return process.env.DELTA_TESTNET === "true" ? TESTNET : BASE;
}

function sign(method: string, path: string, body: string, ts: string, secret: string): string {
  const payload = method + ts + path + body;
  return createHmac("sha256", secret).update(payload).digest("hex");
}

async function deltaFetch(path: string, method = "GET", body = ""): Promise<{ ok: boolean; data: unknown; status: number }> {
  const key = process.env.DELTA_API_KEY ?? "";
  const secret = process.env.DELTA_API_SECRET ?? "";
  if (!key || !secret) return { ok: false, data: { error: "DELTA_API_KEY / DELTA_API_SECRET not set" }, status: 500 };

  const ts = String(Math.floor(Date.now() / 1000));
  const sig = sign(method, path, body, ts, secret);

  const res = await fetch(getBase() + path, {
    method,
    headers: {
      "api-key": key,
      "timestamp": ts,
      "signature": sig,
      "Content-Type": "application/json",
      "Accept": "application/json",
    },
    body: body || undefined,
    cache: "no-store",
  });

  const data: unknown = await res.json();
  return { ok: res.ok, data, status: res.status };
}

function parseFloat2(v: unknown): number {
  if (typeof v === "number") return v;
  if (typeof v === "string") return parseFloat(v) || 0;
  return 0;
}

export async function GET() {
  const key = process.env.DELTA_API_KEY;
  const secret = process.env.DELTA_API_SECRET;

  if (!key || !secret) {
    return NextResponse.json({
      configured: false,
      testnet: false,
      error: "DELTA_API_KEY and DELTA_API_SECRET not configured",
    });
  }

  // Get our outbound IP so we can show it if whitelisting fails
  let outboundIP = "unknown";
  try {
    const ipRes = await fetch("https://api.ipify.org?format=json", { cache: "no-store" });
    const ipData = await ipRes.json() as { ip?: string };
    outboundIP = ipData.ip ?? "unknown";
  } catch { /* ignore */ }

  // Fetch wallet, positions, open orders in parallel
  const [walletRes, posRes, ordersRes] = await Promise.allSettled([
    deltaFetch("/v2/wallet/balances"),
    deltaFetch("/v2/positions/margined"),
    deltaFetch("/v2/orders?state=open"),
  ]);

  // --- Wallets ---
  type WalletRaw = { asset_symbol?: string; balance?: unknown; available_balance?: unknown; blocked_margin?: unknown; unrealised_cashflow?: unknown };
  const wallets: { asset: string; balance: number; availableBalance: number; blockedBalance: number; unrealisedPnl: number }[] = [];
  if (walletRes.status === "fulfilled" && walletRes.value.ok) {
    const d = walletRes.value.data as { result?: WalletRaw[] };
    for (const w of d.result ?? []) {
      const bal = parseFloat2(w.balance);
      const avail = parseFloat2(w.available_balance);
      if (bal !== 0 || avail !== 0) {
        wallets.push({
          asset: w.asset_symbol ?? "",
          balance: bal,
          availableBalance: avail,
          blockedBalance: parseFloat2(w.blocked_margin),
          unrealisedPnl: parseFloat2(w.unrealised_cashflow),
        });
      }
    }
  }

  // --- Positions ---
  type PosRaw = { symbol?: string; product_id?: number; size?: unknown; entry_price?: unknown; mark_price?: unknown; unrealised_pnl?: unknown; realised_pnl?: unknown; margin?: unknown };
  const positions: { symbol: string; productId: number; size: number; entryPrice: number; markPrice: number; unrealisedPnl: number; realisedPnl: number; margin: number; side: string }[] = [];
  if (posRes.status === "fulfilled" && posRes.value.ok) {
    const d = posRes.value.data as { result?: PosRaw[] };
    for (const p of d.result ?? []) {
      const size = parseFloat2(p.size);
      if (size !== 0) {
        positions.push({
          symbol: p.symbol ?? "",
          productId: p.product_id ?? 0,
          size,
          entryPrice: parseFloat2(p.entry_price),
          markPrice: parseFloat2(p.mark_price),
          unrealisedPnl: parseFloat2(p.unrealised_pnl),
          realisedPnl: parseFloat2(p.realised_pnl),
          margin: parseFloat2(p.margin),
          side: size >= 0 ? "LONG" : "SHORT",
        });
      }
    }
  }

  // --- Open orders ---
  type OrderRaw = { id?: number; symbol?: string; side?: string; size?: unknown; limit_price?: unknown; state?: string; created_at?: string };
  const openOrders: { orderId: string; symbol: string; side: string; size: number; price: number; state: string; createdAt: string }[] = [];
  if (ordersRes.status === "fulfilled" && ordersRes.value.ok) {
    const d = ordersRes.value.data as { result?: { data?: OrderRaw[] } };
    for (const o of d.result?.data ?? []) {
      openOrders.push({
        orderId: String(o.id ?? ""),
        symbol: o.symbol ?? "",
        side: o.side ?? "",
        size: parseFloat2(o.size),
        price: parseFloat2(o.limit_price),
        state: o.state ?? "",
        createdAt: o.created_at ?? "",
      });
    }
  }

  // Wallet error check
  let walletError: string | undefined;
  if (walletRes.status === "fulfilled" && !walletRes.value.ok) {
    const d = walletRes.value.data as { error?: { code?: string } };
    const code = d?.error?.code ?? `HTTP ${walletRes.value.status}`;
    if (code === "ip_not_whitelisted_for_api_key") {
      walletError = `IP not whitelisted. Add this Vercel server IP to Delta Exchange API key whitelist: ${outboundIP}`;
    } else {
      walletError = code;
    }
  } else if (walletRes.status === "rejected") {
    walletError = String(walletRes.reason);
  }

  const usdtWallet = wallets.find((w) => w.asset === "USDT" || w.asset === "USD");

  return NextResponse.json({
    configured: true,
    testnet: process.env.DELTA_TESTNET === "true",
    walletUsdt: usdtWallet?.availableBalance ?? 0,
    account: {
      wallets,
      positions,
      openOrders,
      fetchedAt: new Date().toISOString(),
      error: walletError,
    },
  });
}
