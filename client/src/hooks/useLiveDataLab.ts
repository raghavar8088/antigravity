"use client";

import { useCallback, useEffect, useState } from "react";

// Probe endpoints are Next.js API routes — they run as Vercel serverless functions
// and are available at /api/probe/* on both local dev and production Vercel.
const DELTA_PROBE_URL = "/api/probe/delta-btc";
const ANGEL_PROBE_URL = "/api/probe/angelone-nifty";

export type DeltaProbeData = {
  source: string;
  symbol: string;
  price: number;
  open: number;
  high: number;
  low: number;
  mark_price: number;
  spot_price: number;
  volume: number;
  contract_type: string;
  exchange_time: string;
  fetched_at: string;
  ok: boolean;
  error: string;
};

export type AngelOneProbeData = {
  source: string;
  exchange: string;
  symbol: string;
  symbol_token: string;
  price: number;
  open: number;
  high: number;
  low: number;
  close: number;
  change: number;
  percent_change: number;
  exchange_time: string;
  fetched_at: string;
  configured: boolean;
  ok: boolean;
  error: string;
};

export type LiveDataLabState = {
  deltaData: DeltaProbeData | null;
  angelData: AngelOneProbeData | null;
  deltaLoading: boolean;
  angelLoading: boolean;
  deltaError: string;
  angelError: string;
  refresh: () => void;
};

export default function useLiveDataLab(): LiveDataLabState {
  const [deltaData, setDeltaData] = useState<DeltaProbeData | null>(null);
  const [angelData, setAngelData] = useState<AngelOneProbeData | null>(null);
  const [deltaLoading, setDeltaLoading] = useState(true);
  const [angelLoading, setAngelLoading] = useState(true);
  const [deltaError, setDeltaError] = useState("");
  const [angelError, setAngelError] = useState("");
  const [refreshCounter, setRefreshCounter] = useState(0);

  const refresh = useCallback(() => {
    setRefreshCounter((c) => c + 1);
  }, []);

  useEffect(() => {
    let cancelled = false;

    const fetchDelta = async () => {
      setDeltaLoading(true);
      try {
        const response = await fetch(DELTA_PROBE_URL);
        if (!response.ok) {
          setDeltaError(`HTTP ${response.status}`);
          return;
        }
        const data = await response.json() as DeltaProbeData;
        if (cancelled) return;
        setDeltaData(data);
        setDeltaError(data.ok ? "" : data.error);
      } catch (err) {
        if (cancelled) return;
        setDeltaError(err instanceof Error ? err.message : "fetch failed");
      } finally {
        if (!cancelled) setDeltaLoading(false);
      }
    };

    const fetchAngel = async () => {
      setAngelLoading(true);
      try {
        const response = await fetch(ANGEL_PROBE_URL);
        if (!response.ok) {
          setAngelError(`HTTP ${response.status}`);
          return;
        }
        const data = await response.json() as AngelOneProbeData;
        if (cancelled) return;
        setAngelData(data);
        setAngelError(data.ok ? "" : data.error);
      } catch (err) {
        if (cancelled) return;
        setAngelError(err instanceof Error ? err.message : "fetch failed");
      } finally {
        if (!cancelled) setAngelLoading(false);
      }
    };

    fetchDelta();
    fetchAngel();

    const deltaInterval = window.setInterval(fetchDelta, 10000);
    const angelInterval = window.setInterval(fetchAngel, 10000);

    return () => {
      cancelled = true;
      window.clearInterval(deltaInterval);
      window.clearInterval(angelInterval);
    };
  }, [refreshCounter]);

  return { deltaData, angelData, deltaLoading, angelLoading, deltaError, angelError, refresh };
}
