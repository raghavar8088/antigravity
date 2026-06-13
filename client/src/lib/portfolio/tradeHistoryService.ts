/**
 * Append-only cross-module trade archive + Excel export.
 * Rows live in Postgres (trade_history_archive) so account resets / clear-history
 * do not remove already-archived trades.
 */

import ExcelJS from "exceljs";
import type { Pool, PoolClient } from "pg";
import { Pool as PgPool } from "pg";

let pool: Pool | null = null;

export function getTradeHistoryPool(): Pool {
  const url = process.env.DATABASE_URL;
  if (!url) throw new Error("DATABASE_URL not set");
  if (!pool) {
    pool = new PgPool({ connectionString: url, ssl: { rejectUnauthorized: false }, max: 5 });
  }
  return pool;
}

export type ArchiveRow = {
  trade_day: string;
  module: string;
  trade_key: string;
  payload: Record<string, unknown>;
};

export function authorizeCronRequest(req: Request): boolean {
  const secret = process.env.CRON_SECRET;
  if (!secret) return false;
  const auth = req.headers.get("authorization");
  if (auth === `Bearer ${secret}`) return true;
  try {
    const url = new URL(req.url);
    return url.searchParams.get("secret") === secret;
  } catch {
    return false;
  }
}

export function defaultArchiveDateUtc(): string {
  const d = new Date();
  d.setUTCHours(0, 0, 0, 0);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}

export function isIsoDate(s: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(s);
}

/** Reads JSON body when present; safe for GET (returns null). */
export async function readArchiveDateFromRequest(req: Request): Promise<string | null> {
  if (req.method === "POST" || req.method === "PUT") {
    try {
      const ct = req.headers.get("content-type") ?? "";
      if (ct.includes("application/json")) {
        const j = (await req.json()) as { date?: unknown };
        if (typeof j.date === "string" && isIsoDate(j.date)) return j.date;
      }
    } catch {
      /* ignore */
    }
  }
  try {
    const url = new URL(req.url);
    const q = url.searchParams.get("date");
    if (q && isIsoDate(q)) return q;
  } catch {
    /* ignore */
  }
  return null;
}

export async function ensureArchiveTable(client: PoolClient): Promise<void> {
  await client.query(`
    CREATE TABLE IF NOT EXISTS trade_history_archive (
      id BIGSERIAL PRIMARY KEY,
      trade_day DATE NOT NULL,
      module TEXT NOT NULL,
      trade_key TEXT NOT NULL,
      payload JSONB NOT NULL,
      inserted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      UNIQUE (trade_day, module, trade_key)
    )
  `);
  await client.query(`
    CREATE INDEX IF NOT EXISTS idx_trade_history_archive_trade_day
    ON trade_history_archive (trade_day DESC)
  `);
}

function isMissingRelation(err: unknown): boolean {
  return Boolean(err && typeof err === "object" && "code" in err && (err as { code: string }).code === "42P01");
}

async function safeQuery<T extends Record<string, unknown> = Record<string, unknown>>(
  client: PoolClient,
  text: string,
  params?: unknown[],
): Promise<T[]> {
  try {
    const { rows } = await client.query<T>(text, params);
    return rows;
  } catch (err) {
    if (isMissingRelation(err)) return [];
    throw err;
  }
}

function toUtcDayString(value: unknown): string | null {
  if (value == null) return null;
  if (value instanceof Date) {
    if (Number.isNaN(value.getTime())) return null;
    return value.toISOString().slice(0, 10);
  }
  if (typeof value === "number") {
    const ms = value < 1e12 ? value * 1000 : value;
    const d = new Date(ms);
    if (Number.isNaN(d.getTime())) return null;
    return d.toISOString().slice(0, 10);
  }
  if (typeof value === "string") {
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return null;
    return d.toISOString().slice(0, 10);
  }
  return null;
}

export function tradeExitUtcDay(trade: Record<string, unknown>): string | null {
  const exit =
    trade.exitTime ??
    trade.exit_time ??
    trade.ExitTime ??
    trade.exitAt ??
    trade.closedAt;
  return toUtcDayString(exit);
}

function stableTradeKey(module: string, trade: Record<string, unknown>, seq: number): string {
  const id = trade.id;
  if (typeof id === "string" && id.length > 0) return id.length > 220 ? id.slice(0, 220) : id;
  if (typeof id === "number" && Number.isFinite(id)) return String(id);
  return `${module}-${seq}`;
}

