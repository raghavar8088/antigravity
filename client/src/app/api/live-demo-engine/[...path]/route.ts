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

/**
 * Actions the demo venue genuinely has no counterpart for.
 *
 * Every entry here was checked against the routes cmd/scalp_prelive actually
 * mounts (live.go registerHTTP + main.go), not against the live engine's API.
 * The two are different programs: the live page talks to cmd/antigravity, which
 * serves the option desk's account, positions, orders and audit; the demo talks
 * to a perpetual desk that serves none of them. Proxying those names anyway
 * returned Go's 404 page, which the browser dropped on the floor — the panels
 * then rendered as empty tables, i.e. as "this desk did nothing".
 */
const NOT_APPLICABLE = new Set([
  // Option desk tables — no options desk on the demo venue at all.
  "closed-positions",
  "orders",
  "daily-pnl",
  "roster",
  // Option positions. The perpetual desk's own open book is a different
  // endpoint (/api/scalp-demo/scalp/live/stats) and the page already reads it.
  "positions",
  // The wallet strip and the audit log live in the options engine.
  "account",
  "audit",
  // Venue Truth reads six private Delta endpoints from cmd/antigravity. The
  // demo engine's own venue check is `reconciliation`, below.
  "venue",
]);

/**
 * Action name → the route the demo engine actually serves.
 *
 * `reconcile` is not `reconciliation`. The name was carried over from the live
 * engine's API and would have 404'd forever, silently: the page's mismatch
 * banner keys off this payload, so the one control that shouts when engine
 * state and venue truth disagree would have stayed quiet by construction.
 */
const UPSTREAM_PATH: Record<string, string> = {
  state: "/scalp/live/stats",
  reconciliation: "/scalp/live/reconcile",
  trades: "/scalp/live/trades",
  paper: "/scalp/live/paper",
};

function demoEngineBase(): string {
  // `||`, not `??`: an env var set to an empty string is a misconfiguration,
  // not a choice. With `??` it survived as "", the fetch target collapsed to a
  // relative path, and the thrown URL error surfaced as the same 502 the page
  // shows when the box is down — two very different faults, one message.
  return (
    process.env.SCALP_DEMO_ENGINE_URL?.trim().replace(/\/+$/, "") || "http://13.233.8.80:8095"
  );
}

function demoToken(): string {
  return process.env.SCALP_DEMO_API_TOKEN?.trim() || process.env.BTC_PRE_LIVE_API_TOKEN?.trim() || "";
}

function notApplicable(reason: string) {
  return NextResponse.json({ items: [], notApplicable: true, reason });
}

/**
 * Shape of the demo perpetual desk's /scalp/live/stats.
 *
 * `enabled` is false until the bridge holds credentials; `armed` is false until
 * an operator arms it, and never survives a restart.
 */
type DemoStats = { enabled?: boolean; stats?: { armed?: boolean; equityUsd?: number } };

/**
 * Translate the demo desk's stats into the state shape the page reads.
 *
 * The page is generated from the live one by scripts/clone_live_demo_page.py,
 * so it reads the OPTIONS engine's LiveState: `armed`, `configured`,
 * `killSwitchActive`, `consecutiveRejects`. The demo engine is a different
 * program and answers with none of those. Passing its payload through meant an
 * entire control card rendered from `undefined` — showing an operator a state
 * that nothing had reported.
 *
 * The adapter belongs here rather than in the page: the page is regenerated,
 * this route is not. The values are constants because they are facts about the
 * venue, not readings — demo.delta.exchange has no options desk, so its options
 * engine is not armed, not configured and has placed no rejected orders. The
 * perpetual desk's real arm state is shown by its own card, which reads
 * /api/scalp-demo directly.
 */
function demoState(body: DemoStats): Record<string, unknown> {
  return {
    state: "DISARMED",
    armed: false,
    configured: false,
    optionsTradingEnabled: false,
    killSwitchActive: false,
    killSwitchControllable: false,
    consecutiveRejects: 0,
    maxConsecutiveRejects: 3,
    ceilingUsd: body.stats?.equityUsd ?? 0,
    /** Demo-only additions, ignored by the generated page's typed reads. */
    demoBridgeEnabled: body.enabled === true,
    demoPerpArmed: body.stats?.armed === true,
  };
}

export async function GET(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  const action = (path ?? []).join("/");

  if (NOT_APPLICABLE.has(action)) {
    return notApplicable(
      "The demo venue runs perpetuals only — there is no demo options desk, so this panel has no source rather than no rows.",
    );
  }

  // Everything else is answered by the demo perpetual engine.
  const base = demoEngineBase();
  const token = demoToken();
  const route = UPSTREAM_PATH[action];
  if (!route) {
    return NextResponse.json({ error: `unknown demo read: ${action}` }, { status: 404 });
  }
  const upstream = `${base}${route}`;

  try {
    const res = await fetch(upstream, {
      cache: "no-store",
      headers: token ? { "X-API-Token": token } : undefined,
      signal: AbortSignal.timeout(25_000),
    });
    const body = await res.text();

    // `state` is the page's reachability probe AND the source of its control
    // card, so it is the one action that must be translated rather than piped.
    if (action === "state" && res.ok) {
      try {
        return NextResponse.json(demoState(JSON.parse(body) as DemoStats));
      } catch {
        // Unparseable upstream is a fault, not an empty desk. Fall through to
        // the raw passthrough so the page reports the real status.
      }
    }

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

/**
 * There are no option-desk mutations on the demo venue, and this route refuses
 * them out loud rather than not existing.
 *
 * The demo page was cloned from the live one and kept its `mutate` helper
 * pointed at /api/live-engine/* — the REAL-money options control plane. A page
 * badged "DEMO — NOT REAL MONEY" therefore had an arm switch, a kill switch and
 * a "Panic — CLOSE ALL" button that all reached the live wallet. Handling POST
 * here means the demo page has a demo-side endpoint to talk to, and one that
 * cannot route an order anywhere: it has no upstream call in it.
 *
 * The perpetual desk — the only thing the demo venue can actually trade — is
 * armed through /api/scalp-demo/scalp/live/{arm,disarm}, which carries the demo
 * credentials and the demo port.
 */
export async function POST(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  const action = (path ?? []).join("/");
  return NextResponse.json(
    {
      ok: false,
      notApplicable: true,
      error: `${action || "this action"} is an options-engine control, and the demo venue has no options desk. Arm the perpetual desk instead — it is the only thing this venue trades.`,
    },
    { status: 501 },
  );
}
