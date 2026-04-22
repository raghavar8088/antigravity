"use client";

import { useEffect, useState } from "react";
import { resolveEngineApiUrl } from "@/lib/engineApi";

type NiftyEngineMarketPoint = {
  time: number;
  price: number;
};

type NiftyEngineMarketResponse = {
  price?: number;
  series?: NiftyEngineMarketPoint[];
  updatedAt?: number;
};

type NiftyEngineMarketState = {
  price: number;
  barCount: number;
  updatedAt: number;
};

const EMPTY_STATE: NiftyEngineMarketState = {
  price: 0,
  barCount: 0,
  updatedAt: 0,
};

export default function useNiftyEngineMarket() {
  const [state, setState] = useState<NiftyEngineMarketState>(EMPTY_STATE);

  useEffect(() => {
    let cancelled = false;

    const fetchMarket = async () => {
      try {
        const response = await fetch(`${resolveEngineApiUrl()}/api/nifty-market`);
        if (!response.ok) {
          return;
        }

        const data = await response.json() as NiftyEngineMarketResponse;
        if (cancelled) {
          return;
        }

        setState({
          price: Number(data.price ?? 0),
          barCount: Array.isArray(data.series) ? data.series.length : 0,
          updatedAt: Number(data.updatedAt ?? 0),
        });
      } catch {
        // Keep the last good snapshot if the engine is temporarily unavailable.
      }
    };

    void fetchMarket();
    const interval = setInterval(fetchMarket, 3000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  return state;
}
