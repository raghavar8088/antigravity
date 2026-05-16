import type { DeskEngineStatus } from "@/components/desk/ui/StatusBadge";
import { moduleStatusChips } from "@/components/desk/WorkspaceNavPanel";
import { workspaceModuleDescription } from "@/lib/workspaceModuleDescription";

export type DeskChromeModule =
  | "options"
  | "options-selling"
  | "btcFuturesScalper"
  | "nifty"
  | "niftySelling"
  | "niftyBees";

const DESK_CHROME_MODULES = new Set<DeskChromeModule>([
  "options",
  "options-selling",
  "btcFuturesScalper",
  "nifty",
  "niftySelling",
  "niftyBees",
]);

export function usesDeskModuleChrome(module: string): module is DeskChromeModule {
  return DESK_CHROME_MODULES.has(module as DeskChromeModule);
}

export type DeskModuleChromeConfig = {
  title: string;
  tagline: string;
  description: string;
  chips: string[];
  status: DeskEngineStatus;
  paperEquity?: number;
  paperCurrency: "USD" | "INR";
  /** Modules that ship a full DeskShell inside the scalper (no outer shell). */
  embedsOwnShell: boolean;
};

const MODULE_TITLES: Record<DeskChromeModule, { title: string; tagline: string }> = {
  options: { title: "BTC Option Buying", tagline: "BTC options paper desk" },
  "options-selling": { title: "BTC Option Selling", tagline: "BTC short-premium paper desk" },
  btcFuturesScalper: { title: "Future Trading", tagline: "Multi-asset perpetual futures" },
  nifty: { title: "Nifty 50 Option Buying", tagline: "NSE NIFTY options paper desk" },
  niftySelling: { title: "Nifty Option Selling", tagline: "NIFTY premium-decay paper desk" },
  niftyBees: { title: "Nifty BEES Scalper", tagline: "NSE ETF paper desk" },
};

export function deskModuleChromeConfig(
  module: DeskChromeModule,
  opts: { online: boolean; paperEquity?: number; paperCurrency?: "USD" | "INR" },
): DeskModuleChromeConfig {
  const meta = MODULE_TITLES[module];
  return {
    title: meta.title,
    tagline: meta.tagline,
    description: workspaceModuleDescription(module),
    chips: moduleStatusChips(module),
    status: opts.online ? "live" : "syncing",
    paperEquity: opts.paperEquity,
    paperCurrency: opts.paperCurrency ?? (module === "nifty" || module === "niftySelling" || module === "niftyBees" ? "INR" : "USD"),
    embedsOwnShell: module === "btcFuturesScalper",
  };
}
