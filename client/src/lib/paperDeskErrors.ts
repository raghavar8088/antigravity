/**
 * paperDeskErrors.ts — Consistent error responses for Paper Desk API routes.
 *
 * Every Paper Desk route imports from here so error shapes are uniform and
 * the frontend can pattern-match on `code` without parsing strings.
 */

import { NextResponse } from "next/server";

export type PaperDeskErrorCode =
  | "AUTH_UNCONFIGURED"   // AUTH_JWT_SECRET / ADMIN vars missing
  | "UNAUTHORIZED"        // no valid session cookie
  | "MONGO_UNCONFIGURED"  // MONGODB_URI missing or wrong scheme
  | "MONGO_UNAVAILABLE"   // URI set but connection/query failed
  | "NO_DATA"             // connected OK but collection is empty for this account
  | "INTERNAL";           // unexpected server error

export function mongoUnconfigured() {
  return NextResponse.json(
    {
      ok: false,
      code: "MONGO_UNCONFIGURED" as PaperDeskErrorCode,
      error: "MongoDB not configured",
      hint: "Set MONGODB_URI=mongodb+srv://... in .env.local and Vercel environment variables.",
    },
    { status: 503 },
  );
}

export function mongoUnavailable(detail: string) {
  return NextResponse.json(
    {
      ok: false,
      code: "MONGO_UNAVAILABLE" as PaperDeskErrorCode,
      error: "MongoDB query failed",
      detail,
      hint: "Check Atlas cluster status, IP whitelist (0.0.0.0/0 or Vercel/Lightsail egress IPs), and connection string credentials.",
    },
    { status: 503 },
  );
}

export function internalError(detail: string) {
  return NextResponse.json(
    {
      ok: false,
      code: "INTERNAL" as PaperDeskErrorCode,
      error: "Unexpected server error",
      detail,
    },
    { status: 500 },
  );
}
