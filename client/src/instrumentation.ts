/**
 * Next.js server instrumentation — runs once when the server process starts.
 * Logs the full environment readiness report so startup failures are visible
 * immediately in Vercel Function logs and local terminal output.
 *
 * Next.js 15+ enables this automatically (no experimental flag needed).
 * Place this file at src/instrumentation.ts (App Router picks it up automatically).
 */

export async function register() {
  // Only run on the server runtime — skip Edge and browser contexts.
  if (
    process.env.NEXT_RUNTIME === "edge" ||
    typeof process === "undefined"
  ) {
    return;
  }

  // Dynamic import keeps this out of the browser bundle entirely.
  const { logEnvReport } = await import("@/lib/envCheck");
  logEnvReport();
}
