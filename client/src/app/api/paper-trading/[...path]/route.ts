/**
 * Paper Trading API — serves both the Delta and the Forex terminals.
 *
 * The first path segment is the VENUE (`delta` or `forex`) and the second is
 * the action, so one route backs two modules rather than two near-identical
 * copies drifting apart.
 *
 * NOT A PROXY. Unlike the desk routes beside it, nothing here forwards to the
 * Go engine. Delta data comes from Delta's public market-data endpoints and
 * forex data from Yahoo Finance; neither needs a credential. There is no code
 * path from this route to a real order, on any venue, at all.
 *
 * AUTH is the middleware's: `client/src/middleware.ts` validates the
 * raig_session JWT at the edge for every path outside PUBLIC_PATHS, so an
 * unauthenticated request never arrives here.
 *
 * WHY POST EXISTS, when the screener's API is read-only: these terminals own
 * state. Placing, cancelling, closing and resetting are the module. Every
 * mutation is against paper money, and `reset` additionally refuses without an
 * explicit confirmation because the trade log is the only record of what the
 * account did.
 */

import { NextRequest, NextResponse } from "next/server";

import {
  cancelOrder,
  closePosition,
  DISPLAY_DEPTH,
  getVenue,
  modifyPosition,
  OrderRejected,
  PaperTradingUnavailable,
  placeOrder,
  resetAccount,
  runCycle,
  setAccountSettings,
  snapshot,
  type PlaceParams,
} from "@/lib/paperTrading/engine";
import type { OrderSide, OrderType, TimeInForce } from "@/lib/paperTrading/types";
import { fetchTrades } from "@/lib/paperTrading/venues/delta";
import { fetchDerivedTape } from "@/lib/paperTrading/venues/forex";

/**
 * A cold snapshot lists every instrument on the venue — one request for Delta,
 * eighteen for the forex desk — and then runs a cycle that may replay bars for
 * every open symbol. Sixty gives room for a slow upstream rather than cutting
 * the cycle off mid-replay, which would leave orders half-resolved.
 */
export const maxDuration = 60;
export const dynamic = "force-dynamic";

type RouteCtx = { params: Promise<{ path: string[] }> };

function qp(req: NextRequest, key: string): string | null {
  const v = req.nextUrl.searchParams.get(key);
  return v === null || v.trim() === "" ? null : v.trim();
}

function qn(req: NextRequest, key: string, fallback: number): number {
  const v = qp(req, key);
  if (v === null) return fallback;
  const n = Number(v);
  return Number.isFinite(n) ? n : fallback;
}

function ok(body: unknown): NextResponse {
  return NextResponse.json({ ok: true, ...(body as object) }, { headers: { "cache-control": "no-store" } });
}

function fail(e: unknown): NextResponse {
  // A rejected order and an unconfigured deployment are different failures and
  // get different codes, so the terminal can show the trader a reason rather
  // than one generic error for both.
  if (e instanceof OrderRejected) {
    return NextResponse.json({ ok: false, error: e.message, rejected: true }, { status: 422 });
  }
  if (e instanceof PaperTradingUnavailable) {
    return NextResponse.json({ ok: false, error: e.message }, { status: 503 });
  }
  return NextResponse.json(
    { ok: false, error: e instanceof Error ? e.message : "paper trading request failed" },
    { status: 500 },
  );
}

export async function GET(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const venueId = (path?.[0] ?? "").toLowerCase();
  const action = (path?.[1] ?? "snapshot").toLowerCase();

  try {
    const venue = getVenue(venueId);

    switch (action) {
      case "snapshot":
        return ok(await snapshot(venueId, qp(req, "tick") !== "false"));

      case "instruments":
        return ok({ instruments: await venue.listInstruments() });

      case "book": {
        const symbol = qp(req, "symbol");
        if (!symbol) return NextResponse.json({ ok: false, error: "book needs ?symbol=" }, { status: 400 });
        const book = await venue.getBook(symbol.toUpperCase(), qn(req, "depth", DISPLAY_DEPTH));
        if (!book) {
          return NextResponse.json(
            { ok: false, error: `no order book available for ${symbol} right now` },
            { status: 502 },
          );
        }
        return ok({ book });
      }

      case "candles": {
        const symbol = qp(req, "symbol");
        if (!symbol) return NextResponse.json({ ok: false, error: "candles needs ?symbol=" }, { status: 400 });
        const resolution = qp(req, "resolution") ?? "5m";
        const spec = venue.resolutions.find((x) => x.key === resolution);
        if (!spec) {
          return NextResponse.json(
            { ok: false, error: `${venue.label} does not serve ${resolution}; it has ${venue.resolutions.map((x) => x.key).join(", ")}` },
            { status: 422 },
          );
        }
        const bars = Math.max(20, Math.min(qn(req, "bars", 300), 1_000));
        const to = Math.floor(Date.now() / 1000);
        const from = to - bars * spec.seconds;
        return ok({
          symbol: symbol.toUpperCase(),
          resolution,
          candles: await venue.getCandles(symbol.toUpperCase(), resolution, from, to),
        });
      }

      case "trades": {
        const symbol = qp(req, "symbol");
        if (!symbol) return NextResponse.json({ ok: false, error: "trades needs ?symbol=" }, { status: 400 });
        const limit = Math.max(5, Math.min(qn(req, "limit", 40), 100));
        // Delta publishes a real print-by-print tape. The forex venue does not,
        // and no free feed does, so its tape is reconstructed from 1-minute
        // bars and flagged `derived` — the UI renders the two differently
        // rather than passing a reconstruction off as prints.
        if (venueId === "delta") {
          return ok({ symbol: symbol.toUpperCase(), derived: false, trades: await fetchTrades(symbol.toUpperCase(), limit) });
        }
        return ok({ symbol: symbol.toUpperCase(), derived: true, trades: await fetchDerivedTape(symbol.toUpperCase(), limit) });
      }

      default:
        return NextResponse.json(
          {
            ok: false,
            error: `unknown read: ${action}`,
            available: ["snapshot", "instruments", "book", "candles", "trades"],
          },
          { status: 404 },
        );
    }
  } catch (e) {
    return fail(e);
  }
}

