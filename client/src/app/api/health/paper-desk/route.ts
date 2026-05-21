import { NextResponse } from "next/server";
import { isMongoConfigured, pingMongo } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

export async function GET() {
  const mongoConfigured = isMongoConfigured();
  let mongoPingOk = false;
  if (mongoConfigured) {
    mongoPingOk = await pingMongo();
  }
  return NextResponse.json({
    ok: true,
    mongoConfigured,
    mongoPingOk,
    supabaseConfigured: false,
    lastPostError: null,
  });
}
