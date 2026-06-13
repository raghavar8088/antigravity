import { NextResponse } from "next/server";
import { isEngineProxyConfigured, engineProxyFetch } from "@/lib/broker/engineProxy";
import { resolveNiftyBeesToken } from "@/lib/trading/niftyBeesTokenResolver";

function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

function toISTString(date: Date): string {
  const istOffset = 5 * 60 + 30;
  const istMs = date.getTime() + istOffset * 60 * 1000;
  const d = new Date(istMs);
  return `${d.getUTCFullYear()}-${pad2(d.getUTCMonth() + 1)}-${pad2(d.getUTCDate())} ${pad2(d.getUTCHours())}:${pad2(d.getUTCMinutes())}`;
}

export type NiftyBeesCandle = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

/** NSE cash session Mon–Fri 09:15–15:30 IST */
export async function GET(request: Request) {
  if (!isEngineProxyConfigured()) {
    return NextResponse.json({
      ok: false,
      candles: [] as NiftyBeesCandle[],
      count: 0,
      error:
        "Not configured — set LIGHTSAIL_ENGINE_URL so Angel One historical candles use the whitelisted IP.",
    });
  }

  const { searchParams } = new URL(request.url);
  const interval = searchParams.get("interval") ?? "ONE_MINUTE";

  try {
    const resolved = await resolveNiftyBeesToken();
    if (!resolved) {
      return NextResponse.json({
        ok: false,
        candles: [],
        count: 0,
        error: "Could not resolve NIFTYBEES token via Angel One searchScrip.",
      });
    }

    const now = new Date();
    const istOffset = (5 * 60 + 30) * 60 * 1000;
    const nowIST = new Date(now.getTime() + istOffset);

    let sessionDayUTC = new Date(Date.UTC(nowIST.getUTCFullYear(), nowIST.getUTCMonth(), nowIST.getUTCDate()));

    const minutesSinceISTMidnight = nowIST.getUTCHours() * 60 + nowIST.getUTCMinutes();
    const sessionOpenMin = 9 * 60 + 15;
    const sessionCloseMin = 15 * 60 + 30;

    if (minutesSinceISTMidnight < sessionOpenMin) {
      sessionDayUTC = new Date(sessionDayUTC.getTime() - 24 * 60 * 60 * 1000);
    }

    let day = nowIST.getUTCDay();
    if (minutesSinceISTMidnight < sessionOpenMin) {
      day = day === 0 ? 6 : day - 1;
    }
    if (day === 0) {
      sessionDayUTC = new Date(sessionDayUTC.getTime() - 2 * 24 * 60 * 60 * 1000);
    } else if (day === 6) {
      sessionDayUTC = new Date(sessionDayUTC.getTime() - 1 * 24 * 60 * 60 * 1000);
    }

    const fromUTC = new Date(sessionDayUTC.getTime() + (3 * 60 + 45) * 60 * 1000);
    const sessionCloseUTC = new Date(sessionDayUTC.getTime() + 10 * 60 * 60 * 1000);

    const isLiveSession =
      minutesSinceISTMidnight >= sessionOpenMin && minutesSinceISTMidnight < sessionCloseMin;
    const toUTC = isLiveSession && now < sessionCloseUTC ? now : sessionCloseUTC;

    const fromdate = toISTString(fromUTC);
    const todate = toISTString(toUTC);

    const res = await engineProxyFetch("/rest/secure/angelbroking/historical/v1/getCandleData", {
      exchange: "NSE",
      symboltoken: resolved.token,
      interval,
      fromdate,
      todate,
    });

    if (!res.ok) {
      return NextResponse.json({
        ok: false,
        candles: [],
        count: 0,
        error: `Angel One returned ${res.status}`,
      });
    }

    const payload = await res.json() as { status?: boolean; data?: unknown[][] };
    if (!payload.status || !Array.isArray(payload.data)) {
      return NextResponse.json({ ok: false, candles: [], count: 0, error: "Candle fetch failed" });
    }

    const candles: NiftyBeesCandle[] = payload.data.map((row) => ({
      time: new Date(row[0] as string).getTime(),
      open: Number(row[1]) || 0,
      high: Number(row[2]) || 0,
      low: Number(row[3]) || 0,
      close: Number(row[4]) || 0,
      volume: Number(row[5]) || 0,
    }));

    return NextResponse.json({
      ok: true,
      candles,
      count: candles.length,
      token: resolved.token,
      tradingSymbol: resolved.tradingSymbol,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown error";
    return NextResponse.json({ ok: false, candles: [], count: 0, error: message });
  }
}
