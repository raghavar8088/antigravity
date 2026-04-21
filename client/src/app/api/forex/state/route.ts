/**
 * /api/forex/state — Persist and restore Forex engine state
 */

import { NextResponse } from "next/server";
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
    CREATE TABLE IF NOT EXISTS forex_state (
      id              INTEGER PRIMARY KEY DEFAULT 1,
      balance         DOUBLE PRECISION NOT NULL DEFAULT 1000000,
      total_wins      INTEGER          NOT NULL DEFAULT 0,
      total_losses    INTEGER          NOT NULL DEFAULT 0,
      total_pnl       DOUBLE PRECISION NOT NULL DEFAULT 0,
      trade_seq       INTEGER          NOT NULL DEFAULT 0,
      positions_json  TEXT             NOT NULL DEFAULT '[]',
      trades_json     TEXT             NOT NULL DEFAULT '[]',
      strategies_json TEXT             NOT NULL DEFAULT '[]',
      saved_at        TIMESTAMPTZ      NOT NULL DEFAULT NOW()
    )
  `);
  await client.query(`
    INSERT INTO forex_state (id) VALUES (1)
    ON CONFLICT (id) DO NOTHING
  `);
}

type Payload = {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalPnl: number;
  tradeSeq: number;
  positions: unknown[];
  trades: unknown[];
  strategies: unknown[];
};

function toFiniteNumber(value: unknown, fallback = 0): number {
  const numeric = typeof value === "number" ? value : Number(value);
  return Number.isFinite(numeric) ? numeric : fallback;
}

function toNonNegativeInt(value: unknown, fallback = 0): number {
  const parsed = Math.trunc(toFiniteNumber(value, fallback));
  return parsed >= 0 ? parsed : fallback;
}

function parseJsonArray(raw: string): unknown[] {
  try {
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function normalizePayload(body: Partial<Payload>): Payload {
  return {
    balance: toFiniteNumber(body.balance, 1_000_000),
    totalWins: toNonNegativeInt(body.totalWins, 0),
    totalLosses: toNonNegativeInt(body.totalLosses, 0),
    totalPnl: toFiniteNumber(body.totalPnl, 0),
    tradeSeq: toNonNegativeInt(body.tradeSeq, 0),
    positions: Array.isArray(body.positions) ? body.positions : [],
    trades: Array.isArray(body.trades) ? body.trades.slice(0, 5000) : [],
    strategies: Array.isArray(body.strategies) ? body.strategies : [],
  };
}

export async function GET() {
  try {
    if (!hasDatabaseUrl()) {
      return NextResponse.json({ ok: true, found: false, disabled: true, reason: "DATABASE_URL not set" });
    }

    const db = getPool();
    const client = await db.connect();
    try {
      await ensureTable(client);
      const { rows } = await client.query<{
        balance: string;
        total_wins: string;
        total_losses: string;
        total_pnl: string;
        trade_seq: string;
        positions_json: string;
        trades_json: string;
        strategies_json: string;
        saved_at: string;
      }>(`SELECT * FROM forex_state WHERE id = 1`);
      if (!rows.length) return NextResponse.json({ ok: true, found: false });
      const row = rows[0];
      return NextResponse.json({
        ok: true,
        found: true,
        state: {
          balance: toFiniteNumber(row.balance, 1_000_000),
          totalWins: toNonNegativeInt(row.total_wins, 0),
          totalLosses: toNonNegativeInt(row.total_losses, 0),
          totalPnl: toFiniteNumber(row.total_pnl, 0),
          tradeSeq: toNonNegativeInt(row.trade_seq, 0),
          positions: parseJsonArray(row.positions_json),
          trades: parseJsonArray(row.trades_json),
          strategies: parseJsonArray(row.strategies_json),
        },
        savedAt: row.saved_at,
      });
    } finally {
      client.release();
    }
  } catch (err) {
    console.error("[forex/state GET]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}

export async function POST(request: Request) {
  try {
    if (!hasDatabaseUrl()) {
      return NextResponse.json({ ok: true, skipped: true, reason: "DATABASE_URL not set" });
    }

    const body = normalizePayload((await request.json()) as Partial<Payload>);
    const db = getPool();
    const client = await db.connect();
    try {
      await ensureTable(client);
      await client.query(
        `UPDATE forex_state SET
          balance         = $1,
          total_wins      = $2,
          total_losses    = $3,
          total_pnl       = $4,
          trade_seq       = $5,
          positions_json  = $6,
          trades_json     = $7,
          strategies_json = $8,
          saved_at        = NOW()
         WHERE id = 1`,
        [
          body.balance,
          body.totalWins,
          body.totalLosses,
          body.totalPnl,
          body.tradeSeq,
          JSON.stringify(body.positions ?? []),
          JSON.stringify(body.trades ?? []),
          JSON.stringify(body.strategies ?? []),
        ],
      );
      return NextResponse.json({ ok: true });
    } finally {
      client.release();
    }
  } catch (err) {
    console.error("[forex/state POST]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}
