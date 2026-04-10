"use client";

import { useCallback, useEffect, useState } from "react";

import type { ChainData } from "@/hooks/useOptionChain";

export type NiftyChainData = ChainData & { source?: string };

export default function useNiftyOptionChain() {
  const [data, setData] = useState<NiftyChainData | null>(null);
  const [selectedExpiry, setSelectedExpiry] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [lastUpdatedAt, setLastUpdatedAt] = useState<number>(0);

  const fetchChain = useCallback(async (expiry?: string) => {
    setLoading(true);
    try {
      const qs = expiry ? `?expiry=${encodeURIComponent(expiry)}` : "";
      const res = await fetch(`/api/nifty/option-chain${qs}`);
      if (!res.ok) {
        setError(`Option chain request returned ${res.status}`);
        return;
      }
      if (res.ok) {
        const json: NiftyChainData = await res.json();
        // If the response indicates an error, don't update chain data
        if ((json as unknown as { ok?: boolean }).ok === false) {
          setError((json as unknown as { error?: string }).error ?? "Option chain returned an empty error response");
          return;
        }
        setData(json);
        setError("");
        setLastUpdatedAt(Date.now());
        if (!selectedExpiry && json.selectedExpiry) {
          setSelectedExpiry(json.selectedExpiry);
        }
      }
    } catch {
      setError("Could not reach the NIFTY option chain service");
    } finally {
      setLoading(false);
    }
  }, [selectedExpiry]);

  useEffect(() => {
    fetchChain(selectedExpiry || undefined);
    const id = setInterval(() => fetchChain(selectedExpiry || undefined), 30000);
    return () => clearInterval(id);
  }, [fetchChain, selectedExpiry]);

  const selectExpiry = (value: string) => {
    setSelectedExpiry(value);
    fetchChain(value);
  };

  return { data, loading, error, lastUpdatedAt, selectedExpiry, selectExpiry };
}
