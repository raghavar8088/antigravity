import { NextResponse } from "next/server";
import { buildSignalImpactReport } from "@/lib/mockTradingExecutor/signalImpact";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";
export const maxDuration = 20;

export async function GET(req: Request): Promise<NextResponse> {
  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const symbol = url.searchParams.get("symbol")?.trim() || "BTCUSD";
  const testThreshold = url.searchParams.has("testThreshold")
    ? Number(url.searchParams.get("testThreshold"))
    : undefined;
  const testMinScore = url.searchParams.has("testMinScore")
    ? Number(url.searchParams.get("testMinScore"))
    : undefined;

  try {
    const impact = await buildSignalImpactReport({
      accountKey,
      symbol,
      testThreshold,
      testMinSignalScore: testMinScore,
    });
    return NextResponse.json({ ok: true, impact });
  } catch (err) {
    const message = err instanceof Error ? err.message : "signal impact failed";
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}
