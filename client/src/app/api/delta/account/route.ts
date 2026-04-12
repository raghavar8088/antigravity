import { NextResponse } from "next/server";
import { deltaFetch } from "@/lib/deltaSign";

function pf(v: unknown): number {
  if (typeof v === "number") return v;
  if (typeof v === "string") return parseFloat(v) || 0;
  return 0;
}

export async function GET() {
  try {
    const key = process.env.DELTA_API_KEY;
    const secret = process.env.DELTA_API_SECRET;

    if (!key || !secret) {
      return NextResponse.json({
        configured: false,
        testnet: false,
        walletUsdt: 0,
        account: { wallets: [], positions: [], openOrders: [], fetchedAt: new Date().toISOString(), error: "DELTA_API_KEY and DELTA_API_SECRET not configured in Vercel environment variables" },
      });
    }

    // Get Vercel outbound IP (for whitelist error message)
    let outboundIP = "";
    try {
      const ipRes = await fetch("https://api.ipify.org?format=json", { cache: "no-store" });
      const ipJson = await ipRes.json() as { ip?: string };
      outboundIP = ipJson.ip ?? "";
    } catch { /* ignore */ }

    const [walletRes, posRes, ordersRes] = await Promise.allSettled([
      deltaFetch("/v2/wallet/balances"),
      deltaFetch("/v2/positions/margined"),
      deltaFetch("/v2/orders?state=open"),
    ]);

    // Wallets
    type WR = { asset_symbol?: string; balance?: unknown; available_balance?: unknown; blocked_margin?: unknown; unrealised_cashflow?: unknown };
    const wallets: { asset: string; balance: number; availableBalance: number; blockedBalance: number; unrealisedPnl: number }[] = [];
    if (walletRes.status === "fulfilled" && walletRes.value.ok) {
      for (const w of (walletRes.value.data as { result?: WR[] }).result ?? []) {
        const bal = pf(w.balance), avail = pf(w.available_balance);
        if (bal !== 0 || avail !== 0) wallets.push({ asset: w.asset_symbol ?? "", balance: bal, availableBalance: avail, blockedBalance: pf(w.blocked_margin), unrealisedPnl: pf(w.unrealised_cashflow) });
      }
    }

    // Positions
    type PR = { symbol?: string; product_id?: number; size?: unknown; entry_price?: unknown; mark_price?: unknown; unrealised_pnl?: unknown; realised_pnl?: unknown; margin?: unknown };
    const positions: { symbol: string; productId: number; size: number; entryPrice: number; markPrice: number; unrealisedPnl: number; realisedPnl: number; margin: number; side: string }[] = [];
    if (posRes.status === "fulfilled" && posRes.value.ok) {
      for (const p of (posRes.value.data as { result?: PR[] }).result ?? []) {
        const size = pf(p.size);
        if (size !== 0) positions.push({ symbol: p.symbol ?? "", productId: p.product_id ?? 0, size, entryPrice: pf(p.entry_price), markPrice: pf(p.mark_price), unrealisedPnl: pf(p.unrealised_pnl), realisedPnl: pf(p.realised_pnl), margin: pf(p.margin), side: size >= 0 ? "LONG" : "SHORT" });
      }
    }

    // Open orders
    type OR = { id?: number; symbol?: string; side?: string; size?: unknown; limit_price?: unknown; state?: string; created_at?: string };
    const openOrders: { orderId: string; symbol: string; side: string; size: number; price: number; state: string; createdAt: string }[] = [];
    if (ordersRes.status === "fulfilled" && ordersRes.value.ok) {
      for (const o of (ordersRes.value.data as { result?: { data?: OR[] } }).result?.data ?? []) {
        openOrders.push({ orderId: String(o.id ?? ""), symbol: o.symbol ?? "", side: o.side ?? "", size: pf(o.size), price: pf(o.limit_price), state: o.state ?? "", createdAt: o.created_at ?? "" });
      }
    }

    // Error check
    let walletError: string | undefined;
    if (walletRes.status === "fulfilled" && !walletRes.value.ok) {
      const code = (walletRes.value.data as { error?: { code?: string } })?.error?.code ?? `HTTP ${walletRes.value.status}`;
      walletError = code === "ip_not_whitelisted_for_api_key"
        ? `IP not whitelisted. Add Vercel server IP to Delta Exchange API key whitelist: ${outboundIP}`
        : code;
    } else if (walletRes.status === "rejected") {
      walletError = String(walletRes.reason);
    }

    const usdtWallet = wallets.find((w) => w.asset === "USDT" || w.asset === "USD");

    return NextResponse.json({
      configured: true,
      testnet: process.env.DELTA_TESTNET === "true",
      walletUsdt: usdtWallet?.availableBalance ?? 0,
      account: { wallets, positions, openOrders, fetchedAt: new Date().toISOString(), error: walletError },
    });

  } catch (err) {
    return NextResponse.json({
      configured: true, testnet: false, walletUsdt: 0,
      account: { wallets: [], positions: [], openOrders: [], fetchedAt: new Date().toISOString(), error: `Server error: ${String(err)}` },
    });
  }
}
