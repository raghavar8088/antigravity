"use client";
import { useEffect, useState } from "react";

const FALLBACK_BALANCE = 1000000.0;
function resolveEngineApiUrl() {
  if (process.env.NEXT_PUBLIC_API_URL) return process.env.NEXT_PUBLIC_API_URL;
  if (typeof window !== "undefined") {
    const host = window.location.hostname;
    if (host && host !== "localhost" && host !== "127.0.0.1") {
      const port = process.env.NEXT_PUBLIC_ENGINE_PORT || "8080";
      return `${window.location.protocol}//${host}:${port}`;
    }
  }
  return "http://localhost:8080";
}

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
