import { NextResponse } from "next/server";
import { isMongoConfigured, pingMongo } from "@/lib/broker/mongoTradesClient";

export const dynamic = "force-dynamic";

const TRADES_COLLECTION = "paper_trades";

function supabaseConfigured(): boolean {
  return (
    typeof process.env.NEXT_PUBLIC_SUPABASE_URL === "string" &&
    process.env.NEXT_PUBLIC_SUPABASE_URL.trim().length > 0 &&
    typeof process.env.SUPABASE_SERVICE_ROLE_KEY === "string" &&
    process.env.SUPABASE_SERVICE_ROLE_KEY.trim().length > 0
  );
}

export async function GET() {
  const mongoConfigured = isMongoConfigured();
  let pingOk = false;
  if (mongoConfigured) {
    pingOk = await pingMongo();
  }
  return NextResponse.json({
    mongo: {
      configured: mongoConfigured,
      pingOk,
      db: process.env.MONGODB_DB?.trim() || "loop_trades",
      collection: TRADES_COLLECTION,
    },
    supabase: { configured: supabaseConfigured() },
    serverTime: new Date().toISOString(),
  });
}
