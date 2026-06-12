import { NextResponse } from "next/server";
import { fetchBtcSpotPrice } from "@/lib/btcSpotPrice";

export const dynamic = "force-dynamic";

export async function GET() {
  const spot = await fetchBtcSpotPrice();
  if (!spot) {
    return NextResponse.json({ ok: false, error: "All price sources failed" }, { status: 503 });
  }

  const { source, ...payload } = spot;
  return NextResponse.json(
    { ok: true, source, ...payload },
    { headers: { "Cache-Control": "no-store, max-age=0" } },
  );
}
