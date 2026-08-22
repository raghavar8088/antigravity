/**
 * Crypto Screener API.
 *
 * NOT A PROXY, unlike every other route in this folder. The desk routes beside
 * it forward to the Go engine on the Lightsail box; this one computes its
 * answer here, because everything it reads is a PUBLIC Delta market-data
 * endpoint. Nothing on this path holds a key, signs a request, or touches the
 * real-money trading process.
 *
 * AUTH is the middleware's, not this file's. `client/src/middleware.ts` treats
 * every path outside PUBLIC_PATHS as protected and validates the raig_session
 * JWT signature at the edge, so an unauthenticated request never reaches this
 * handler. The neighbouring proxies re-verify because they mint an engine token
 * off the session; there is no token to mint here.
 *
 * READ-ONLY. There is no POST. A screener describes the market; it has nothing
 * to mutate, and an endpoint that could be POSTed to would be an invitation to
 * find out what it does.
 *
 * `?fresh=true` bypasses the snapshot cache and rebuilds from the venue. It is
 * the expensive path — roughly 440 requests to Delta — so it is behind an
 * explicit flag rather than being the default.
 */

import { NextRequest, NextResponse } from "next/server";

import {
  basisBoard,
  config,
  correlationBoard,
  fundingBoard,
  microBoard,
  momentumBoard,
  oiBoard,
  patternBoard,
  ScreenerRequestError,
  sectorBoard,
  sectorDrilldown,
  setups,
  sources,
  summary,
  symbolDetail,
  volumeBoard,
} from "@/lib/cryptoScreener/engine";
import { DeltaFeedError } from "@/lib/cryptoScreener/delta";
import type { HorizonKey } from "@/lib/cryptoScreener/horizons";
import type { PlanKind } from "@/lib/cryptoScreener/plans";

/**
 * A cold build fetches ~440 candle series from the venue; measured at about 10
 * seconds. Sixty gives room for a slow upstream without the request being cut
 * off mid-scan, which would leave the caller unable to tell a timeout from an
 * empty market.
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

/**
 * `minTurnover` distinguishes three states, and collapsing them would be a real
 * loss: absent means "use the board's own default", `0` means "the caller
 * explicitly wants the illiquid tail included", and any other number is a
 * floor. `0` is the interesting one — it is how a reader inspects the 200-odd
 * contracts the default hides, and mapping it to the default would make that
 * impossible.
 */
function turnoverFloor(req: NextRequest, fallback: number | null): number | null {
  const v = qp(req, "minTurnover");
  if (v === null) return fallback;
  const n = Number(v);
  if (!Number.isFinite(n)) return fallback;
  return n <= 0 ? 0 : n;
}

/** As above, but leaves `undefined` in place so the board applies its own default. */
function turnoverFloorOpt(req: NextRequest): number | null | undefined {
  const v = qp(req, "minTurnover");
  if (v === null) return undefined;
  const n = Number(v);
  if (!Number.isFinite(n)) return undefined;
  return n <= 0 ? 0 : n;
}

export async function GET(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const action = (path?.[0] ?? "").toLowerCase();
  const arg = path?.[1] ? decodeURIComponent(path[1]) : null;
  const fresh = qp(req, "fresh") === "true";

  try {
    switch (action) {
      case "config":
        return ok(config());

      case "summary":
        return ok(await summary(fresh));

      case "momentum":
        if (arg) return ok(await symbolDetail(arg, fresh));
        return ok(
          await momentumBoard({
            horizon: (qp(req, "horizon") ?? "1d") as HorizonKey,
            sector: qp(req, "sector"),
            assetClass: qp(req, "assetClass"),
            limit: qn(req, "limit", 120),
            minTurnover: turnoverFloorOpt(req),
            fresh,
          }),
        );

      case "sectors":
        if (arg) {
          return ok(await sectorDrilldown(arg, (qp(req, "horizon") ?? "1d") as HorizonKey, fresh));
        }
        return ok(await sectorBoard(qp(req, "horizon") as HorizonKey | null, fresh));

      case "volume":
        return ok(
          await volumeBoard(qp(req, "window") ?? "1d", qp(req, "state"), qn(req, "limit", 80), fresh),
        );

      case "funding":
        return ok(
          await fundingBoard(
            qp(req, "side") as "longs" | "shorts" | null,
            qn(req, "limit", 120),
            turnoverFloor(req, 250_000),
            fresh,
          ),
        );

      case "open-interest":
        return ok(
          await oiBoard(qp(req, "buildup"), qn(req, "limit", 120), turnoverFloor(req, 250_000), fresh),
        );

      case "basis":
        return ok(
          await basisBoard(qp(req, "state"), qn(req, "limit", 120), turnoverFloor(req, 250_000), fresh),
        );

      case "microstructure":
        return ok(
          await microBoard(
            qp(req, "tradableOnly") === "true",
            qn(req, "limit", 250),
            turnoverFloor(req, null),
            fresh,
          ),
        );

      case "correlation":
        return ok(
          await correlationBoard(qn(req, "limit", 250), turnoverFloor(req, 250_000), fresh),
        );

      case "patterns":
        return ok(
          await patternBoard({
            timeframe: qp(req, "timeframe"),
            pattern: qp(req, "pattern"),
            family: qp(req, "family"),
            state: qp(req, "state"),
            direction: qp(req, "direction"),
            sector: qp(req, "sector"),
            limit: qn(req, "limit", 300),
            fresh,
          }),
        );

      case "setups":
        return ok(
          await setups((qp(req, "kind") ?? "scalp") as PlanKind, qn(req, "limit", 40), fresh),
        );

      case "sources":
        return ok(await sources(fresh));

      default:
        return NextResponse.json(
          {
            ok: false,
            error: `unknown crypto-screener read: ${action || "(none)"}`,
            available: [
              "config",
              "summary",
              "momentum",
              "momentum/{symbol}",
              "sectors",
              "sectors/{sector}",
              "volume",
              "funding",
              "open-interest",
              "basis",
              "microstructure",
              "correlation",
              "patterns",
              "setups",
              "sources",
            ],
          },
          { status: 404 },
        );
    }
  } catch (e) {
    // A bad request from the caller and an upstream outage are different
    // failures and get different codes, so the page can tell the reader which
    // one happened instead of showing one generic error for both.
    if (e instanceof ScreenerRequestError) {
      return NextResponse.json({ ok: false, error: e.message }, { status: 422 });
    }
    if (e instanceof DeltaFeedError) {
      return NextResponse.json(
        {
          ok: false,
          error: e.message,
          hint:
            "This is the venue, not the app. Delta's public market-data endpoints are unauthenticated, " +
            "so this is either an outage or a rate limit rather than a credential problem.",
        },
        { status: 502 },
      );
    }
    return NextResponse.json(
      { ok: false, error: e instanceof Error ? e.message : "crypto screener failed" },
      { status: 500 },
    );
  }
}

function ok(body: unknown): NextResponse {
  return NextResponse.json({ ok: true, ...(body as object) }, {
    headers: { "cache-control": "no-store" },
  });
}
