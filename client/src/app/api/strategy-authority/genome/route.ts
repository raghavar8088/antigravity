import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getPortfolioGenomes, getGenomeById } from "@/lib/strategyAuthority/portfolioIntelligenceMongo";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  const { searchParams } = new URL(req.url);
  const strategyId = searchParams.get("id");
  const limit = parseInt(searchParams.get("limit") ?? "305", 10);

  try {
    if (strategyId) {
      const genome = await getGenomeById(strategyId);
      if (!genome) {
        return NextResponse.json({ ok: false, code: "NOT_FOUND" }, { status: 404 });
      }
      return NextResponse.json({ ok: true, genome });
    }

    const genomes = await getPortfolioGenomes(Math.min(305, limit));
    return NextResponse.json({ ok: true, genomes, count: genomes.length });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "GENOME_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