function parseJsonArray(raw: string | null | undefined): unknown[] {
  if (!raw || raw === "[]") return [];
  try {
    const v = JSON.parse(raw) as unknown;
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}

function rowToPlainPayload(row: Record<string, unknown>): Record<string, unknown> {
  const o: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(row)) {
    if (v instanceof Date) o[k] = v.toISOString();
    else o[k] = v;
  }
  return o;
}

function addJsonTrades(out: ArchiveRow[], module: string, trades: unknown[], targetDay: string): void {
  if (!Array.isArray(trades)) return;
  let seq = 0;
  for (const t of trades) {
    if (!t || typeof t !== "object") {
      seq++;
      continue;
    }
    const tr = t as Record<string, unknown>;
    const day = tradeExitUtcDay(tr);
    if (day !== targetDay) {
      seq++;
      continue;
    }
    const trade_key = stableTradeKey(module, tr, seq);
    out.push({ trade_day: targetDay, module, trade_key, payload: tr });
    seq++;
  }
}

function readPaperTrades(payload: unknown): unknown[] {
  if (!payload || typeof payload !== "object") return [];
  const p = payload as { trades?: unknown };
  return Array.isArray(p.trades) ? p.trades : [];
}

async function loadPaperDesk(client: PoolClient, desk: "buy" | "sell"): Promise<unknown[]> {
  const rows = await safeQuery<{ payload: unknown }>(
    client,
    `SELECT payload FROM btc_options_paper_snapshot WHERE desk = $1`,
    [desk],
  );
  if (!rows.length) return [];
  return readPaperTrades(rows[0].payload);
}

export async function collectRowsForDay(client: PoolClient, targetDay: string): Promise<ArchiveRow[]> {
  const out: ArchiveRow[] = [];

  const rel = await safeQuery<Record<string, unknown>>(
    client,
    `SELECT * FROM trades
     WHERE ((exit_time AT TIME ZONE 'UTC')::date) = $1::date`,
    [targetDay],
  );
  for (const row of rel) {
    const day = toUtcDayString(row.exit_time);
    if (day !== targetDay) continue;
    const id = row.id;
    const trade_key = typeof id === "string" && id.length ? id : stableTradeKey("btc_futures", row, out.length);
    out.push({
      trade_day: targetDay,
      module: "btc_futures",
      trade_key,
      payload: rowToPlainPayload(row),
    });
  }

  const eng = await safeQuery<{
    trades_json: string;
    options_trades_json: string;
    options_selling_trades_json: string;
  }>(
    client,
    `SELECT trades_json,
            COALESCE(options_trades_json, '[]') AS options_trades_json,
            COALESCE(options_selling_trades_json, '[]') AS options_selling_trades_json
     FROM engine_state WHERE id = 1`,
  );

  if (eng.length) {
    const e = eng[0];
    // Same module + keys as relational `trades` so duplicates are deduped by ON CONFLICT.
    addJsonTrades(out, "btc_futures", parseJsonArray(e.trades_json), targetDay);

    const paperBuy = await loadPaperDesk(client, "buy");
    if (paperBuy.length) addJsonTrades(out, "btc_options_buy", paperBuy, targetDay);
    else addJsonTrades(out, "btc_options_buy", parseJsonArray(e.options_trades_json), targetDay);

    const paperSell = await loadPaperDesk(client, "sell");
    if (paperSell.length) addJsonTrades(out, "btc_options_sell", paperSell, targetDay);
    else addJsonTrades(out, "btc_options_sell", parseJsonArray(e.options_selling_trades_json), targetDay);
  } else {
    const paperBuy = await loadPaperDesk(client, "buy");
    addJsonTrades(out, "btc_options_buy", paperBuy, targetDay);
    const paperSell = await loadPaperDesk(client, "sell");
    addJsonTrades(out, "btc_options_sell", paperSell, targetDay);
  }

  return out;
}

export async function insertArchiveRows(client: PoolClient, rows: ArchiveRow[]): Promise<number> {
  let inserted = 0;
  for (const r of rows) {
    const res = await client.query(
      `INSERT INTO trade_history_archive (trade_day, module, trade_key, payload)
       VALUES ($1::date, $2, $3, $4::jsonb)
       ON CONFLICT (trade_day, module, trade_key) DO NOTHING
       RETURNING id`,
      [r.trade_day, r.module, r.trade_key, JSON.stringify(r.payload)],
    );
    inserted += res.rowCount ?? 0;
  }
  return inserted;
}

