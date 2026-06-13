import { z } from "zod";
import { paperTradeExitReasonSchema, paperTradeSideSchema } from "@/lib/portfolio/paperTradesTypes";

export const shadowIntentKindSchema = z.enum(["open", "close"]);

export const shadowIntentPostBodySchema = z.object({
  clientIntentId: z.string().uuid().optional(),
  intentKind: shadowIntentKindSchema,
  symbol: z.string().min(1).max(32),
  side: paperTradeSideSchema,
  notional: z.number().finite().nonnegative(),
  entryPrice: z.number().finite().positive(),
  exitPrice: z.number().finite().positive().optional().nullable(),
  exitReason: paperTradeExitReasonSchema.optional().nullable(),
  strategyId: z.number().int().nonnegative(),
  strategyName: z.string().min(1).max(256).optional(),
});

export type ShadowIntentPostBody = z.infer<typeof shadowIntentPostBodySchema>;

export type ShadowTradeIntentDbRow = {
  id: string;
  created_at: string;
  user_id: string;
  client_intent_id: string;
  intent_kind: "open" | "close";
  symbol: string;
  side: string;
  notional: number;
  entry_price: number;
  exit_price: number | null;
  exit_reason: string | null;
  strategy_id: number;
  strategy_name: string | null;
  would_place_testnet: boolean;
};

export type ShadowIntentListItem = {
  id: string;
  createdAt: string;
  intentKind: "open" | "close";
  symbol: string;
  side: string;
  notional: number;
  entryPrice: number;
  exitPrice: number | null;
  exitReason: string | null;
  strategyId: number;
  strategyName: string | null;
  wouldPlaceTestnet: boolean;
};
