/** All TypeScript interfaces for the Live Trading Dashboard (Phase 3). */

export type Exchange = "BINANCE" | "DELTA" | "COINBASE";
export type Direction = "LONG" | "SHORT" | "BUY" | "SELL";
export type OrderType = "MARKET" | "LIMIT" | "SL" | "SL-M";
export type OrderStatus = "PENDING" | "OPEN" | "PARTIAL_FILL" | "FILLED" | "CANCELLED" | "REJECTED";

// ── P&L Update (WebSocket tick) ───────────────────────────────────────────────
export interface PnlUpdate {
  strategyId: string;
  symbol: string;
  exchange: Exchange;
  direction: "LONG" | "SHORT";
  entryPrice: number;
  currentPrice: number;
  unrealizedPnl: number;
  realizedPnl: number;
  sessionPnl: number;
  quantity: number;
  timestamp: string;
}

// ── Order (OMS v3) ────────────────────────────────────────────────────────────
export interface Order {
  orderId: string;
  symbol: string;
  exchange: Exchange;
  strategyId: string;
  direction: "BUY" | "SELL";
  orderType: OrderType;
  quantity: number;
  price: number | null;
  status: OrderStatus;
  filledQuantity: number;
  fillPrice: number | null;
  rejectionReason: string | null;
  placedAt: string;
  updatedAt: string;
  latencyMs: number | null;
}

// ── Position ──────────────────────────────────────────────────────────────────
export interface Position {
  positionId: string;
  symbol: string;
  exchange: Exchange;
  strategyId: string;
  direction: "LONG" | "SHORT";
  size: number;
  entryPrice: number;
  currentPrice: number;
  unrealizedPnl: number;
  returnPct: number;
  stopLoss: number | null;
  target: number | null;
  openedAt: string;
  ageMs: number;
}

// ── Engine Event (Event Sourcing Ledger) ──────────────────────────────────────
export type EngineEventType =
  | "OrderCreated"
  | "OrderAccepted"
  | "OrderFilled"
  | "PartialFilled"
  | "OrderRejected"
  | "OrderCancelled"
  | "PositionOpened"
  | "PositionClosed"
  | "RiskViolation"
  | "KillSwitchTriggered"
  | "ReconciliationMismatch"
  | "StrategySignal"
  | "EngineHealthCheck";

export interface EngineEvent {
  eventId: string;
  type: EngineEventType;
  symbol?: string;
  strategyId?: string;
  orderId?: string;
  exchange?: Exchange;
  details: string;
  pnl?: number;
  timestamp: string;
}

// ── Reconciliation ────────────────────────────────────────────────────────────
export type ReconciliationStatus = "IN_SYNC" | "DRIFT_DETECTED" | "MISMATCH";

export interface ReconciliationState {
  status: ReconciliationStatus;
  lastSyncAt: string;
  mismatchCount: number;
  driftItems: Array<{
    symbol: string;
    exchangeQty: number;
    omsQty: number;
    ledgerQty: number;
    resolvedAutomatically: boolean;
  }>;
}

// ── Risk Summary (used by RiskHUD and risk page) ──────────────────────────────
export interface RiskSummary {
  dailyPnl: number;
  dailyPnlLimit: number;
  dailyPnlUsedPct: number;
  drawdownPct: number;
  drawdownLimitPct: number;
  drawdownUsedPct: number;
  exposureUsdt: number;
  totalCapitalUsdt: number;
  exposurePct: number;
  cashPct: number;
  nseExposurePct: number;
  cryptoLongPct: number;
  cryptoShortPct: number;
  activeStrategies: number;
  openPositions: number;
  openOrders: number;
  lastUpdated: string;
}

export interface RiskGate {
  id: string;
  label: string;
  state: "open" | "throttled" | "closed";
  lastCheckAt: string;
  passCountToday: number;
  failCountToday: number;
}
