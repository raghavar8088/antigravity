import { NextResponse } from "next/server";
import { isMongoConfigured, getResearchState, upsertResearchState } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim();
  const namespace = url.searchParams.get("namespace")?.trim();
  if (!accountKey || !namespace) {
    return NextResponse.json({ ok: false, error: "account_key and namespace required" }, { status: 400 });
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }
  const doc = await getResearchState(accountKey, namespace);
  return NextResponse.json({
    ok: true,
    winners: doc?.winners ?? [],
    retiredIds: doc?.retired_ids ?? [],
  });
}

export async function POST(req: Request) {
  let body: unknown;
  try { body = await req.json(); } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }
  const b = body as Record<string, unknown>;
  const accountKey = typeof b.accountKey === "string" ? b.accountKey.trim() : null;
  const namespace = typeof b.namespace === "string" ? b.namespace.trim() : null;
  if (!accountKey || !namespace) {
    return NextResponse.json({ ok: false, error: "accountKey and namespace required" }, { status: 400 });
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }
  await upsertResearchState({
    account_key: accountKey,
    namespace,
    winners: Array.isArray(b.winners) ? (b.winners as number[]) : [],
    retired_ids: Array.isArray(b.retiredIds) ? (b.retiredIds as number[]) : [],
    updated_at: new Date().toISOString(),
  });
  return NextResponse.json({ ok: true });
}