export async function POST(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const venueId = (path?.[0] ?? "").toLowerCase();
  const action = (path?.[1] ?? "").toLowerCase();

  let body: Record<string, unknown> = {};
  try {
    const text = await req.text();
    if (text) body = JSON.parse(text) as Record<string, unknown>;
  } catch {
    return NextResponse.json({ ok: false, error: "body is not valid JSON" }, { status: 400 });
  }

  try {
    getVenue(venueId);

    switch (action) {
      case "order": {
        const params: PlaceParams = {
          symbol: String(body.symbol ?? ""),
          side: String(body.side ?? "buy") as OrderSide,
          type: String(body.type ?? "market") as OrderType,
          size: Number(body.size ?? 0),
          limitPrice: body.limitPrice == null ? null : Number(body.limitPrice),
          stopPrice: body.stopPrice == null ? null : Number(body.stopPrice),
          leverage: body.leverage == null ? null : Number(body.leverage),
          timeInForce: (body.timeInForce as TimeInForce) ?? "GTC",
          reduceOnly: Boolean(body.reduceOnly),
          postOnly: Boolean(body.postOnly),
          takeProfit: body.takeProfit == null ? null : Number(body.takeProfit),
          stopLoss: body.stopLoss == null ? null : Number(body.stopLoss),
        };
        if (!params.symbol) throw new OrderRejected("an order needs a symbol");
        if (!(params.size > 0)) throw new OrderRejected("an order needs a size above zero");
        if (!["buy", "sell"].includes(params.side)) throw new OrderRejected("side must be buy or sell");
        if (!["market", "limit", "stop_market", "stop_limit"].includes(params.type)) {
          throw new OrderRejected("type must be market, limit, stop_market or stop_limit");
        }
        return ok(await placeOrder(venueId, params));
      }

      case "cancel":
        return ok(await cancelOrder(venueId, String(body.orderId ?? "")));

      case "close":
        return ok(
          await closePosition(
            venueId,
            String(body.positionId ?? ""),
            body.size == null ? null : Number(body.size),
          ),
        );

      case "modify":
        return ok(
          await modifyPosition(venueId, String(body.positionId ?? ""), {
            takeProfit: body.takeProfit === undefined ? undefined : body.takeProfit === null ? null : Number(body.takeProfit),
            stopLoss: body.stopLoss === undefined ? undefined : body.stopLoss === null ? null : Number(body.stopLoss),
          }),
        );

      case "settings":
        return ok({
          account: await setAccountSettings(venueId, {
            leverage: body.leverage == null ? undefined : Number(body.leverage),
            accountType: body.accountType as never,
            marginMode: body.marginMode as never,
          }),
        });

      case "tick":
        return ok({ cycle: await runCycle(venueId, true) });

      case "reset": {
        if (qp(req, "confirm") !== "true") {
          return NextResponse.json(
            {
              ok: false,
              error:
                "Refusing to reset without ?confirm=true. This deletes every order, position and " +
                "closed trade on this desk and returns the balance to its opening figure — the " +
                "trade log is the only record of what the account did, and it is not recoverable.",
            },
            { status: 400 },
          );
        }
        return ok({ cleared: await resetAccount(venueId) });
      }

      default:
        return NextResponse.json(
          {
            ok: false,
            error: `unknown action: ${action}`,
            available: ["order", "cancel", "close", "modify", "settings", "tick", "reset"],
          },
          { status: 404 },
        );
    }
  } catch (e) {
    return fail(e);
  }
}
