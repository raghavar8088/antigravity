"use client";
import { useEffect, useState } from "react";
import { resolveEngineApiUrl } from "@/lib/engineApi";

const FALLBACK_BALANCE = 1000000.0;

export default function useEngineState() {
  const [engineOnline, setEngineOnline] = useState(false);

  useEffect(() => {
    const apiUrl = resolveEngineApiUrl();
    let cancelled = false;

    const checkHealth = () => {
      fetch(`${apiUrl}/health`)
        .then((res) => {
          if (!cancelled) {
            setEngineOnline(res.ok);
          }
        })
        .catch(() => {
          if (!cancelled) {
            setEngineOnline(false);
          }
        });
    };

    checkHealth();
    const interval = setInterval(checkHealth, 5000);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  return {
    engineOnline,
    balance: FALLBACK_BALANCE,
  };
}
