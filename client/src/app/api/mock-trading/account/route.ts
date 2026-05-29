import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { insertMockAccountSnapshot } from "@/lib/mockTradingMongo";
import { mockAccountSnapshotBodySchema } from "@/lib/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";

export async function POST(req: Request) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, code: "INVALID_JSON", error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = mockAccountSnapshotBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" }, { status: 503 });
  }

  try {
    const snapshot = await insertMockAccountSnapshot(parsed.data.accountKey, parsed.data.account, parsed.data.config);
    return NextResponse.json({
      ok: true,
      storage: "mongo",
      timestamp: snapshot.timestamp,
    });
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
