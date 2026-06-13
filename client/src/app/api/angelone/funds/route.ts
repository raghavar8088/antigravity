import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/broker/angelAuth";
import { getAuthenticatedApiSession } from "@/lib/broker/getAuthenticatedApiSession";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  if (!isAngelConfigured()) {
    return NextResponse.json({
      ok: false,
      error: `Not configured — set these env vars: ${angelMissingEnv()}`,
    });
  }

  try {
    const jwt = await getAngelJWT();

    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/user/v1/getRMS`, {
      method: "GET",
      headers: commonHeaders(jwt),
      next: { revalidate: 0 },
    });

    if (!res.ok) {
      return NextResponse.json({ ok: false, error: `Angel One returned ${res.status}` });
    }

    const payload = await res.json() as {
      status?: boolean;
      message?: string;
      errorcode?: string;
      data?: {
        availablecash?: string | number;
        utilisedmargin?: string | number;
        availablemargin?: string | number;
        collateral?: string | number;
        m2munrealized?: string | number;
        m2mrealized?: string | number;
        net?: string | number;
      } | null;
    };

    if (!payload.status || !payload.data) {
      const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || "Funds fetch failed";
      return NextResponse.json({ ok: false, error: msg });
    }

    const d = payload.data;
    const availableCash = Number(d.availablecash) || 0;
    const usedMargin = Number(d.utilisedmargin) || 0;
    const total = availableCash + usedMargin;

    return NextResponse.json({
      ok: true,
      available_cash: availableCash,
      used_margin: usedMargin,
      total,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, error: message });
  }
}
