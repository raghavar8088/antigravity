import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import {
  getMigrationStatus,
  migrateAllToGrade5,
} from "@/lib/strategyAuthority/strategyAuthorityMongo";
import { TOTAL_STRATEGIES } from "@/lib/strategyAuthority/strategyCatalog";

export const dynamic = "force-dynamic";

export async function GET(): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const status = await getMigrationStatus();
    return NextResponse.json({ ok: true, ...status, totalCatalog: TOTAL_STRATEGIES });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "STATUS_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}

export async function POST(): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const result = await migrateAllToGrade5();
    return NextResponse.json({
      ok: true,
      message: `Migration complete. Inserted: ${result.inserted}, Already existed: ${result.alreadyExisted}`,
      ...result,
    });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "MIGRATION_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
