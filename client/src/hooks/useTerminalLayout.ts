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

function normalizeLayout(raw: unknown): LayoutConfig {
  if (!raw || typeof raw !== "object") return DEFAULT_LAYOUT;
  const candidate = raw as Partial<LayoutConfig>;
  if (!Array.isArray(candidate.panels)) return DEFAULT_LAYOUT;
  const panels = candidate.panels
    .filter((panel): panel is LayoutConfig["panels"][number] => {
      return !!panel
        && typeof panel === "object"
        && typeof panel.id === "string"
        && typeof panel.visible === "boolean"
        && typeof panel.gridColumn === "string";
    });
  if (panels.length === 0) return DEFAULT_LAYOUT;
  return {
    id: typeof candidate.id === "string" ? candidate.id : DEFAULT_LAYOUT.id,
    name: typeof candidate.name === "string" ? candidate.name : DEFAULT_LAYOUT.name,
    panels,
  };
}

export function useTerminalLayout(storageKey: string = "terminal-layout") {
  const [layout, setLayout] = useState<LayoutConfig>(DEFAULT_LAYOUT);
  const [isHydrated, setIsHydrated] = useState(false);

  useEffect(() => {
    const saved = localStorage.getItem(storageKey);
    if (saved) {
      try {
        setLayout(normalizeLayout(JSON.parse(saved)));
      } catch (e) {
        console.error("Failed to parse saved layout", e);
        localStorage.removeItem(storageKey);
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
