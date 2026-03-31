"use client";
import { useEffect, useState } from "react";

const FALLBACK_BALANCE = 100000.0;

export default function useEngineState() {
  const [engineOnline, setEngineOnline] = useState(false);

  useEffect(() => {
    const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    let cancelled = false;

    const checkHealth = () => {
      fetch(`${API_URL}/health`)
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
