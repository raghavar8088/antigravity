"use client";
import { useState, useEffect, useCallback } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export type ChainLeg = {
  iv: number;
  delta: number;
  gamma: number;
  theta: number;
  vega: number;
  mark: number;
  bid: number;
  ask: number;
  oi: number;
  volume: number;
  isItm: boolean;
};

export type ChainRow = {
  strike: number;
  isAtm: boolean;
  moneynessPC: number;
  call: ChainLeg;
  put: ChainLeg;
};

export type ExpiryMeta = {
  label: string;
  value: string;
  dte: number;
};

export type ChainData = {
  underlyingPrice: number;
  baseIv: number;
  expiries: ExpiryMeta[];
  selectedExpiry: string;
  expiryLabel: string;
  dte: number;
  chain: ChainRow[];
};

export default function useOptionChain() {
  const [data, setData] = useState<ChainData | null>(null);
  const [selectedExpiry, setSelectedExpiry] = useState<string>("");
  const [loading, setLoading] = useState(true);

  const fetchChain = useCallback(async (expiry?: string) => {
    try {
      const qs = expiry ? `?expiry=${encodeURIComponent(expiry)}` : "";
      const res = await fetch(`${API_URL}/api/option-chain${qs}`);
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

  // Auto-refresh every 3 seconds
  useEffect(() => {
    fetchChain(selectedExpiry || undefined);
    const id = setInterval(() => fetchChain(selectedExpiry || undefined), 3000);
    return () => clearInterval(id);
  }, [selectedExpiry, fetchChain]);

  const selectExpiry = (val: string) => {
    setSelectedExpiry(val);
    fetchChain(val);
  };

  return { data, loading, selectedExpiry, selectExpiry };
}
