import { z } from "zod";

export const testnetPlaceOrderBodySchema = z
  .object({
    symbol: z.string().min(1).max(32).transform((s) => s.trim().toUpperCase()),
    side: z.enum(["buy", "sell"]),
    size: z.coerce.number().positive().max(1_000),
    type: z.enum(["limit", "market"]),
    price: z.coerce.number().positive().optional(),
  })
  .superRefine((val, ctx) => {
    if (val.type === "limit" && (val.price == null || !Number.isFinite(val.price))) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "price is required for limit orders",
        path: ["price"],
      });
    }
    if (val.type === "market" && val.price != null) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "price must be omitted for market orders",
        path: ["price"],
      });
    }
  });

export type TestnetPlaceOrderBody = z.infer<typeof testnetPlaceOrderBodySchema>;

export const testnetCancelOrderBodySchema = z.object({
  orderId: z.union([z.string().min(1).max(64), z.coerce.number().int().positive()]),
});

export type TestnetCancelOrderBody = z.infer<typeof testnetCancelOrderBodySchema>;

export function isDeskTestnetOpsPanelEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_TESTNET_OPS === "1";
}
