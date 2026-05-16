import { deltaSign, nowTs } from "@/lib/deltaSign";
import {
  assertDeltaTestnetExecutionEnv,
  getDeltaServerCredentials,
  resolveDeltaTestnetBaseUrl,
} from "./deltaConfig";
import { DeltaClientError } from "./deltaErrors";
import type {
  DeltaApiResult,
  DeltaBalanceSnippet,
  DeltaMarginedPosition,
  DeltaOpenOrder,
  DeltaWalletBalance,
  PlaceOrderParams,
  PlaceOrderResult,
} from "./deltaTypes";

export type DeltaHttpResult = {
  ok: boolean;
  status: number;
  data: unknown;
};

export type DeltaHttpClient = (
  url: string,
  method: string,
  headers: Record<string, string>,
  body?: string,
) => Promise<DeltaHttpResult>;

/** Build HMAC auth headers for Delta v2 (testable without network). */
export function buildDeltaSignedHeaders(
  method: string,
  path: string,
  body: string,
  apiKey: string,
  apiSecret: string,
  timestamp: string = nowTs(),
): Record<string, string> {
  const signature = deltaSign(method, path, body, timestamp, apiSecret);
  return {
    "api-key": apiKey,
    timestamp,
    signature,
    "Content-Type": "application/json",
    Accept: "application/json",
  };
}

function parseNum(v: unknown): number {
  if (typeof v === "number" && Number.isFinite(v)) return v;
  if (typeof v === "string") {
    const n = parseFloat(v);
    return Number.isFinite(n) ? n : 0;
  }
  return 0;
}

function apiError(status: number, data: unknown): string {
  if (data && typeof data === "object") {
    const err = (data as { error?: unknown }).error;
    if (typeof err === "string") return err;
    if (err && typeof err === "object") {
      const o = err as { message?: string; code?: string };
      return o.message ?? o.code ?? `HTTP ${status}`;
    }
  }
  return `HTTP ${status}`;
}

export const defaultDeltaHttpClient: DeltaHttpClient = async (url, method, headers, body) => {
  const res = await fetch(url, {
    method,
    headers,
    body,
    signal: AbortSignal.timeout(15_000),
  });
  const raw = await res.text();
  let data: unknown;
  try {
    data = JSON.parse(raw);
  } catch {
    data = { _raw: raw.slice(0, 300) };
  }
  return { ok: res.ok, status: res.status, data };
};

export type DeltaClientOptions = {
  apiKey: string;
  apiSecret: string;
  baseUrl: string;
  httpClient: DeltaHttpClient;
};

export class DeltaTestnetClient {
  private readonly apiKey: string;
  private readonly apiSecret: string;
  private readonly baseUrl: string;
  private readonly http: DeltaHttpClient;

  constructor(opts: DeltaClientOptions) {
    this.apiKey = opts.apiKey;
    this.apiSecret = opts.apiSecret;
    this.baseUrl = opts.baseUrl.replace(/\/+$/, "");
    this.http = opts.httpClient;
  }

  /** Server env only; refuses when `DELTA_TESTNET` is not `true`/`1`. */
  static fromEnv(httpClient: DeltaHttpClient = defaultDeltaHttpClient): DeltaTestnetClient {
    assertDeltaTestnetExecutionEnv();
    const { apiKey, apiSecret } = getDeltaServerCredentials();
    return new DeltaTestnetClient({
      apiKey,
      apiSecret,
      baseUrl: resolveDeltaTestnetBaseUrl(),
      httpClient,
    });
  }

  private async signedRequest(
    method: string,
    path: string,
    body = "",
  ): Promise<DeltaHttpResult> {
    const headers = buildDeltaSignedHeaders(method, path, body, this.apiKey, this.apiSecret);
    return this.http(`${this.baseUrl}${path}`, method, headers, body || undefined);
  }

  async getBalances(): Promise<DeltaApiResult<DeltaWalletBalance[]>> {
    const path = "/v2/wallet/balances";
    const res = await this.signedRequest("GET", path);
    if (!res.ok) {
      return { ok: false, status: res.status, error: apiError(res.status, res.data), raw: res.data };
    }
    type Row = {
      asset_symbol?: string;
      balance?: unknown;
      available_balance?: unknown;
      blocked_margin?: unknown;
      unrealised_cashflow?: unknown;
    };
    const rows = (res.data as { result?: Row[] }).result ?? [];
    const balances: DeltaWalletBalance[] = rows.map((w) => ({
      asset: w.asset_symbol ?? "",
      balance: parseNum(w.balance),
      availableBalance: parseNum(w.available_balance),
      blockedBalance: parseNum(w.blocked_margin),
      unrealisedPnl: parseNum(w.unrealised_cashflow),
    }));
    return { ok: true, data: balances };
  }

