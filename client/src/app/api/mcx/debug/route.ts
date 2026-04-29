import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";

export async function GET(): Promise<Response> {
  if (!isAngelConfigured()) {
    return NextResponse.json({ ok: false, error: `Not configured: ${angelMissingEnv()}` });
  }

  try {
    const jwt = await getAngelJWT();

    // Test one commodity search and return raw response
    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/order/v1/searchScrip`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({ exchange: "MCX", searchscrip: "CRUDEOIL" }),
      next: { revalidate: 0 },
    });

    const raw = await res.json();
    return NextResponse.json({
      ok: true,
      jwtObtained: true,
      httpStatus: res.status,
      rawResponse: raw,
    });
  } catch (err) {
    return NextResponse.json({ ok: false, error: err instanceof Error ? err.message : String(err) });
  }
}
