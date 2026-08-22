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
 * MOSTLY READ-ONLY. Every screener board is a GET; a screener describes the
 * market and has nothing to mutate. The two exceptions belong to the paper
 * desk, which does own state: `POST /paper/run` forces a manage-then-scan
 * cycle, and `POST /paper/reset` wipes the desk behind an explicit
 * `?confirm=true`. Neither can reach a broker — the desk holds no keys and has
 * no order-routing path.
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
import {
  listPositions,
  manualRun,
  paperConfigured,
  PaperUnavailableError,
  reset as paperReset,
  summary as paperSummary,
} from "@/lib/cryptoScreener/paper/engine";
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

      case "paper": {
        const sub = (path?.[1] ?? "summary").toLowerCase();
        if (!paperConfigured()) return ok(paperOffline());
        if (sub === "summary") return ok(await paperSummary(true));
        if (sub === "positions") {
          const status = (qp(req, "status") ?? "OPEN").toUpperCase() === "CLOSED" ? "CLOSED" : "OPEN";
          return ok(
            await listPositions(status, qp(req, "family"), qp(req, "symbol"), qn(req, "limit", 200)),
          );
        }
        return NextResponse.json(
          { ok: false, error: `unknown paper read: ${sub}`, available: ["paper/summary", "paper/positions"] },
          { status: 404 },
        );
      }

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
              "paper/summary",
              "paper/positions",
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
    // The desk being unconfigured is not a failure of this request — it is a
    // deployment fact, and it gets a 503 with the reason rather than a 500 that
    // reads as a crash.
    if (e instanceof PaperUnavailableError) {
      return NextResponse.json({ ok: false, error: e.message }, { status: 503 });
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

/**
 * What the paper tab is told when the desk has no database.
 *
 * A 200 with `configured: false` rather than an error, because this is a
 * complete and correct answer to "what is the state of the desk" — it lets the
 * page explain the situation instead of rendering a red banner that looks like
 * an outage.
 */
function paperOffline() {
  return {
    configured: false,
    reason:
      "MONGODB_URI is not set on this deployment, so the paper desk has nowhere to keep positions. " +
      "It is reported as unavailable rather than run from memory: on serverless, an in-memory desk " +
      "loses positions between requests and then reports the survivors as its record, which is worse " +
      "than having no desk at all.",
    books: [],
    families: [],
    totals: null,
  };
}

/**
 * Paper-desk mutations only.
 *
 * `run` forces a cycle past the 60-second throttle that read-triggered ticks
 * obey. `reset` deletes every book, position and trade, and refuses without an
 * explicit confirmation: the trade log is the only record of which signals
 * worked, and deleting it is not undoable.
 */
export async function POST(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  if ((path?.[0] ?? "").toLowerCase() !== "paper") {
    return NextResponse.json(
      { ok: false, error: "the crypto screener has no mutations outside the paper desk" },
      { status: 404 },
    );
  }
  const action = (path?.[1] ?? "").toLowerCase();

  try {
    if (action === "run") {
      return ok({ cycle: await manualRun() });
    }
    if (action === "reset") {
      if (qp(req, "confirm") !== "true") {
        return NextResponse.json(
          {
            ok: false,
            error:
              "Refusing to wipe the paper desk without ?confirm=true. The trade log is the only " +
              "record of which signals actually worked; deleting it is not undoable.",
          },
          { status: 400 },
        );
      }
      return ok({ cleared: await paperReset() });
    }
    return NextResponse.json(
      { ok: false, error: `unknown paper action: ${action}`, available: ["paper/run", "paper/reset"] },
      { status: 404 },
    );
  } catch (e) {
    if (e instanceof PaperUnavailableError) {
      return NextResponse.json({ ok: false, error: e.message }, { status: 503 });
    }
    return NextResponse.json(
      { ok: false, error: e instanceof Error ? e.message : "paper action failed" },
      { status: 500 },
    );
  }
}
