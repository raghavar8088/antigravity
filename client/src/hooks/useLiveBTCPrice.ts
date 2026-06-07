"use client";
import { useState, useEffect, useRef } from "react";

interface LivePriceState {
  price: number;
  prevPrice: number;
  change24h: number;
  high24h: number;
  low24h: number;
  volume24h: number;
  ticksPerSecond: number;
  connected: boolean;
  recentPrices: { time: number; price: number }[];
}

const POLL_INTERVAL_MS = 3_000;

export default function useLiveBTCPrice(): LivePriceState {
  const [state, setState] = useState<LivePriceState>({
    price: 0,
    prevPrice: 0,
    change24h: 0,
    high24h: 0,
    low24h: 0,
    volume24h: 0,
    ticksPerSecond: 0,
    connected: false,
    recentPrices: [],
  });

  const tickCounter = useRef(0);
  const prevPriceRef = useRef(0);

  useEffect(() => {
    let cancelled = false;

    async function poll() {
      try {
        const res = await fetch("/api/btc/price", { cache: "no-store" });
        if (!res.ok || cancelled) return;
        const data = await res.json() as {
          ok: boolean;
          price?: number;
          change24h?: number;
          high24h?: number;
          low24h?: number;
          volume24h?: number;
        };
        if (!data.ok || !data.price || !Number.isFinite(data.price) || data.price <= 0) return;
        tickCounter.current++;
        const newPrice = data.price;
        setState((prev) => {
          prevPriceRef.current = prev.price || newPrice;
          return {
            price: newPrice,
            prevPrice: prevPriceRef.current,
            change24h: data.change24h ?? prev.change24h,
            high24h: data.high24h ?? prev.high24h,
            low24h: data.low24h ?? prev.low24h,
            volume24h: data.volume24h ?? prev.volume24h,
            ticksPerSecond: prev.ticksPerSecond,
            connected: true,
            recentPrices: [...prev.recentPrices, { time: Date.now(), price: newPrice }].slice(-240),
          };
        });
      } catch {
        if (!cancelled) setState((prev) => ({ ...prev, connected: false }));
      }
    }

    // Immediate first fetch, then poll every POLL_INTERVAL_MS
    poll();
    const interval = setInterval(poll, POLL_INTERVAL_MS);

    // ticks-per-second counter
    const tpsInterval = setInterval(() => {
      setState((prev) => ({ ...prev, ticksPerSecond: tickCounter.current }));
      tickCounter.current = 0;
    }, 1000);

    return () => {
      cancelled = true;
      clearInterval(interval);
      clearInterval(tpsInterval);
    };
  }, []);

  return state;
}
