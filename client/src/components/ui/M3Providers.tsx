"use client";

import type { ReactNode } from "react";
import { DensityProvider } from "./DensityProvider";
import { M3TooltipProvider } from "./Tooltip";
import { SnackbarProvider } from "./Snackbar";
import { ThemeProvider } from "./ThemeProvider";

export function M3Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider>
      <DensityProvider>
        <M3TooltipProvider>
          <SnackbarProvider>{children}</SnackbarProvider>
        </M3TooltipProvider>
      </DensityProvider>
    </ThemeProvider>
  );
}

export function isM3Enabled(): boolean {
  if (typeof process !== "undefined" && process.env.NEXT_PUBLIC_UI_M3 === "0") return false;
  return true;
}
