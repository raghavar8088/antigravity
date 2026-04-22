/**
 * Cross-session persistence for BTC options buying / selling UIs.
 * Mirrors crypto/equity-state: survives engine redeploys when DATABASE_URL is set on the Next host.
 *
 * GET  ?desk=buy|sell
 * POST { desk, positions, trades, strategies, stats }
 */

import { NextRequest, NextResponse } from "next/server";
import { Pool } from "pg";

let pool: Pool | null = null;

function hasDatabaseUrl(): boolean {
  return Boolean(process.env.DATABASE_URL);
}

function getPool(): Pool {
  if (!pool) {
    const url = process.env.DATABASE_URL;
    if (!url) throw new Error("DATABASE_URL not set");
    pool = new Pool({ connectionString: url, ssl: { rejectUnauthorized: false }, max: 3 });
  }
  return pool;
}

async function ensureTable(client: { query: (sql: string) => Promise<unknown> }) {
  await client.query(`
    CREATE TABLE IF NOT EXISTS btc_options_paper_snapshot (
      desk       TEXT PRIMARY KEY,
      payload    JSONB NOT NULL DEFAULT '{}'::jsonb,
      saved_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
    )
  `);
}

type Desk = "buy" | "sell";

function parseDesk(raw: string | null): Desk | null {
  if (raw === "buy" || raw === "sell") return raw;
  return null;
}

export async function GET(req: NextRequest) {
  try {
    if (!hasDatabaseUrl()) {
      return NextResponse.json({ ok: true, found: false, disabled: true, reason: "DATABASE_URL not set" });
    }
    const desk = parseDesk(req.nextUrl.searchParams.get("desk"));
    if (!desk) {
      return NextResponse.json({ ok: false, error: "desk must be buy or sell" }, { status: 400 });
    }

    const db = getPool();
    const client = await db.connect();
    try {
      await ensureTable(client);
      const { rows } = await client.query<{ payload: unknown }>(
        `SELECT payload FROM btc_options_paper_snapshot WHERE desk = $1`,
        [desk],
      );
      if (!rows.length) {
        return NextResponse.json({ ok: true, found: false });
      }
      return NextResponse.json({ ok: true, found: true, state: rows[0].payload });
    } finally {
      client.release();
    }
  } catch (err) {
    console.error("[options/paper-snapshot GET]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}

type PostBody = {
  desk?: string;
  positions?: unknown[];
  trades?: unknown[];
  strategies?: unknown[];
  stats?: unknown | null;
};

export async function POST(req: NextRequest) {
  try {
    if (!hasDatabaseUrl()) {
      return NextResponse.json({ ok: true, skipped: true, reason: "DATABASE_URL not set" });
    }
    const body = (await req.json()) as PostBody;
    const desk = parseDesk(body.desk ?? null);
    if (!desk) {
      return NextResponse.json({ ok: false, error: "desk must be buy or sell" }, { status: 400 });
    }

    const payload = {
      positions: body.positions ?? [],
      trades: (body.trades ?? []).slice(0, 2000),
      strategies: body.strategies ?? [],
      stats: body.stats ?? null,
    };

    const db = getPool();
    const client = await db.connect();
    try {
      await ensureTable(client);
      await client.query(
        `INSERT INTO btc_options_paper_snapshot (desk, payload, saved_at)
         VALUES ($1, $2::jsonb, NOW())
         ON CONFLICT (desk) DO UPDATE SET payload = EXCLUDED.payload, saved_at = NOW()`,
        [desk, JSON.stringify(payload)],
      );
      return NextResponse.json({ ok: true });
    } finally {
      client.release();
    }
  } catch (err) {
    console.error("[options/paper-snapshot POST]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}
