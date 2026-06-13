import { NextResponse } from "next/server";
import { getAuthenticatedApiSession, type ApiSessionContext } from "@/lib/broker/apiSessionAuth";
import { isDeltaTestnetExecutionEnabled } from "./deltaConfig";
import { isDeskTestnetOpsPanelEnabled } from "./deltaTestnetSchemas";

export type TestnetRouteContext = ApiSessionContext;

export async function guardTestnetApiRoute(): Promise<
  | { ok: true; ctx: TestnetRouteContext }
  | { ok: false; response: NextResponse }
> {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth;

  if (!isDeltaTestnetExecutionEnabled()) {
    return {
      ok: false,
      response: NextResponse.json(
        {
          ok: false,
          error: "DELTA_TESTNET must be true or 1 (mainnet execution disabled)",
        },
        { status: 503 },
      ),
    };
  }

  return { ok: true, ctx: auth.ctx };
}

export function guardTestnetOpsPanelEnabled(): NextResponse | null {
  if (!isDeskTestnetOpsPanelEnabled()) {
    return NextResponse.json(
      { ok: false, error: "Testnet ops disabled (set NEXT_PUBLIC_DESK_TESTNET_OPS=1)" },
      { status: 403 },
    );
  }
  return null;
}
