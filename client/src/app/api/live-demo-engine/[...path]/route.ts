/**
 * Live DEMO Engine control plane.
 *
 * The Live Engine page draws from two processes: the perpetual engine (scalp)
 * and the options engine (antigravity, which also serves Venue Truth, the
 * roster and the audit log). The demo side has only the first — there is no
 * second options engine, and option trading is off by design.
 *
 * So the options-only actions answer here rather than upstream, and they answer
 * with an explicit `notApplicable` marker instead of a bare empty list.
 *
 * That distinction matters. An empty list renders identically to "this desk
 * traded nothing", which is a claim about results. `notApplicable` says the
 * desk does not exist on this venue, which is a claim about wiring. This
 * session has already lost hours to a zero that meant "not calculated" being
 * read as "zero risk", and to a page that reported 0 open while the venue held
 * real positions — silent zeros are the recurring failure here, not an
 * acceptable default.
 */
import { NextRequest, NextResponse } from "next/server";

/** Actions the demo venue genuinely has no counterpart for. */
const OPTIONS_ONLY = new Set(["closed-positions", "orders", "daily-pnl", "roster"]);

function demoEngineBase(): string {
  return (
    process.env.SCALP_DEMO_ENGINE_URL?.trim().replace(/\/+$/, "") ?? "http://13.233.8.80:8095"
  );
}

function demoToken(): string {
  return process.env.SCALP_DEMO_API_TOKEN?.trim() ?? process.env.BTC_PRE_LIVE_API_TOKEN?.trim() ?? "";
}

export async function GET(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  const action = (path ?? []).join("/");

  if (OPTIONS_ONLY.has(action)) {
    return NextResponse.json({
      items: [],
      notApplicable: true,
      reason:
        "The demo venue runs perpetuals only — there is no demo options desk, so this table has no source rather than no rows.",
    });
  }

  // Everything else is answered by the demo perpetual engine.
  const base = demoEngineBase();
  const token = demoToken();
  const upstream = `${base}/scalp/live/${action === "state" ? "stats" : action}`;

  try {
    const res = await fetch(upstream, {
      cache: "no-store",
      headers: token ? { "X-API-Token": token } : undefined,
      signal: AbortSignal.timeout(25_000),
    });
    const body = await res.text();
    return new NextResponse(body, {
      status: res.status,
      headers: { "content-type": res.headers.get("content-type") ?? "application/json" },
    });
  } catch (err) {
    // Report the reach failure as a failure. Returning an empty payload here
    // would render as a flat, healthy-looking desk while the engine is down.
    return NextResponse.json(
      { error: `demo engine unreachable: ${err instanceof Error ? err.message : String(err)}` },
      { status: 502 },
    );
  }
}
