"use client";
import { useEffect, useState } from "react";
import { resolveEngineApiUrl } from "@/lib/broker/engineApi";

type EngineState = {
  engineOnline: boolean;
  balance: number | null;
  equity: number | null;
  unrealizedPnl: number | null;
  drawdownPct: number | null;
  loading: boolean;
};

export default function useEngineState(): EngineState {
  const [engineOnline, setEngineOnline] = useState(false);
  const [balance, setBalance] = useState<number | null>(null);
  const [equity, setEquity] = useState<number | null>(null);
  const [unrealizedPnl, setUnrealizedPnl] = useState<number | null>(null);
  const [drawdownPct, setDrawdownPct] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const apiUrl = resolveEngineApiUrl();
    let cancelled = false;

    const checkHealth = () => {
      fetch(`${apiUrl}/health`)
        .then((res) => {
          if (!cancelled) setEngineOnline(res.ok);
        })
        .catch(() => {
          if (!cancelled) setEngineOnline(false);
        });
    };

    const fetchState = () => {
      fetch("/api/paper-desk/state")
        .then((res) => (res.ok ? res.json() : null))
        .then((data) => {
          if (cancelled || !data?.ok) return;
          const s = data.state;
          if (!s) return;
          setBalance(typeof s.balance === "number" ? s.balance : null);
          setEquity(typeof s.equity === "number" ? s.equity : null);
          setUnrealizedPnl(typeof s.unrealized_pnl === "number" ? s.unrealized_pnl : null);
          setDrawdownPct(typeof s.current_drawdown === "number" ? s.current_drawdown : null);
        })
        .catch(() => {})
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    };

    checkHealth();
    fetchState();

    const healthInterval = setInterval(checkHealth, 5000);
    const stateInterval = setInterval(fetchState, 10000);

    return () => {
      cancelled = true;
      clearInterval(healthInterval);
      clearInterval(stateInterval);
    };
  }, []);

  return { engineOnline, balance, equity, unrealizedPnl, drawdownPct, loading };
}
