export type DeskChromeModule = never;

const DESK_CHROME_MODULES = new Set<string>();

export function usesDeskModuleChrome(module: string): module is DeskChromeModule {
  return DESK_CHROME_MODULES.has(module);
}

export type DeskModuleChromeConfig = {
  title: string;
  tagline: string;
  description: string;
  chips: string[];
  status: import("@/components/desk/ui/StatusBadge").DeskEngineStatus;
  paperEquity?: number;
  paperCurrency: "USD" | "INR";
  embedsOwnShell: boolean;
};

export function deskModuleChromeConfig(
  _module: DeskChromeModule,
  _opts: { online: boolean; paperEquity?: number; paperCurrency?: "USD" | "INR" },
): DeskModuleChromeConfig {
  return {
    title: "",
    tagline: "",
    description: "",
    chips: [],
    status: "syncing",
    paperCurrency: "USD",
    embedsOwnShell: false,
  };
}
