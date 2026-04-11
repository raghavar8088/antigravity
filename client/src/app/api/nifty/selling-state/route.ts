/**
 * /api/nifty/selling-state — Persist and restore NIFTY option selling state
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
    CREATE TABLE IF NOT EXISTS nifty_selling_state (
      id                  INTEGER PRIMARY KEY DEFAULT 1,
      balance             DOUBLE PRECISION NOT NULL DEFAULT 1000000,
      total_wins          INTEGER          NOT NULL DEFAULT 0,
      total_losses        INTEGER          NOT NULL DEFAULT 0,
      total_pnl           DOUBLE PRECISION NOT NULL DEFAULT 0,
      total_premium_spent DOUBLE PRECISION NOT NULL DEFAULT 0,
      trade_seq           INTEGER          NOT NULL DEFAULT 0,
      last_price          DOUBLE PRECISION NOT NULL DEFAULT 0,
      last_minute         BIGINT           NOT NULL DEFAULT 0,
      minute_bars_json    TEXT             NOT NULL DEFAULT '[]',
      positions_json      TEXT             NOT NULL DEFAULT '[]',
      trades_json         TEXT             NOT NULL DEFAULT '[]',
      strategies_json     TEXT             NOT NULL DEFAULT '[]',
      saved_at            TIMESTAMPTZ      NOT NULL DEFAULT NOW()
    )
  `);
  await client.query(`
    INSERT INTO nifty_selling_state (id) VALUES (1)
    ON CONFLICT (id) DO NOTHING
  `);
}

type Payload = {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalPnl: number;
  totalPremiumSpent: number;
  tradeSeq: number;
  lastPrice: number;
  lastMinute: number;
  minuteBars: unknown[];
  positions: unknown[];
  trades: unknown[];
  strategies: unknown[];
};

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
        total_premium_spent: string;
        trade_seq: string;
        last_price: string;
        last_minute: string;
        minute_bars_json: string;
        positions_json: string;
        trades_json: string;
        strategies_json: string;
        saved_at: string;
      }>(`SELECT * FROM nifty_selling_state WHERE id = 1`);
      if (!rows.length) return NextResponse.json({ ok: true, found: false });
      const row = rows[0];
      return NextResponse.json({
        ok: true,
        found: true,
        state: {
          balance: parseFloat(row.balance),
          totalWins: parseInt(row.total_wins),
          totalLosses: parseInt(row.total_losses),
          totalPnl: parseFloat(row.total_pnl),
          totalPremiumSpent: parseFloat(row.total_premium_spent),
          tradeSeq: parseInt(row.trade_seq),
          lastPrice: parseFloat(row.last_price),
          lastMinute: parseInt(row.last_minute),
          minuteBars: JSON.parse(row.minute_bars_json) as unknown[],
          positions: JSON.parse(row.positions_json) as unknown[],
          trades: JSON.parse(row.trades_json) as unknown[],
          strategies: JSON.parse(row.strategies_json) as unknown[],
        },
        savedAt: row.saved_at,
      });
    } finally {
      client.release();
    }
  } catch (err) {
    console.error("[nifty/selling-state GET]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}

export async function POST(request: Request) {
  try {
    if (!hasDatabaseUrl()) {
      return NextResponse.json({ ok: true, skipped: true, reason: "DATABASE_URL not set" });
    }
    const body = await request.json() as Payload;
    const db = getPool();
    const client = await db.connect();
    try {
      await ensureTable(client);
      await client.query(
        `UPDATE nifty_selling_state SET
          balance             = $1,
          total_wins          = $2,
          total_losses        = $3,
          total_pnl           = $4,
          total_premium_spent = $5,
          trade_seq           = $6,
          last_price          = $7,
          last_minute         = $8,
          minute_bars_json    = $9,
          positions_json      = $10,
          trades_json         = $11,
          strategies_json     = $12,
          saved_at            = NOW()
         WHERE id = 1`,
        [
          body.balance,
          body.totalWins,
          body.totalLosses,
          body.totalPnl,
          body.totalPremiumSpent,
          body.tradeSeq,
          body.lastPrice,
          body.lastMinute,
          JSON.stringify(body.minuteBars ?? []),
          JSON.stringify(body.positions ?? []),
          JSON.stringify((body.trades ?? []).slice(0, 500)),
          JSON.stringify(body.strategies ?? []),
        ],
      );
      return NextResponse.json({ ok: true });
    } finally {
      client.release();
    }
  } catch (err) {
    console.error("[nifty/selling-state POST]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}
