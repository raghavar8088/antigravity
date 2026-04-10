/**
 * /api/nifty/state — Persist and restore NIFTY options engine state
 *
 * GET  — load last saved state from PostgreSQL
 * POST — save current engine snapshot to PostgreSQL
 *
 * Uses the same DATABASE_URL as the Go engine (single shared Postgres instance).
 * Table: nifty_client_state (auto-created on first call)
 */

import { NextResponse } from "next/server";
import { Pool } from "pg";

// ── DB connection (reuse Go engine's DATABASE_URL) ─────────────────────────

let pool: Pool | null = null;

function getPool(): Pool {
  if (!pool) {
    const url = process.env.DATABASE_URL;
    if (!url) throw new Error("DATABASE_URL not set");
    pool = new Pool({ connectionString: url, ssl: { rejectUnauthorized: false }, max: 3 });
  }
  return pool;
}

// ── Ensure table exists ────────────────────────────────────────────────────

async function ensureTable(client: { query: (sql: string) => Promise<unknown> }) {
  await client.query(`
    CREATE TABLE IF NOT EXISTS nifty_client_state (
      id                  INTEGER PRIMARY KEY DEFAULT 1,
      balance             DOUBLE PRECISION    NOT NULL DEFAULT 1000000,
      total_wins          INTEGER             NOT NULL DEFAULT 0,
      total_losses        INTEGER             NOT NULL DEFAULT 0,
      total_pnl           DOUBLE PRECISION    NOT NULL DEFAULT 0,
      total_premium_spent DOUBLE PRECISION    NOT NULL DEFAULT 0,
      trade_seq           INTEGER             NOT NULL DEFAULT 0,
      minute_bars_json    TEXT                NOT NULL DEFAULT '[]',
      trades_json         TEXT                NOT NULL DEFAULT '[]',
      strategies_json     TEXT                NOT NULL DEFAULT '[]',
      saved_at            TIMESTAMPTZ         NOT NULL DEFAULT NOW()
    )
  `);
  // Ensure at least one row exists
  await client.query(`
    INSERT INTO nifty_client_state (id) VALUES (1)
    ON CONFLICT (id) DO NOTHING
  `);
}

// ── Types ──────────────────────────────────────────────────────────────────

export type NiftyStatePayload = {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalPnl: number;
  totalPremiumSpent: number;
  tradeSeq: number;
  minuteBars: number[];
  trades: unknown[];
  strategies: unknown[];
};

// ── GET — load state ───────────────────────────────────────────────────────

export async function GET() {
  try {
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
        minute_bars_json: string;
        trades_json: string;
        strategies_json: string;
        saved_at: string;
      }>(`SELECT * FROM nifty_client_state WHERE id = 1`);

      if (!rows.length) {
        return NextResponse.json({ ok: true, found: false });
      }

      const row = rows[0];
      const state: NiftyStatePayload = {
        balance:           parseFloat(row.balance),
        totalWins:         parseInt(row.total_wins),
        totalLosses:       parseInt(row.total_losses),
        totalPnl:          parseFloat(row.total_pnl),
        totalPremiumSpent: parseFloat(row.total_premium_spent),
        tradeSeq:          parseInt(row.trade_seq),
        minuteBars:        JSON.parse(row.minute_bars_json) as number[],
        trades:            JSON.parse(row.trades_json) as unknown[],
        strategies:        JSON.parse(row.strategies_json) as unknown[],
      };

      return NextResponse.json({ ok: true, found: true, state, savedAt: row.saved_at });
    } finally {
      client.release();
    }
  } catch (err) {
    console.error("[nifty/state GET]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}

// ── POST — save state ──────────────────────────────────────────────────────

export async function POST(request: Request) {
  try {
    const body = await request.json() as NiftyStatePayload;

    const db = getPool();
    const client = await db.connect();
    try {
      await ensureTable(client);
      await client.query(
        `UPDATE nifty_client_state SET
          balance             = $1,
          total_wins          = $2,
          total_losses        = $3,
          total_pnl           = $4,
          total_premium_spent = $5,
          trade_seq           = $6,
          minute_bars_json    = $7,
          trades_json         = $8,
          strategies_json     = $9,
          saved_at            = NOW()
         WHERE id = 1`,
        [
          body.balance,
          body.totalWins,
          body.totalLosses,
          body.totalPnl,
          body.totalPremiumSpent,
          body.tradeSeq,
          JSON.stringify(body.minuteBars ?? []),
          JSON.stringify((body.trades ?? []).slice(0, 500)),
          JSON.stringify(body.strategies ?? []),
        ],
      );
      return NextResponse.json({ ok: true });
    } finally {
      client.release();
    }
  } catch (err) {
    console.error("[nifty/state POST]", err);
    return NextResponse.json({ ok: false, error: String(err) }, { status: 500 });
  }
}
