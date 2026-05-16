import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/apiSessionAuth";
import { DeltaClientError } from "@/server/delta/deltaErrors";
import { DeltaTestnetClient, balanceSnippetFromWallets } from "@/server/delta/deltaClient";
import { isDeltaTestnetExecutionEnabled, resolveDeltaTestnetBaseUrl } from "@/server/delta/deltaConfig";

export const dynamic = "force-dynamic";

/**
 * Operator verify: signed-in user + testnet Delta credentials → wallet snippet.
 * Does not place orders. No keys in browser.
 */
export async function POST() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  if (!isDeltaTestnetExecutionEnabled()) {
    return NextResponse.json(
      {
        ok: false,
        error: "DELTA_TESTNET must be true or 1 (mainnet execution disabled)",
      },
      { status: 503 },
    );
  }

  try {
    const client = DeltaTestnetClient.fromEnv();
    const balances = await client.getBalances();
    if (!balances.ok) {
      return NextResponse.json(
        {
          ok: false,
          error: balances.error,
          status: balances.status,
        },
        { status: balances.status >= 400 ? balances.status : 502 },
      );
    }

    return NextResponse.json({
      ok: true,
      testnet: true,
      baseUrl: resolveDeltaTestnetBaseUrl(),
      balanceSnippet: balanceSnippetFromWallets(balances.data),
    });
  } catch (e) {
    const message = e instanceof DeltaClientError ? e.message : "Delta testnet ping failed";
    return NextResponse.json({ ok: false, error: message }, { status: 503 });
  }
}
