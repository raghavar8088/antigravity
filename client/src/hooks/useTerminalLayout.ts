"use client";

import { useState, useEffect } from "react";

export type LayoutConfig = {
  id: string;
  name: string;
  panels: Array<{
    id: string;
    visible: boolean;
    gridColumn: string;
    gridRow?: string;
  }>;
};

const DEFAULT_LAYOUT: LayoutConfig = {
  id: "default",
  name: "Institutional Default",
  panels: [
    { id: "account", visible: true, gridColumn: "span 4" },
    { id: "ticker", visible: true, gridColumn: "span 4" },
    { id: "regime", visible: true, gridColumn: "span 4" },
    { id: "research", visible: true, gridColumn: "span 6" },
    { id: "pine-editor", visible: true, gridColumn: "span 6" },
    { id: "readiness", visible: true, gridColumn: "span 6" },
    { id: "diagnostics", visible: true, gridColumn: "span 6" },
    { id: "lab", visible: true, gridColumn: "span 6" },
    { id: "charts", visible: true, gridColumn: "span 12" },
    { id: "analytics", visible: true, gridColumn: "span 6" },
    { id: "leaderboard", visible: true, gridColumn: "span 6" },
    { id: "config", visible: true, gridColumn: "span 12" },
    { id: "history", visible: true, gridColumn: "span 12" },
    { id: "rollups", visible: true, gridColumn: "span 12" },
    { id: "logs", visible: true, gridColumn: "span 12" },
  ],
};

export function useTerminalLayout(storageKey: string = "terminal-layout") {
  const [layout, setLayout] = useState<LayoutConfig>(DEFAULT_LAYOUT);
  const [isHydrated, setIsHydrated] = useState(false);

  useEffect(() => {
    const saved = localStorage.getItem(storageKey);
    if (saved) {
      try {
        setLayout(JSON.parse(saved));
      } catch (e) {
        console.error("Failed to parse saved layout", e);
      }
    }
    setIsHydrated(true);
  }, [storageKey]);

  const saveLayout = (newLayout: LayoutConfig) => {
    setLayout(newLayout);
    localStorage.setItem(storageKey, JSON.stringify(newLayout));
  };

  const resetLayout = () => {
    saveLayout(DEFAULT_LAYOUT);
  };

  return { layout, saveLayout, resetLayout, isHydrated };
}
