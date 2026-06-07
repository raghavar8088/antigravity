/**
 * When true, the Go engine on AWS Lightsail is the sole execution authority.
 * The legacy TypeScript paper worker and cron tick must not write MongoDB state.
 */

export function isEngineExecutionAuthority(): boolean {
  const v = process.env.ENGINE_EXECUTION_AUTHORITY?.trim().toLowerCase();
  if (v === "0" || v === "false" || v === "off") return false;
  return true;
}

export const ENGINE_AUTHORITY_SKIP_REASON =
  "Go engine is sole execution authority (ENGINE_EXECUTION_AUTHORITY=1)";
