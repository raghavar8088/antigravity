import { NextResponse } from "next/server";
import {
  getExecutorConfigView,
  saveExecutorThresholdConfig,
} from "@/lib/mockTradingExecutor/executorConfig";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;

  try {
    const config = await getExecutorConfigView(accountKey);
    return NextResponse.json({ ok: true, config });
  } catch (err) {
    const message = err instanceof Error ? err.message : "config fetch failed";
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}

export async function POST(req: Request): Promise<NextResponse> {
  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;

  try {
    const body = (await req.json()) as {
      signalThreshold?: number;
      minSignalScore?: number;
      reason?: string;
    };

    if (!Number.isFinite(body.signalThreshold) || !Number.isFinite(body.minSignalScore)) {
      return NextResponse.json(
        { ok: false, error: "signalThreshold and minSignalScore are required" },
        { status: 400 },
      );
    }

    const userId = req.headers.get("x-user-email") ?? req.headers.get("user-email") ?? "ui";
    const result = await saveExecutorThresholdConfig({
      accountKey,
      signalThreshold: body.signalThreshold!,
      minSignalScore: body.minSignalScore!,
      reason: body.reason,
      userId,
    });

    return NextResponse.json({
      ok: true,
      success: true,
      config: result.config,
      changeId: result.changeId,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "config save failed";
    if (message.includes("must be")) {
      return NextResponse.json({ ok: false, error: message }, { status: 400 });
    }
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}