  async getPositions(): Promise<DeltaApiResult<DeltaMarginedPosition[]>> {
    const path = "/v2/positions/margined";
    const res = await this.signedRequest("GET", path);
    if (!res.ok) {
      return { ok: false, status: res.status, error: apiError(res.status, res.data), raw: res.data };
    }
    type Row = {
      symbol?: string;
      product_id?: number;
      size?: unknown;
      entry_price?: unknown;
      mark_price?: unknown;
      unrealised_pnl?: unknown;
      realised_pnl?: unknown;
      margin?: unknown;
    };
    const rows = (res.data as { result?: Row[] }).result ?? [];
    const positions: DeltaMarginedPosition[] = [];
    for (const p of rows) {
      const size = parseNum(p.size);
      if (size === 0) continue;
      positions.push({
        symbol: p.symbol ?? "",
        productId: p.product_id ?? 0,
        size,
        entryPrice: parseNum(p.entry_price),
        markPrice: parseNum(p.mark_price),
        unrealisedPnl: parseNum(p.unrealised_pnl),
        realisedPnl: parseNum(p.realised_pnl),
        margin: parseNum(p.margin),
        side: size >= 0 ? "LONG" : "SHORT",
      });
    }
    return { ok: true, data: positions };
  }

  async getOpenOrders(): Promise<DeltaApiResult<DeltaOpenOrder[]>> {
    const path = "/v2/orders?state=open";
    const res = await this.signedRequest("GET", path);
    if (!res.ok) {
      return { ok: false, status: res.status, error: apiError(res.status, res.data), raw: res.data };
    }
    type Row = {
      id?: number;
      symbol?: string;
      product_id?: number;
      side?: string;
      size?: unknown;
      limit_price?: unknown;
      state?: string;
      created_at?: string;
    };
    const rows = (res.data as { result?: { data?: Row[] } }).result?.data ?? [];
    const orders: DeltaOpenOrder[] = rows.map((o) => ({
      orderId: String(o.id ?? ""),
      symbol: o.symbol ?? "",
      productId: o.product_id ?? 0,
      side: o.side === "buy" ? "buy" : "sell",
      size: parseNum(o.size),
      limitPrice: o.limit_price != null ? parseNum(o.limit_price) : null,
      state: o.state ?? "",
      createdAt: o.created_at ?? "",
    }));
    return { ok: true, data: orders };
  }

  async placeOrder(params: PlaceOrderParams): Promise<DeltaApiResult<PlaceOrderResult>> {
    const path = "/v2/orders";
    const payload: Record<string, unknown> = {
      product_id: params.productId,
      size: params.size,
      side: params.side,
      order_type: params.orderType,
      reduce_only: params.reduceOnly ?? false,
    };
    if (params.orderType === "limit_order") {
      if (params.limitPrice == null || !Number.isFinite(params.limitPrice)) {
        return { ok: false, status: 400, error: "limitPrice required for limit_order" };
      }
      payload.limit_price = String(params.limitPrice);
    }
    const body = JSON.stringify(payload);
    const res = await this.signedRequest("POST", path, body);
    if (!res.ok) {
      return { ok: false, status: res.status, error: apiError(res.status, res.data), raw: res.data };
    }
    type Row = {
      id?: number;
      symbol?: string;
      state?: string;
      average_fill_price?: unknown;
    };
    const row = (res.data as { result?: Row }).result;
    return {
      ok: true,
      data: {
        orderId: String(row?.id ?? ""),
        symbol: row?.symbol ?? "",
        state: row?.state ?? "",
        averageFillPrice:
          row?.average_fill_price != null ? parseNum(row.average_fill_price) : null,
      },
    };
  }

  async cancelOrder(orderId: string | number): Promise<DeltaApiResult<{ cancelled: boolean }>> {
    const id = String(orderId).trim();
    if (!id) return { ok: false, status: 400, error: "orderId required" };
    const path = `/v2/orders/${id}`;
    const res = await this.signedRequest("DELETE", path);
    if (!res.ok) {
      return { ok: false, status: res.status, error: apiError(res.status, res.data), raw: res.data };
    }
    return { ok: true, data: { cancelled: true } };
  }
}

/** Operator-facing wallet summary (no full book). */
export function balanceSnippetFromWallets(
  balances: readonly DeltaWalletBalance[],
): DeltaBalanceSnippet {
  return balances
    .filter((w) => w.balance !== 0 || w.availableBalance !== 0)
    .slice(0, 8)
    .map((w) => ({
      asset: w.asset,
      availableBalance: w.availableBalance,
    }));
}

// Thin exports matching LIVE_TRADING_PHASE naming
export async function getBalances(client?: DeltaTestnetClient) {
  return (client ?? DeltaTestnetClient.fromEnv()).getBalances();
}

export async function getPositions(client?: DeltaTestnetClient) {
  return (client ?? DeltaTestnetClient.fromEnv()).getPositions();
}

export async function getOpenOrders(client?: DeltaTestnetClient) {
  return (client ?? DeltaTestnetClient.fromEnv()).getOpenOrders();
}

export async function placeOrder(params: PlaceOrderParams, client?: DeltaTestnetClient) {
  return (client ?? DeltaTestnetClient.fromEnv()).placeOrder(params);
}

export async function cancelOrder(orderId: string | number, client?: DeltaTestnetClient) {
  return (client ?? DeltaTestnetClient.fromEnv()).cancelOrder(orderId);
}
