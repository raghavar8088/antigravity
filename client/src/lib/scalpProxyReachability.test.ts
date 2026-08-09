/**
 * Every control the Live Engine page POSTs must be reachable through the proxy.
 *
 * This exists because a control shipped that rendered, responded to clicks, and
 * reached nothing: the per-stream ON/OFF switch posted to /scalp/live/strategy,
 * which was missing from the proxy's MUTATION_PATHS, so every click was
 * rejected one layer before the engine. The engine endpoint itself worked
 * perfectly when called directly with curl — which is exactly why it was
 * verified as working and shipped broken.
 *
 * It is the second time this desk has had a control that existed only visually.
 * The first cost 105 minutes of an operator believing strategies were live when
 * the visible toggle armed a different engine.
 *
 * Testing the engine is not enough; the browser's path has to be tested too.
 */
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const ROOT = join(__dirname, "..", "..");

function read(rel: string): string {
  return readFileSync(join(ROOT, rel), "utf8");
}

/** Paths the proxy will forward a POST to. */
function mutationPaths(proxy: string): string[] {
  const block = proxy.match(/const MUTATION_PATHS = \[([\s\S]*?)\];/);
  if (!block) throw new Error("MUTATION_PATHS not found — the proxy changed shape");
  return [...block[1]!.matchAll(/"([^"]+)"/g)].map((m) => m[1]!);
}

/**
 * Upstream paths the page POSTs to, as `/scalp/...`.
 *
 * The page calls `/api/scalp/scalp/live/x`; the proxy strips the `/api/scalp`
 * mount and forwards `/scalp/live/x`.
 */
function postedScalpPaths(page: string): string[] {
  const found = new Set<string>();
  // fetch("<url>", { ... method: "POST" ... }) — match the url, then confirm a
  // POST appears before the call's closing brace.
  for (const m of page.matchAll(/fetch\(\s*["'`](\/api\/scalp(?:-demo)?\/scalp\/[^"'`]+)["'`]\s*,\s*\{([\s\S]{0,400}?)\}/g)) {
    const [, url, opts] = m;
    if (!/method:\s*["']POST["']/.test(opts!)) continue;
    found.add(url!.replace(/^\/api\/scalp(?:-demo)?/, ""));
  }
  return [...found];
}

/**
 * Assert one posted path is reachable.
 *
 * Some calls are template literals — the arm control posts
 * `/scalp/live/${action}` with "arm" or "disarm" — which cannot be resolved
 * statically. Rather than drop those (which would silently stop checking the
 * single most dangerous control on the page), the literal prefix must still
 * match something the proxy allows. That catches a path pointing at an
 * entirely unknown area while tolerating the interpolation.
 */
function expectReachable(posted: string, allowed: string[], who: string): void {
  const marker = posted.indexOf("${");
  if (marker < 0) {
    expect(allowed, `${posted} is POSTed by the ${who} but the proxy will reject it`).toContain(posted);
    return;
  }
  const prefix = posted.slice(0, marker);
  const matches = allowed.filter((a) => a.startsWith(prefix));
  expect(
    matches.length,
    `${posted} is POSTed by the ${who}; no allowed path starts with "${prefix}", so every value it can take is rejected`,
  ).toBeGreaterThan(0);
}

describe("Live Engine controls reach the engine", () => {
  it("every POST the page makes is allowed by the scalp proxy", () => {
    const posted = postedScalpPaths(read("src/app/live-engine/page.tsx"));
    const allowed = mutationPaths(read("src/app/api/scalp/[...path]/route.ts"));

    // Guard the guard: if the extraction finds nothing, the test would pass
    // vacuously while the page was full of dead controls.
    expect(posted.length).toBeGreaterThan(0);

    for (const p of posted) {
      expectReachable(p, allowed, "Live Engine page");
    }
  });

  it("the demo page's POSTs are allowed by the demo proxy", () => {
    const posted = postedScalpPaths(read("src/app/live-demo-engine/page.tsx"));
    const allowed = mutationPaths(read("src/app/api/scalp-demo/[...path]/route.ts"));

    expect(posted.length).toBeGreaterThan(0);
    for (const p of posted) {
      expectReachable(p, allowed, "Live Demo Engine page");
    }
  });

  it("the per-stream switch specifically is reachable on both desks", () => {
    // Named explicitly: this is the control that shipped broken, and a
    // regression here is silent from the browser.
    for (const proxy of ["src/app/api/scalp/[...path]/route.ts", "src/app/api/scalp-demo/[...path]/route.ts"]) {
      expect(mutationPaths(read(proxy)), proxy).toContain("/scalp/live/strategy");
    }
  });
});