export async function runDailyArchive(tradeDay: string): Promise<{ inserted: number; scanned: number }> {
  const db = getTradeHistoryPool();
  const client = await db.connect();
  try {
    await ensureArchiveTable(client);
    const rows = await collectRowsForDay(client, tradeDay);
    const inserted = await insertArchiveRows(client, rows);
    return { inserted, scanned: rows.length };
  } finally {
    client.release();
  }
}

export async function listArchiveDays(limit = 366): Promise<string[]> {
  const db = getTradeHistoryPool();
  const client = await db.connect();
  try {
    await ensureArchiveTable(client);
    const { rows } = await client.query<{ d: string }>(
      `SELECT trade_day::text AS d FROM trade_history_archive
       GROUP BY trade_day
       ORDER BY trade_day DESC
       LIMIT $1`,
      [limit],
    );
    return rows.map((r) => r.d);
  } finally {
    client.release();
  }
}

export async function loadArchiveForDay(tradeDay: string): Promise<ArchiveRow[]> {
  const db = getTradeHistoryPool();
  const client = await db.connect();
  try {
    await ensureArchiveTable(client);
    const { rows } = await client.query<{
      trade_day: string;
      module: string;
      trade_key: string;
      payload: Record<string, unknown>;
    }>(
      `SELECT trade_day::text AS trade_day, module, trade_key, payload
       FROM trade_history_archive
       WHERE trade_day = $1::date
       ORDER BY module ASC, trade_key ASC`,
      [tradeDay],
    );
    return rows.map((r) => ({
      trade_day: r.trade_day,
      module: r.module,
      trade_key: r.trade_key,
      payload: r.payload,
    }));
  } finally {
    client.release();
  }
}

function pickStrategy(p: Record<string, unknown>): string {
  const v = p.strategyName ?? p.strategy_name;
  return typeof v === "string" ? v : "";
}

function pickSymbol(p: Record<string, unknown>): string {
  const v = p.symbol ?? p.commodityId ?? p.commodityName ?? p.strike;
  if (v == null) return "";
  return String(v);
}

function pickSide(p: Record<string, unknown>): string {
  const v = p.side ?? p.optionType;
  return v == null ? "" : String(v);
}

function pickNetPnl(p: Record<string, unknown>): string | number {
  const v = p.netPnl ?? p.net_pnl;
  if (typeof v === "number" && Number.isFinite(v)) return v;
  if (typeof v === "string") return v;
  return "";
}

function pickExitIso(p: Record<string, unknown>): string {
  const v = p.exitTime ?? p.exit_time;
  if (v instanceof Date) return v.toISOString();
  if (typeof v === "string") return v;
  if (typeof v === "number") return new Date(v < 1e12 ? v * 1000 : v).toISOString();
  return "";
}

export async function buildWorkbookForDay(tradeDay: string): Promise<Buffer> {
  const rows = await loadArchiveForDay(tradeDay);
  const wb = new ExcelJS.Workbook();
  wb.creator = "RAIG Trading Workspace";
  wb.created = new Date();
  const ws = wb.addWorksheet("Trades", {
    views: [{ state: "frozen", ySplit: 1 }],
  });
  ws.columns = [
    { header: "Module", key: "module", width: 22 },
    { header: "Trade key", key: "trade_key", width: 28 },
    { header: "Exit (UTC)", key: "exit_utc", width: 24 },
    { header: "Strategy", key: "strategy", width: 28 },
    { header: "Symbol / instrument", key: "symbol", width: 22 },
    { header: "Side / type", key: "side", width: 14 },
    { header: "Net PnL", key: "net_pnl", width: 14 },
    { header: "Payload (JSON)", key: "json", width: 80 },
  ];
  for (const r of rows) {
    ws.addRow({
      module: r.module,
      trade_key: r.trade_key,
      exit_utc: pickExitIso(r.payload),
      strategy: pickStrategy(r.payload),
      symbol: pickSymbol(r.payload),
      side: pickSide(r.payload),
      net_pnl: pickNetPnl(r.payload),
      json: JSON.stringify(r.payload),
    });
  }
  ws.getRow(1).font = { bold: true };
  const buf = await wb.xlsx.writeBuffer();
  if (Buffer.isBuffer(buf)) return buf;
  return Buffer.from(new Uint8Array(buf as ArrayBuffer));
}
