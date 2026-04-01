"use client";

import { useEffect, useState } from "react";

export type AgentSignal = {
  role: "BULL" | "BEAR" | "MACRO";
  shouldTrade: boolean;
  confidence: number;
  thesis: string;
  sizeBtc: number;
  stopLossPct: number;
  takeProfitPct: number;
  error?: string;
};

export type RiskVerdict = {
  approved: boolean;
  approvedAction: "BUY" | "SELL" | "HOLD";
  vetoReason?: string;
  reasoning: string;
  adjustedSize: number;
  error?: string;
};

export type AIDecision = {
  id: string;
  timestamp: string;
  price: number;
  bullSignal: AgentSignal;
  bearSignal: AgentSignal;
  macroSignal: AgentSignal; // Gemini Flash top-down analyst
  riskVerdict: RiskVerdict;
  finalAction: "BUY" | "SELL" | "HOLD";
  finalReasoning: string;
  executed: boolean;
  regime: string;
};

type AIInsightsState = {
  enabled: boolean;
  geminiEnabled: boolean; // true when GEMINI_API_KEY is set
  message?: string;
  latest: AIDecision | null;
  recent: AIDecision[];
  loading: boolean;
};

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export default function useAIInsights(refreshKey = 0): AIInsightsState {
  const [state, setState] = useState<AIInsightsState>({
    enabled: false,
    geminiEnabled: false,
    latest: null,
    recent: [],
    loading: true,
  });

  useEffect(() => {
    let cancelled = false;

    const fetchInsights = async () => {
      try {
        const res = await fetch(`${API_URL}/api/ai/insights`);
        if (!res.ok) return;
        const data = await res.json();
        if (!cancelled) {
          setState({
            enabled: data.enabled ?? false,
            geminiEnabled: data.geminiEnabled ?? false,
            message: data.message,
            latest: data.latest ?? null,
            recent: data.recent ?? [],
            loading: false,
          });
        }
      } catch {
        if (!cancelled) setState((s) => ({ ...s, loading: false }));
      }
    };

    fetchInsights();
    const interval = setInterval(fetchInsights, 15_000); // refresh every 15s
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [refreshKey]);

  return state;
}
