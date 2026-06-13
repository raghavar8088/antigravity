import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/broker/getAuthenticatedApiSession";
import {
  dbRowToShadowIntentListItem,
  isDeskShadowIntentsEnabled,
  shadowIntentPostBodyToInsertRow,
} from "@/lib/ai/shadowTradeIntentMapper";
import {
  shadowIntentPostBodySchema,
  type ShadowTradeIntentDbRow,
} from "@/lib/ai/shadowTradeIntentTypes";
import { computeWouldPlaceTestnetShadow } from "@/server/delta/shadowWouldPlaceTestnet";
import { createServiceSupabase } from "@/lib/supabase/server";
import { z } from "zod";

export const dynamic = "force-dynamic";

const listQuerySchema = z.object({
  limit: z.coerce.number().int().min(1).max(50).default(20),
});

function shadowApiDisabled() {
  return NextResponse.json(
    { ok: false, error: "Shadow intents disabled (NEXT_PUBLIC_DESK_SHADOW_INTENTS=1)" },
    { status: 403 },
  );
}

export async function POST(req: Request) {
  if (!isDeskShadowIntentsEnabled()) return shadowApiDisabled();

  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = shadowIntentPostBodySchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, error: "Validation failed", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  if (parsed.data.intentKind === "close" && (parsed.data.exitPrice == null || !parsed.data.exitReason)) {
    return NextResponse.json(
      { ok: false, error: "close intents require exitPrice and exitReason" },
      { status: 400 },
    );
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "Supabase client unavailable" }, { status: 503 });
  }

  const wouldPlaceTestnet = computeWouldPlaceTestnetShadow();
  const row = shadowIntentPostBodyToInsertRow(auth.ctx.userId, parsed.data, wouldPlaceTestnet);

  const { error } = await supabase.from("shadow_trade_intents").upsert(row, {
    onConflict: "client_intent_id",
    ignoreDuplicates: true,
  });

  if (error) {
    console.error("[shadow-trade-intents] insert", error);
    return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
  }

  return NextResponse.json({
    ok: true,
    clientIntentId: row.client_intent_id,
    wouldPlaceTestnet,
  });
}

export async function GET(req: Request) {
  if (!isDeskShadowIntentsEnabled()) return shadowApiDisabled();

  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  const url = new URL(req.url);
  const parsed = listQuerySchema.safeParse({ limit: url.searchParams.get("limit") ?? undefined });
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "Supabase client unavailable" }, { status: 503 });
  }

  const { data, error } = await supabase
    .from("shadow_trade_intents")
    .select("*")
    .eq("user_id", auth.ctx.userId)
    .order("created_at", { ascending: false })
    .limit(parsed.data.limit);

  if (error) {
    console.error("[shadow-trade-intents] list", error);
    return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
  }

  const intents = ((data ?? []) as ShadowTradeIntentDbRow[]).map(dbRowToShadowIntentListItem);

  return NextResponse.json({
    ok: true,
    intents,
  });
}
