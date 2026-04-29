import { NextResponse } from "next/server";
import { isEngineProxyConfigured, engineProxyFetch } from "@/lib/engineProxy";

export async function GET(request: Request): Promise<Response> {
  if (!isEngineProxyConfigured()) {
    return NextResponse.json({
      ok: false,
      error: "Not configured — set LIGHTSAIL_ENGINE_URL for engine proxy (MCX debug search).",
    });
  }

  const { searchParams } = new URL(request.url);
  const query = searchParams.get("q") ?? searchParams.get("commodity") ?? "CRUDEOIL";

  try {
    const res = await engineProxyFetch("/rest/secure/angelbroking/order/v1/searchScrip", {
      exchange: "MCX",
      searchscrip: query,
    });

    const raw = await res.json();
    return NextResponse.json({
      ok: true,
      query,
      jwtObtained: true,
      httpStatus: res.status,
      rawResponse: raw,
    });
  } catch (err) {
    return NextResponse.json({ ok: false, error: err instanceof Error ? err.message : String(err) });
  }
}
