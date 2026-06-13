/**
 * POST /api/strategy-authority/compute-portfolio
 *
 * Triggers the full APICAP pipeline:
 *   Correlation → Diversification → Regime → Allocation → Genome → Candidate Queue → Portfolio Construction
 *
 * This is a long-running operation (30–120s depending on strategy count).
 * Protected by CRON_SECRET in production-like envs; open in dev.
 */

import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { runFullPortfolioIntelligence } from "@/lib/strategyAuthority/portfolioIntelligenceMongo";

export const dynamic = "force-dynamic";
// Allow up to 5 min execution (Vercel Pro)
export const maxDuration = 300;

export async function POST(req: Request): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }

  // Optional: require CRON_SECRET for protection
  const secret = process.env.CRON_SECRET?.trim();
  if (secret) {
    const auth = req.headers.get("authorization") ?? "";
    if (auth !== `Bearer ${secret}`) {
      return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
    }
  }

  try {
    const result = await runFullPortfolioIntelligence();
    return NextResponse.json({ ok: true, result });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return NextResponse.json({ ok: false, error: msg }, { status: 500 });
  }
}
