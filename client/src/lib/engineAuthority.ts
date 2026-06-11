/**
 * The Go engine on AWS Lightsail is the SOLE execution authority.
 * All client-side trade generation, paper-desk workers, and cron ticks are
 * permanently disabled. This function is hardcoded to true — there is no
 * env-var override path. All write routes return HTTP 410 unconditionally.
 *
 * SINGLE MOCK TRADING AUTHORITY PROGRAM — Phase 7 enforcement (2026-06-11).
 */

export function isEngineExecutionAuthority(): boolean {
  return true;
}

export const ENGINE_AUTHORITY_SKIP_REASON =
  "Go engine is sole execution authority — client-side execution permanently disabled";
