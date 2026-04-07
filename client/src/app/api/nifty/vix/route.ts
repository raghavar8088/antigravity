import { NextResponse } from "next/server";
import { getAngelJWT, isAngelConfigured, angelMissingEnv, commonHeaders, BASE_URL } from "@/lib/angelAuth";

function n(v: unknown): number {
  const num = Number(v);
  return Number.isFinite(num) ? num : 0;
}

export async function GET() {
  if (!isAngelConfigured()) {
    return NextResponse.json({
      ok: false,
      vix: 0,
      error: `Not configured — set these env vars: ${angelMissingEnv()}`,
    });
  }

  try {
    const jwt = await getAngelJWT();

    const res = await fetch(`${BASE_URL}/rest/secure/angelbroking/order/v1/getLtpData`, {
      method: "POST",
      headers: commonHeaders(jwt),
      body: JSON.stringify({ exchange: "NSE", tradingsymbol: "INDIA VIX", symboltoken: "1" }),
      next: { revalidate: 0 },
    });

    if (!res.ok) {
      return NextResponse.json({ ok: false, vix: 0, error: `Angel One returned ${res.status}` });
    }

    const payload = await res.json() as {
      status?: boolean;
      message?: string;
      errorcode?: string;
      data?: {
        ltp?: unknown;
        close?: unknown;
      };
    };

    if (!payload.status) {
      const msg = [payload.message, payload.errorcode].filter(Boolean).join(" ") || "VIX fetch failed";
      return NextResponse.json({ ok: false, vix: 0, error: msg });
    }

    const vix = n(payload.data?.ltp);
    const prevClose = n(payload.data?.close);
    const change = prevClose > 0 ? vix - prevClose : 0;
    const percentChange = prevClose > 0 ? (change / prevClose) * 100 : 0;
    const fetchedAt = new Date().toISOString();

    return NextResponse.json({ ok: true, vix, change, percentChange, fetched_at: fetchedAt });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, vix: 0, error: message });
  }
}
