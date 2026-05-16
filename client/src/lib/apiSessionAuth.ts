import { NextResponse } from "next/server";
import { requireAuthenticatedPaperUser } from "@/lib/paperTradesAuth";
import { createServerSupabase } from "@/lib/supabase/server";
import { isSupabaseAuthConfigured } from "@/lib/supabase/client";

export type ApiSessionContext = {
  userId: string;
};

/** Supabase session only (no service role required). For operator/server routes like testnet ping. */
export async function getAuthenticatedApiSession(): Promise<
  | { ok: true; ctx: ApiSessionContext }
  | { ok: false; response: NextResponse }
> {
  if (!isSupabaseAuthConfigured()) {
    return {
      ok: false,
      response: NextResponse.json(
        {
          ok: false,
          error: "Supabase auth not configured (NEXT_PUBLIC_SUPABASE_URL + NEXT_PUBLIC_SUPABASE_ANON_KEY)",
        },
        { status: 503 },
      ),
    };
  }

  try {
    const supabase = await createServerSupabase();
    const {
      data: { user },
      error,
    } = await supabase.auth.getUser();
    if (error || !user?.id) {
      return {
        ok: false,
        response: NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 }),
      };
    }
    const guard = requireAuthenticatedPaperUser(user.id);
    if (!guard.ok) {
      return {
        ok: false,
        response: NextResponse.json({ ok: false, error: guard.error }, { status: guard.status }),
      };
    }
    return { ok: true, ctx: { userId: guard.userId } };
  } catch {
    return {
      ok: false,
      response: NextResponse.json({ ok: false, error: "Auth unavailable" }, { status: 503 }),
    };
  }
}
