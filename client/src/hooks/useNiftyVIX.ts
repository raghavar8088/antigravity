"use client";

import { useEffect, useState } from "react";

export default function useNiftyVIX() {
  const [vix, setVix] = useState(0);
  const [change, setChange] = useState(0);
  const [percentChange, setPercentChange] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const fetchVix = async () => {
      try {
        const res = await fetch("/api/nifty/vix");
        if (!res.ok) return;
        const data = await res.json() as {
          ok?: boolean;
          vix?: number;
          change?: number;
          percentChange?: number;
          error?: string;
        };
        if (data.ok) {
          setVix(Number(data.vix ?? 0));
          setChange(Number(data.change ?? 0));
          setPercentChange(Number(data.percentChange ?? 0));
          setError("");
        } else {
          setError(data.error ?? "VIX fetch failed");
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "unknown error");
      } finally {
        setLoading(false);
      }
    };

    fetchVix();
    const id = setInterval(fetchVix, 15000); // VIX changes slowly
    return () => clearInterval(id);
  }, []);

  return { vix, change, percentChange, loading, error };
}
