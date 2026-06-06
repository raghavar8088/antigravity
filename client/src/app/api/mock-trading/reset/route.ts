import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { resetMockTradingState } from "@/lib/mockTradingMongo";
import { mockResetBodySchema } from "@/lib/mockTradingPersistenceTypes";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";

export const dynamic = "force-dynamic";

export async function DELETE(req: Request) {
  const accountKey = OWNER_ACCOUNT_KEY;

  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, code: "INVALID_JSON", error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = mockResetBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      {
        ok: false,
        code: "CONFIRMATION_REQUIRED",
        error: "Reset requires confirmation: RESET_MOCK_TRADING",
        details: parsed.error.flatten(),
      },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
  }

  try {
    const result = await resetMockTradingState(accountKey);
    return NextResponse.json({ ok: true, storage: "mongo", ...result });
  } catch (err) {
    return NextResponse.json(
      {
        ok: false,
        code: "MONGO_WRITE_FAILED",
        error: "Mongo write failed",
        detail: err instanceof Error ? err.message : "unknown",
      },
      { status: 500 },
    );
  }
}
