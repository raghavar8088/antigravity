import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getPaperState } from "@/lib/paperDeskClient";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) return mongoUnconfigured();

  let state;
  try {
    state = await getPaperState(accountKey);
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }

  return NextResponse.json({
    ok: true,
    account_key: accountKey,
    state,
    engine_writing: state !== null,
  });
}
