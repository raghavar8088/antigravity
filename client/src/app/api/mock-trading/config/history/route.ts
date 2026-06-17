import { NextResponse } from "next/server";
import { listExecutorConfigHistory } from "@/lib/mockTradingExecutor/executorConfig";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const limit = Number(url.searchParams.get("limit") ?? 10);

  try {
    const history = await listExecutorConfigHistory(accountKey, limit);
    return NextResponse.json({ ok: true, history });
  } catch (err) {
    const message = err instanceof Error ? err.message : "config history failed";
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}
