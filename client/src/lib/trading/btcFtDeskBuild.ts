/** Shown in BTC Future Trading tagline so you can confirm Vercel deployed the latest client. */
export const BTC_FT_DESK_BUILD =
  process.env.NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA?.slice(0, 7) ||
  process.env.NEXT_PUBLIC_BTC_FT_DESK_BUILD?.trim() ||
  "dev";

/**
 * The git commit SHA baked in at build time (7 chars). Used to detect stale
 * Vercel deployments: if the browser reports a different SHA than the server,
 * the operator needs to redeploy + restart the VPS worker.
 */
export const APP_BUILD_SHA = BTC_FT_DESK_BUILD;

/**
 * Returns true when `runtimeSha` doesn't match the build-time SHA, indicating
 * the Vercel deployment serving this page is not the latest commit.
 *
 * Pass the sha returned by `/api/health/desk-worker` or any server route that
 * embeds `process.env.NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA` at deploy time.
 */
export function isDeploymentStale(runtimeSha: string | null | undefined): boolean {
  if (!runtimeSha || APP_BUILD_SHA === "dev") return false;
  return runtimeSha.slice(0, 7) !== APP_BUILD_SHA.slice(0, 7);
}
