import { describe, expect, it } from "vitest";
import { BTC_FT_EXTENDED_DEFS_PROOF } from "@/lib/trading/btcFtStrategyTemplates";

describe("BTC FT extended template pool (removed)", () => {
  it("proof roster is empty — extended/template pool has been removed", () => {
    expect(BTC_FT_EXTENDED_DEFS_PROOF.length).toBe(0);
  });
});
