"use client";

import { useCallback, useEffect, useState } from "react";

import type { ChainData } from "@/hooks/useOptionChain";

export type NiftyChainData = ChainData & { source?: string };

export default function useNiftyOptionChain() {
  const [data, setData] = useState<NiftyChainData | null>(null);
  const [selectedExpiry, setSelectedExpiry] = useState<string>("");
  const [loading, setLoading] = useState(true);

  const fetchChain = useCallback(async (expiry?: string) => {
    try {
      const qs = expiry ? `?expiry=${encodeURIComponent(expiry)}` : "";
      const res = await fetch(`/api/nifty/option-chain${qs}`);
      if (res.ok) {
        const json: NiftyChainData = await res.json();
        // If the response indicates an error, don't update chain data
        if ((json as unknown as { ok?: boolean }).ok === false) {
          return;
        }
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
    const id = setInterval(() => fetchChain(selectedExpiry || undefined), 30000);
    return () => clearInterval(id);
  }, [fetchChain, selectedExpiry]);

  const selectExpiry = (value: string) => {
    setSelectedExpiry(value);
    fetchChain(value);
  };

  return { data, loading, selectedExpiry, selectExpiry };
}
