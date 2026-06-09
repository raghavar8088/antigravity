import { NextResponse } from "next/server";

/** Standard 410 response for retired direct-broker API routes. */
export function blockedDirectExecutionRoute(route: string) {
  return NextResponse.json(
    {
      ok: false,
      code: "EXECUTION_ROUTE_RETIRED",
      error: `Direct broker execution via ${route} is disabled.`,
      hint: "Submit an execution request via POST /api/execution/request — all orders flow through the institutional gateway.",
    },
    { status: 410 },
  );
}
