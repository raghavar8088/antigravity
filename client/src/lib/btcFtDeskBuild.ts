/** Shown in BTC Future Trading tagline so you can confirm Vercel deployed the latest client. */
export const BTC_FT_DESK_BUILD =
  process.env.NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA?.slice(0, 7) ||
  process.env.NEXT_PUBLIC_BTC_FT_DESK_BUILD?.trim() ||
  "dev";
