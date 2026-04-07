"use client";

import { useCallback, useEffect, useState } from "react";

import type { ChainData } from "@/hooks/useOptionChain";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export default function useNiftyOptionChain() {
  const [data, setData] = useState<ChainData | null>(null);
  const [selectedExpiry, setSelectedExpiry] = useState<string>("");
  const [loading, setLoading] = useState(true);

  const fetchChain = useCallback(async (expiry?: string) => {
    try {
      const qs = expiry ? `?expiry=${encodeURIComponent(expiry)}` : "";
      const res = await fetch(`${API_URL}/api/nifty-option-chain${qs}`);
      if (res.ok) {
        const json: ChainData = await res.json();
        setData(json);
        if (!selectedExpiry && json.selectedExpiry) {
          setSelectedExpiry(json.selectedExpiry);
        }
      }
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  }, [selectedExpiry]);

  useEffect(() => {
    fetchChain(selectedExpiry || undefined);
    const id = setInterval(() => fetchChain(selectedExpiry || undefined), 3000);
    return () => clearInterval(id);
  }, [fetchChain, selectedExpiry]);

  const selectExpiry = (value: string) => {
    setSelectedExpiry(value);
    fetchChain(value);
  };

  return { data, loading, selectedExpiry, selectExpiry };
}
