"use client";

import type { ReactNode } from "react";
import { createContext, useCallback, useContext, useMemo, useState } from "react";

export type DensityMode = "comfortable" | "compact" | "ultra-compact";

const DensityContext = createContext<{ density: DensityMode; setDensity: (d: DensityMode) => void } | null>(null);

export function DensityProvider({ children, defaultDensity = "comfortable" }: { children: ReactNode; defaultDensity?: DensityMode }) {
  const [density, setDensity] = useState<DensityMode>(defaultDensity);
  const value = useMemo(() => ({ density, setDensity }), [density]);
  return <DensityContext.Provider value={value}>{children}</DensityContext.Provider>;
}

export function useDensity() {
  const ctx = useContext(DensityContext);
  if (!ctx) return { density: "comfortable" as DensityMode, setDensity: () => {} };
  return ctx;
}

export function densityClass(density: DensityMode): string {
  return `m3-density-${density}`;
}
