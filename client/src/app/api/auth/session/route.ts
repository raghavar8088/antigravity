import { NextResponse } from "next/server";
import { createServerSupabase } from "@/lib/supabase/server";
import { isSupabaseAuthConfigured } from "@/lib/supabase/client";

export const dynamic = "force-dynamic";

export async function GET() {
  if (!isSupabaseAuthConfigured()) {
    return NextResponse.json({ ok: false, error: "Supabase auth not configured" }, { status: 503 });
  }

  try {
    const supabase = await createServerSupabase();
    const {
      data: { user },
    } = await supabase.auth.getUser();
    if (!user) {
      return NextResponse.json({ ok: true, user: null });
    }
    return NextResponse.json({
      ok: true,
      user: { id: user.id, email: user.email ?? null },
    });
  } catch {
    return NextResponse.json({ ok: false, error: "Auth unavailable" }, { status: 503 });
  }
}
