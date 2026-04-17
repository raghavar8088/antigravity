import { NextRequest, NextResponse } from "next/server";
import { deltaFetch, type DeltaKeyOverride } from "@/lib/deltaSign";

function pf(v: unknown): number {
  if (typeof v === "number") return v;
  if (typeof v === "string") return parseFloat(v) || 0;
  return 0;
}

export async function GET(req: NextRequest) {
  try {
    // Allow UI-supplied keys via request headers (takes priority over env vars)
    const overrides: DeltaKeyOverride = {};
    const hKey = req.headers.get("x-delta-api-key");
    const hSecret = req.headers.get("x-delta-api-secret");
    const hTestnet = req.headers.get("x-delta-testnet");
    if (hKey) overrides.apiKey = hKey;
    if (hSecret) overrides.apiSecret = hSecret;
    if (hTestnet !== null) overrides.testnet = hTestnet === "true";

    const key = overrides.apiKey || process.env.DELTA_API_KEY;
    const secret = overrides.apiSecret || process.env.DELTA_API_SECRET;

    if (!key || !secret) {
      return NextResponse.json({
        configured: false,
        testnet: false,
        walletUsdt: 0,
        account: {
          wallets: [],
          positions: [],
          openOrders: [],
          fetchedAt: new Date().toISOString(),
          error: "No API keys — enter them in the Delta Live module config panel",
        },
      });
    }

    // Fetch wallet, positions, open orders in parallel
    const [walletRes, posRes, ordersRes] = await Promise.allSettled([
      deltaFetch("/v2/wallet/balances", "GET", "", overrides),
      deltaFetch("/v2/positions/margined", "GET", "", overrides),
      deltaFetch("/v2/orders?state=open", "GET", "", overrides),
    ]);

    // --- Wallets ---
    type WR = { asset_symbol?: string; balance?: unknown; available_balance?: unknown; blocked_margin?: unknown; unrealised_cashflow?: unknown };
    const wallets: { asset: string; balance: number; availableBalance: number; blockedBalance: number; unrealisedPnl: number }[] = [];
    if (walletRes.status === "fulfilled" && walletRes.value.ok) {
      for (const w of (walletRes.value.data as { result?: WR[] }).result ?? []) {
        const bal = pf(w.balance), avail = pf(w.available_balance);
        if (bal !== 0 || avail !== 0) {
          wallets.push({
            asset: w.asset_symbol ?? "",
            balance: bal,
            availableBalance: avail,
            blockedBalance: pf(w.blocked_margin),
            unrealisedPnl: pf(w.unrealised_cashflow),
          });
        }
      }
    }

    // --- Positions ---
    type PR = { symbol?: string; product_id?: number; size?: unknown; entry_price?: unknown; mark_price?: unknown; unrealised_pnl?: unknown; realised_pnl?: unknown; margin?: unknown };
    const positions: { symbol: string; productId: number; size: number; entryPrice: number; markPrice: number; unrealisedPnl: number; realisedPnl: number; margin: number; side: string }[] = [];
    if (posRes.status === "fulfilled" && posRes.value.ok) {
      for (const p of (posRes.value.data as { result?: PR[] }).result ?? []) {
        const size = pf(p.size);
        if (size !== 0) {
          positions.push({
            symbol: p.symbol ?? "",
            productId: p.product_id ?? 0,
            size,
            entryPrice: pf(p.entry_price),
            markPrice: pf(p.mark_price),
            unrealisedPnl: pf(p.unrealised_pnl),
            realisedPnl: pf(p.realised_pnl),
            margin: pf(p.margin),
            side: size >= 0 ? "LONG" : "SHORT",
          });
        }
      }
    }

    // --- Open orders ---
    type OR = { id?: number; symbol?: string; side?: string; size?: unknown; limit_price?: unknown; state?: string; created_at?: string };
    const openOrders: { orderId: string; symbol: string; side: string; size: number; price: number; state: string; createdAt: string }[] = [];
    if (ordersRes.status === "fulfilled" && ordersRes.value.ok) {
      for (const o of (ordersRes.value.data as { result?: { data?: OR[] } }).result?.data ?? []) {
        openOrders.push({
          orderId: String(o.id ?? ""),
          symbol: o.symbol ?? "",
          side: o.side ?? "",
          size: pf(o.size),
          price: pf(o.limit_price),
          state: o.state ?? "",
          createdAt: o.created_at ?? "",
        });
      }
    }

    // Error check
    let walletError: string | undefined;
    if (walletRes.status === "fulfilled" && !walletRes.value.ok) {
      const code = (walletRes.value.data as { error?: { code?: string } })?.error?.code ?? `HTTP ${walletRes.value.status}`;
      walletError = code === "ip_not_whitelisted_for_api_key"
        ? "IP not whitelisted on Delta Exchange API key. Remove IP restriction from Delta Exchange → Settings → API Keys."
        : code;
    } else if (walletRes.status === "rejected") {
      walletError = String(walletRes.reason);
    }

    const usdtWallet = wallets.find((w) => w.asset === "USDT" || w.asset === "USD");

    return NextResponse.json({
      configured: true,
      testnet: overrides.testnet ?? (process.env.DELTA_TESTNET === "true"),
      walletUsdt: usdtWallet?.availableBalance ?? 0,
      account: {
        wallets,
        positions,
        openOrders,
        fetchedAt: new Date().toISOString(),
        error: walletError,
      },
    });

  } catch (err) {
    return NextResponse.json({
      configured: true,
      testnet: false,
      walletUsdt: 0,
      account: {
        wallets: [],
        positions: [],
        openOrders: [],
        fetchedAt: new Date().toISOString(),
        error: `Server error: ${String(err)}`,
      },
    });
  }
}
