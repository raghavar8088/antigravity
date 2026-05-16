import { DeltaClientError } from "./deltaErrors";

export const DELTA_TESTNET_API_BASE = "https://testnet-api.delta.exchange";

/** P3-A: execution adapter is testnet-only (`DELTA_TESTNET=true` or `1`). */
export function isDeltaTestnetExecutionEnabled(): boolean {
  const v = process.env.DELTA_TESTNET?.trim().toLowerCase();
  return v === "true" || v === "1";
}

export function resolveDeltaTestnetBaseUrl(): string {
  const proxy = process.env.DELTA_PROXY_URL?.replace(/\/+$/, "");
  return proxy || DELTA_TESTNET_API_BASE;
}

export function assertDeltaTestnetExecutionEnv(): void {
  if (!isDeltaTestnetExecutionEnabled()) {
    throw new DeltaClientError(
      "DELTA_TESTNET must be true or 1 — testnet execution adapter refuses mainnet",
    );
  }
}

export function getDeltaServerCredentials(): { apiKey: string; apiSecret: string } {
  const apiKey = process.env.DELTA_API_KEY?.trim() ?? "";
  const apiSecret = process.env.DELTA_API_SECRET?.trim() ?? "";
  if (!apiKey || !apiSecret) {
    throw new DeltaClientError("DELTA_API_KEY and DELTA_API_SECRET must be set (server env only)");
  }
  return { apiKey, apiSecret };
}
