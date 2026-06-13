import type { BTCFuturesTrade } from "@/lib/trading/btcFuturesTrade.types";
import type {
  ShadowIntentPostBody,
  ShadowIntentListItem,
  ShadowTradeIntentDbRow,
} from "@/lib/ai/shadowTradeIntentTypes";

export type ShadowPaperOpenLike = {
  symbol: string;
  side: "LONG" | "SHORT";
  notional: number;
  entryPrice: number;
  strategyId: number;
  strategyName: string;
};

export function isDeskShadowIntentsEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_SHADOW_INTENTS === "1";
}

export function isDeskShadowLogOpenEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_SHADOW_LOG_OPEN === "1";
}

function newClientIntentId(): string {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return globalThis.crypto.randomUUID();
  }
  return `shadow-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export function shadowIntentFromPaperOpen(position: ShadowPaperOpenLike): ShadowIntentPostBody {
  return {
    clientIntentId: newClientIntentId(),
    intentKind: "open",
    symbol: position.symbol,
    side: position.side,
    notional: position.notional,
    entryPrice: position.entryPrice,
    exitPrice: null,
    exitReason: null,
    strategyId: position.strategyId,
    strategyName: position.strategyName,
  };
}

export function shadowIntentFromPaperClose(trade: BTCFuturesTrade): ShadowIntentPostBody {
  return {
    clientIntentId: trade.clientTradeId,
    intentKind: "close",
    symbol: trade.symbol,
    side: trade.side,
    notional: trade.notional,
    entryPrice: trade.entryPrice,
    exitPrice: trade.exitPrice,
    exitReason: trade.exitReason,
    strategyId: trade.strategyId,
    strategyName: trade.strategyName,
  };
}

export function shadowIntentPostBodyToInsertRow(
  userId: string,
  body: ShadowIntentPostBody,
  wouldPlaceTestnet: boolean,
): Omit<ShadowTradeIntentDbRow, "id" | "created_at"> & { created_at?: string } {
  return {
    user_id: userId,
    client_intent_id: body.clientIntentId ?? newClientIntentId(),
    intent_kind: body.intentKind,
    symbol: body.symbol,
    side: body.side,
    notional: body.notional,
    entry_price: body.entryPrice,
    exit_price: body.exitPrice ?? null,
    exit_reason: body.exitReason ?? null,
    strategy_id: body.strategyId,
    strategy_name: body.strategyName ?? null,
    would_place_testnet: wouldPlaceTestnet,
  };
}

export function dbRowToShadowIntentListItem(row: ShadowTradeIntentDbRow): ShadowIntentListItem {
  return {
    id: row.id,
    createdAt: row.created_at,
    intentKind: row.intent_kind,
    symbol: row.symbol,
    side: row.side,
    notional: row.notional,
    entryPrice: row.entry_price,
    exitPrice: row.exit_price,
    exitReason: row.exit_reason,
    strategyId: row.strategy_id,
    strategyName: row.strategy_name,
    wouldPlaceTestnet: row.would_place_testnet,
  };
}
