/**
 * The paper desk: one manage-then-scan cycle, and the reporting over it.
 *
 * ════════════════════════════════════════════════════════════════════════════
 * THE HARD PROBLEM, AND HOW IT IS SOLVED
 * ════════════════════════════════════════════════════════════════════════════
 *
 * A paper desk needs a clock. This one cannot have a scheduled tick: Vercel
 * caps this project at TWO cron jobs, both slots are already taken by the mock
 * trading tick and the policy snapshot, and the platform silently breaks
 * webhooks when a third is added. So the desk ticks ON READ — opening the tab,
 * or any paper endpoint, runs a cycle first.
 *
 * That is normally a fudge, because a desk nobody looks at for three days would
 * mark its positions at a price three days late and record a stop that "should"
 * have hit as an exit at today's price instead. The exit would be fiction.
 *
 * IT IS NOT A FUDGE HERE, because positions are not managed against the current
 * price. They are managed by REPLAYING THE BARS that elapsed since the desk
 * last looked. A stop touched two days ago is found in the bar that touched it,
 * closed at that level, and stamped with that bar's time. Sparse ticks delay
 * DISCOVERY, not correctness — the trade log is the same whether the page was
 * opened hourly or once a week.
 *
 * The one thing that does degrade is funding: it accrues at the rate observed
 * when the desk next looks, not the rate that was live at each settlement in
 * between. That is stated on the page rather than hidden, and it is a second
 * order effect next to getting the exit price right.
 *
 * ════════════════════════════════════════════════════════════════════════════
 * WHAT A BACKTEST USUALLY LIES ABOUT, AND WHAT IS DONE INSTEAD
 * ════════════════════════════════════════════════════════════════════════════
 *
 * GAPS. Filling a stop at the stop level after price has already jumped past it
 * is the most common way a paper record flatters itself. When a bar OPENS
 * beyond the level, the fill is the open — that is what a stop order does — and
 * the trade is flagged as gapped.
 *
 * SAME-BAR STOP AND TARGET. When one bar's range contains both, OHLC alone
 * cannot say which came first, and quietly picking the target is how a strategy
 * invents a win rate. This desk drops to 1-minute bars for that single window
 * and finds out. If the finer bars are unavailable it assumes the STOP came
 * first — the unfavourable branch — and records that it assumed.
 *
 * THE ENTRY BAR is excluded from the replay: its range includes price action
 * from before the position existed, and reading a stop out of it would close a
 * trade on a move that happened before it opened. The cost is that a stop
 * touched and recovered within the first five minutes is not seen. That is a
 * real, bounded optimism and it is written down rather than papered over.
 *
 * COSTS ARE CHARGED IN FULL: the taker fee on both legs, half the quoted spread
 * as slippage on both legs, and every 8-hour funding settlement the position
 * was open across, signed by direction so a short RECEIVES funding when longs
 * are paying.
 */

import { fetchBarsBetween, type Bar } from "../delta";
import { getSnapshot, type ScreenerRow } from "../universe";
import { PERP_TAKER_FEE_RATE } from "../derivatives";
import { collectSignals, type Signal } from "./signals";
import {
  BOOK_STARTING_EQUITY_USD,
  halfSpreadFraction,
  sizePosition,
  slipped,
} from "./sizing";
import {
  acquireTickLease,
  applyTradeToBook,
  books,
  ensureBook,
  FAMILIES,
  FAMILY_LABELS,
  FAMILY_MAX_HOLD_HOURS,
  paperConfigured,
  PaperUnavailableError,
  positions,
  readState,
  releaseTickLease,
  resetAll,
  trades,
  type BookDoc,
  type PositionDoc,
  type SignalFamily,
  type TradeDoc,
} from "./store";

/** Minimum gap between automatic cycles. Manual runs bypass it. */
export const TICK_MIN_INTERVAL_MS = 60_000;

/** How long one container may hold the tick lease before it expires. */
const TICK_LEASE_MS = 150_000;

/** Ceiling on concurrently open positions across every book. */
export const MAX_OPEN_TOTAL = 60;

/** Ceiling per symbol, so one book cannot commit all its equity to one theme. */
export const MAX_OPEN_PER_SYMBOL = 2;

/** How many new positions one cycle may open. Bounds a cold start's cost. */
const MAX_OPENS_PER_CYCLE = 12;

/** Funding settles every 8 hours, at 00:00 / 08:00 / 16:00 UTC. */
const FUNDING_INTERVAL_SEC = 8 * 3_600;

function nowSec(): number {
  return Math.floor(Date.now() / 1000);
}

/** Settlements strictly after `fromSec` and at or before `toSec`. */
export function fundingSettlementsBetween(fromSec: number, toSec: number): number {
  if (!(toSec > fromSec)) return 0;
  return Math.floor(toSec / FUNDING_INTERVAL_SEC) - Math.floor(fromSec / FUNDING_INTERVAL_SEC);
}

/**
 * Bar resolution for replaying a window.
 *
 * 5-minute bars for anything up to about ten days, which is every hold this
 * desk takes; the venue returns roughly 4,000 bars per request, and 10 days of
 * 5m is 2,880. Longer windows step down to 15m rather than silently receiving a
 * truncated series — a truncated series would look like a quiet market and the
 * position would never find its exit.
 */
export function replayResolution(windowSec: number): string {
  if (windowSec <= 10 * 86_400) return "5m";
  if (windowSec <= 40 * 86_400) return "15m";
  return "1h";
}

export type ExitHit = {
  reason: "TARGET" | "STOP";
  /** The level that triggered, before slippage. */
  level: number;
  /** Where it actually filled — the level, or the bar's open when price gapped past it. */
  fill: number;
  at: number;
  gapped: boolean;
  ambiguous: boolean;
  resolvedBy: "1m-bars" | "assumed-stop-first" | null;
};

/**
 * Walk one bar and decide whether it closed the position.
 *
 * Returns null when the bar did neither. The gap branch is what separates this
 * from a naive backtest: a bar that OPENS past the level filled at the open,
 * not at the level.
 */
export function evaluateBar(
  bar: Bar,
  side: "long" | "short",
  stop: number,
  target: number,
): { stopHit: boolean; targetHit: boolean; stopFill: number; targetFill: number } {
  if (side === "long") {
    const stopHit = bar.low <= stop;
    const targetHit = bar.high >= target;
    return {
      stopHit,
      targetHit,
      // Gapped through: the order fills at the open, which is worse than the stop.
      stopFill: bar.open <= stop ? bar.open : stop,
      // A favourable gap fills better than the target; taking the open is honest
      // in both directions.
      targetFill: bar.open >= target ? bar.open : target,
    };
  }
  const stopHit = bar.high >= stop;
  const targetHit = bar.low <= target;
  return {
    stopHit,
    targetHit,
    stopFill: bar.open >= stop ? bar.open : stop,
    targetFill: bar.open <= target ? bar.open : target,
  };
}

/**
 * Which of the two came first inside one bar, using 1-minute bars.
 *
 * Called only when a single replay bar contains both levels. Returns null when
 * the finer series is unavailable, and the caller then assumes the stop — the
 * unfavourable branch — rather than picking the outcome that looks better.
 */
async function resolveSameBar(
  symbol: string,
  bar: Bar,
  side: "long" | "short",
  stop: number,
  target: number,
  bucketSec: number,
): Promise<{ reason: "TARGET" | "STOP"; level: number; fill: number; at: number } | null> {
  let fine: Bar[];
  try {
    fine = await fetchBarsBetween(symbol, "1m", bar.ts, bar.ts + bucketSec);
  } catch {
    return null;
  }
  if (fine.length === 0) return null;
  for (const m of fine) {
    if (m.ts < bar.ts || m.ts >= bar.ts + bucketSec) continue;
    const e = evaluateBar(m, side, stop, target);
    // Still ambiguous inside a single minute: fall through to the caller's
    // conservative assumption rather than guessing again at finer resolution.
    if (e.stopHit && e.targetHit) return null;
    if (e.stopHit) return { reason: "STOP", level: stop, fill: e.stopFill, at: m.ts };
    if (e.targetHit) return { reason: "TARGET", level: target, fill: e.targetFill, at: m.ts };
  }
  return null;
}

/** Replay the window since the last check and find the exit, if there was one. */
export async function findExit(
  pos: PositionDoc,
  toSec: number,
): Promise<{ hit: ExitHit | null; checkedTo: number }> {
  const from = Math.max(pos.checked_to, pos.opened_at);
  if (toSec <= from + 60) return { hit: null, checkedTo: pos.checked_to };

  const res = replayResolution(toSec - from);
  const bucket = res === "5m" ? 300 : res === "15m" ? 900 : 3_600;

  let bars: Bar[];
  try {
    bars = await fetchBarsBetween(pos.symbol, res, from, toSec);
  } catch {
    // A feed failure must not close a position at a guessed price. The window
    // stays unchecked and is replayed on the next cycle.
    return { hit: null, checkedTo: pos.checked_to };
  }

  // Only bars that OPEN at or after the entry. The entry bar's range covers
  // price action from before the position existed.
  const eligible = bars.filter((b) => b.ts >= pos.opened_at && b.ts <= toSec);

  for (const bar of eligible) {
    const e = evaluateBar(bar, pos.side, pos.stop, pos.target);
    if (!e.stopHit && !e.targetHit) continue;

    if (e.stopHit && e.targetHit) {
      const finer = await resolveSameBar(pos.symbol, bar, pos.side, pos.stop, pos.target, bucket);
      if (finer) {
        return {
          hit: {
            reason: finer.reason,
            level: finer.level,
            fill: finer.fill,
            at: finer.at,
            gapped: finer.fill !== finer.level,
            ambiguous: true,
            resolvedBy: "1m-bars",
          },
          checkedTo: toSec,
        };
      }
      return {
        hit: {
          reason: "STOP",
          level: pos.stop,
          fill: e.stopFill,
          at: bar.ts,
          gapped: e.stopFill !== pos.stop,
          ambiguous: true,
          resolvedBy: "assumed-stop-first",
        },
        checkedTo: toSec,
      };
    }

    if (e.stopHit) {
      return {
        hit: {
          reason: "STOP",
          level: pos.stop,
          fill: e.stopFill,
          at: bar.ts,
          gapped: e.stopFill !== pos.stop,
          ambiguous: false,
          resolvedBy: null,
        },
        checkedTo: toSec,
      };
    }
    return {
      hit: {
        reason: "TARGET",
        level: pos.target,
        fill: e.targetFill,
        at: bar.ts,
        gapped: e.targetFill !== pos.target,
        ambiguous: false,
        resolvedBy: null,
      },
      checkedTo: toSec,
    };
  }

  return { hit: null, checkedTo: toSec };
}

/** Funding owed since `pos.funding_to`, signed from this position's own side. */
function accrueFunding(pos: PositionDoc, row: ScreenerRow | undefined, toSec: number): number {
  const rate = row?.funding.ratePct8h;
  if (rate === null || rate === undefined) return 0;
  const n = fundingSettlementsBetween(pos.funding_to, toSec);
  if (n <= 0) return 0;
  // Positive funding means longs pay shorts. A short is therefore CREDITED, and
  // clamping that to zero would misprice every position on the discount side of
  // this venue.
  const signed = pos.side === "long" ? 1 : -1;
  return (pos.notional_usd * (rate / 100) * n) * signed;
}

async function closePosition(
  pos: PositionDoc,
  hit: { reason: TradeDoc["exit_reason"]; level: number; fill: number; at: number; gapped: boolean; ambiguous: boolean; resolvedBy: TradeDoc["ambiguity_resolved_by"] },
  row: ScreenerRow | undefined,
  fundingUsd: number,
): Promise<TradeDoc> {
  const halfSpread = row ? halfSpreadFraction(row) : 0.0025;
  const exitFill = slipped(hit.fill, pos.side, false, halfSpread);
  const exitSlippageUsd = Math.abs(exitFill - hit.fill) * pos.quantity;
  const exitNotional = exitFill * pos.quantity;
  const exitFee = exitNotional * PERP_TAKER_FEE_RATE;

  const gross =
    pos.side === "long"
      ? (exitFill - pos.entry) * pos.quantity
      : (pos.entry - exitFill) * pos.quantity;

  const totalFunding = pos.funding_usd + fundingUsd;
  const costs = pos.entry_fee_usd + exitFee + totalFunding;
  const net = gross - costs;

  const trade: TradeDoc = {
    ...pos,
    status: "CLOSED",
    exit: r(exitFill, 8),
    exit_level: r(hit.level, 8),
    exit_reason: hit.reason,
    closed_at: hit.at,
    exit_fee_usd: r(exitFee, 4),
    exit_slippage_usd: r(exitSlippageUsd, 4),
    funding_usd: r(totalFunding, 4),
    funding_to: hit.at,
    gross_pnl_usd: r(gross, 4),
    costs_usd: r(costs, 4),
    net_pnl_usd: r(net, 4),
    return_pct: r((net / Math.max(1e-9, pos.margin_usd)) * 100, 3),
    // R is measured against the risk the position was SIZED to, so it is
    // comparable across contracts with wildly different stop widths — which is
    // the entire reason this desk sizes by risk rather than by ticket.
    r_multiple: r(net / Math.max(1e-9, pos.risk_usd), 3),
    hold_hours: r((hit.at - pos.opened_at) / 3_600, 2),
    same_bar_ambiguity: hit.ambiguous,
    ambiguity_resolved_by: hit.resolvedBy,
  };

  const t = await trades();
  await t.insertOne(trade);
  const p = await positions();
  await p.deleteOne({ position_id: pos.position_id });
  await applyTradeToBook(pos.symbol, trade.net_pnl_usd);
  return trade;
}

function r(v: number, dp: number): number {
  if (!Number.isFinite(v)) return 0;
  const f = 10 ** dp;
  return Math.round(v * f) / f;
}

// ── the cycle ───────────────────────────────────────────────────────────────

export type CycleResult = {
  ran: boolean;
  skippedReason?: string;
  managed: number;
  closed: number;
  opened: number;
  refusals: { reason: string; n: number }[];
  elapsedMs: number;
};

/**
 * One manage-then-scan cycle. Manage FIRST, so equity freed by a close is
 * available to the scan in the same cycle rather than a tick later.
 */
export async function runCycle(force = false): Promise<CycleResult> {
  const started = Date.now();
  if (!paperConfigured()) {
    throw new PaperUnavailableError(
      "MONGODB_URI is not set on this deployment, so the paper desk has nowhere to keep positions.",
    );
  }

  const st = await readState();
  if (!force && st && Date.now() - st.last_tick_at < TICK_MIN_INTERVAL_MS) {
    return {
      ran: false,
      skippedReason: `last cycle ran ${Math.round((Date.now() - st.last_tick_at) / 1000)}s ago`,
      managed: 0,
      closed: 0,
      opened: 0,
      refusals: [],
      elapsedMs: 0,
    };
  }

  if (!(await acquireTickLease(TICK_LEASE_MS))) {
    return {
      ran: false,
      skippedReason: "another request is running the cycle right now",
      managed: 0,
      closed: 0,
      opened: 0,
      refusals: [],
      elapsedMs: 0,
    };
  }

  let closed = 0;
  let opened = 0;
  let managed = 0;
  const refusals = new Map<string, number>();
  const note = (reason: string) => refusals.set(reason, (refusals.get(reason) ?? 0) + 1);
  let lastError: string | null = null;

  try {
    const snap = await getSnapshot();
    const bySymbol = new Map(snap.rows.map((x) => [x.symbol, x]));
    const to = nowSec();

    // ── manage ────────────────────────────────────────────────────────────
    const p = await positions();
    const open = await p.find({}).toArray();
    managed = open.length;

    // The replay is the expensive half of a cycle: one bar request per open
    // position. Run them with bounded concurrency rather than in sequence —
    // sixty positions at roughly 300ms each is eighteen seconds serially, which
    // on top of a cold snapshot would crowd the function's 60s ceiling and
    // truncate the cycle mid-manage. The WRITES stay sequential below, because
    // they are fast and ordering them keeps book updates easy to reason about.
    const exits = new Map<string, { hit: ExitHit | null; checkedTo: number }>();
    {
      let cursor = 0;
      const workers = Array.from({ length: Math.min(8, open.length) }, async () => {
        for (;;) {
          const i = cursor++;
          if (i >= open.length) return;
          const pos = open[i]!;
          exits.set(pos.position_id, await findExit(pos, to));
        }
      });
      await Promise.all(workers);
    }

    for (const pos of open) {
      const row = bySymbol.get(pos.symbol);
      const { hit, checkedTo } = exits.get(pos.position_id) ?? { hit: null, checkedTo: pos.checked_to };
      const fundingUsd = accrueFunding(pos, row, hit ? hit.at : to);

      if (hit) {
        await closePosition(pos, hit, row, fundingUsd);
        closed++;
        continue;
      }

      // Time stop. Measured from the position's own clock, not from the tick's,
      // so a desk that went unread for a week still closes the trade at the
      // deadline rather than a week late.
      const ageHours = (to - pos.opened_at) / 3_600;
      if (ageHours >= pos.max_hold_hours) {
        const deadline = pos.opened_at + pos.max_hold_hours * 3_600;
        const price = await priceAt(pos.symbol, deadline, row);
        if (price !== null) {
          await closePosition(
            pos,
            {
              reason: "TIME",
              level: price,
              fill: price,
              at: deadline,
              gapped: false,
              ambiguous: false,
              resolvedBy: null,
            },
            row,
            accrueFunding(pos, row, deadline),
          );
          closed++;
          continue;
        }
      }

      await p.updateOne(
        { position_id: pos.position_id },
        {
          $set: {
            checked_to: checkedTo,
            funding_usd: r(pos.funding_usd + fundingUsd, 4),
            funding_to: to,
            ts: Date.now(),
          },
        },
      );
    }

    // ── scan ──────────────────────────────────────────────────────────────
    const stillOpen = await p.countDocuments({});
    let room = Math.min(MAX_OPEN_TOTAL - stillOpen, MAX_OPENS_PER_CYCLE);
    if (room > 0) {
      const { signals, skipped } = await collectSignals();
      for (const [k, v] of skipped) refusals.set(k, (refusals.get(k) ?? 0) + v);

      const openBySymbol = new Map<string, number>();
      const openKeys = new Set<string>();
      for (const o of await p.find({}, { projection: { symbol: 1, family: 1 } }).toArray()) {
        openBySymbol.set(o.symbol, (openBySymbol.get(o.symbol) ?? 0) + 1);
        openKeys.add(`${o.symbol}:${o.family}`);
      }

      for (const sig of signals) {
        if (room <= 0) break;
        if (openKeys.has(`${sig.symbol}:${sig.family}`)) {
          note("already holding this family on this contract");
          continue;
        }
        if ((openBySymbol.get(sig.symbol) ?? 0) >= MAX_OPEN_PER_SYMBOL) {
          note(`already holding ${MAX_OPEN_PER_SYMBOL} positions on this contract`);
          continue;
        }
        const row = bySymbol.get(sig.symbol);
        if (!row) {
          note("contract left the universe between the board and the fill");
          continue;
        }

        const outcome = await openPosition(sig, row);
        if (outcome.ok) {
          opened++;
          room--;
          openKeys.add(`${sig.symbol}:${sig.family}`);
          openBySymbol.set(sig.symbol, (openBySymbol.get(sig.symbol) ?? 0) + 1);
        } else {
          note(outcome.reason);
        }
      }
    } else if (stillOpen >= MAX_OPEN_TOTAL) {
      note(`desk is at its ${MAX_OPEN_TOTAL}-position ceiling`);
    }
  } catch (e) {
    lastError = e instanceof Error ? e.message : String(e);
  } finally {
    await releaseTickLease({
      last_tick_at: Date.now(),
      last_tick_ms: Date.now() - started,
      last_opened: opened,
      last_closed: closed,
      last_error: lastError,
    });
    if (opened || closed) {
      const s = await (await import("./store")).state();
      await s.updateOne(
        { _id: "crypto_screener_paper" },
        { $inc: { opened_total: opened, closed_total: closed } },
      );
    }
  }

  return {
    ran: true,
    managed,
    closed,
    opened,
    refusals: [...refusals.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 12)
      .map(([reason, n]) => ({ reason, n })),
    elapsedMs: Date.now() - started,
  };
}

/** The price at a past instant, from bars. Used for the TIME exit. */
async function priceAt(symbol: string, atSec: number, row: ScreenerRow | undefined): Promise<number | null> {
  try {
    const bars = await fetchBarsBetween(symbol, "5m", atSec - 1_800, atSec + 1_800);
    // The last bar that had CLOSED by the deadline: closing "at the deadline"
    // must not use a bar that had not finished forming then.
    const usable = bars.filter((b) => b.ts + 300 <= atSec);
    if (usable.length > 0) return usable[usable.length - 1]!.close;
  } catch {
    /* fall through to the live price */
  }
  return row?.price ?? null;
}

type OpenOutcome = { ok: true } | { ok: false; reason: string };

async function openPosition(sig: Signal, row: ScreenerRow): Promise<OpenOutcome> {
  const book = await ensureBook(sig.symbol, BOOK_STARTING_EQUITY_USD);
  const p = await positions();

  const held = await p.find({ symbol: sig.symbol }).toArray();
  const marginPosted = held.reduce((s, h) => s + h.margin_usd, 0);
  const equity = book.starting_equity_usd + book.realised_pnl_usd;
  const available = equity - marginPosted;

  const halfSpread = halfSpreadFraction(row);
  const entryFill = slipped(sig.price, sig.side, true, halfSpread);

  const sizing = sizePosition({
    row,
    side: sig.side,
    entry: entryFill,
    stop: sig.stop,
    availableEquityUsd: available,
    bookEquityUsd: equity,
  });
  if (!sizing.ok) return { ok: false, reason: sizing.reason };

  const entrySlippageUsd = Math.abs(entryFill - sig.price) * sizing.quantity;
  const entryFee = sizing.notionalUsd * PERP_TAKER_FEE_RATE;
  const now = nowSec();

  const doc: PositionDoc = {
    position_id: `${sig.symbol}:${sig.family}:${now}`,
    symbol: sig.symbol,
    family: sig.family,
    side: sig.side,
    status: "OPEN",
    entry: r(entryFill, 8),
    signal_price: r(sig.price, 8),
    stop: r(sig.stop, 8),
    target: r(sig.target, 8),
    liquidation_price: sizing.liquidationPrice,
    contracts: sizing.contracts,
    quantity: sizing.quantity,
    notional_usd: sizing.notionalUsd,
    leverage: sizing.leverage,
    margin_usd: sizing.marginUsd,
    risk_usd: sizing.riskUsd,
    maintenance_margin_pct: sizing.maintenanceMarginPct,
    entry_fee_usd: r(entryFee, 4),
    entry_slippage_usd: r(entrySlippageUsd, 4),
    funding_usd: 0,
    funding_to: now,
    opened_at: now,
    checked_to: now,
    max_hold_hours: sig.maxHoldHours,
    signal_reason: sig.reason,
    signal_chips: sig.chips.slice(0, 4),
    pattern: sig.pattern,
    net_rr_at_entry: sig.netRr,
    ts: Date.now(),
  };

  try {
    await p.insertOne(doc);
    return { ok: true };
  } catch (e) {
    // The partial unique index rejected a duplicate — another container opened
    // the same idea between the pre-check and the insert. Not an error.
    if (e && typeof e === "object" && "code" in e && (e as { code: number }).code === 11000) {
      return { ok: false, reason: "another cycle opened this position first" };
    }
    return { ok: false, reason: e instanceof Error ? e.message : "insert failed" };
  }
}

/** Run a cycle unless one ran recently. Swallows failures — a read must still serve. */
export async function maybeTick(): Promise<CycleResult | { ran: false; skippedReason: string; managed: 0; closed: 0; opened: 0; refusals: []; elapsedMs: 0 }> {
  try {
    return await runCycle(false);
  } catch (e) {
    return {
      ran: false,
      skippedReason: e instanceof Error ? e.message : "cycle failed",
      managed: 0,
      closed: 0,
      opened: 0,
      refusals: [],
      elapsedMs: 0,
    };
  }
}

// ── reporting ───────────────────────────────────────────────────────────────

function stats(rows: TradeDoc[]) {
  const wins = rows.filter((t) => t.net_pnl_usd > 0);
  const losses = rows.filter((t) => t.net_pnl_usd <= 0);
  const net = rows.reduce((s, t) => s + t.net_pnl_usd, 0);
  const grossWin = wins.reduce((s, t) => s + t.net_pnl_usd, 0);
  const grossLoss = Math.abs(losses.reduce((s, t) => s + t.net_pnl_usd, 0));
  return {
    trades: rows.length,
    wins: wins.length,
    losses: losses.length,
    winRate: rows.length ? r((wins.length / rows.length) * 100, 1) : null,
    netPnlUsd: r(net, 2),
    grossPnlUsd: r(rows.reduce((s, t) => s + t.gross_pnl_usd, 0), 2),
    costsUsd: r(rows.reduce((s, t) => s + t.costs_usd, 0), 2),
    fundingUsd: r(rows.reduce((s, t) => s + t.funding_usd, 0), 2),
    // Profit factor and expectancy are the two that actually rank a strategy.
    // Win rate alone ranks a martingale first.
    profitFactor: grossLoss > 0 ? r(grossWin / grossLoss, 2) : null,
    expectancyUsd: rows.length ? r(net / rows.length, 2) : null,
    avgR: rows.length ? r(rows.reduce((s, t) => s + t.r_multiple, 0) / rows.length, 2) : null,
    bestUsd: rows.length ? r(Math.max(...rows.map((t) => t.net_pnl_usd)), 2) : null,
    worstUsd: rows.length ? r(Math.min(...rows.map((t) => t.net_pnl_usd)), 2) : null,
    avgHoldHours: rows.length ? r(rows.reduce((s, t) => s + t.hold_hours, 0) / rows.length, 1) : null,
  };
}

export async function summary(tick = true) {
  const cycle = tick ? await maybeTick() : null;
  const [p, t, b, st, snap] = await Promise.all([
    positions(),
    trades(),
    books(),
    readState(),
    getSnapshot().catch(() => null),
  ]);

  const [openDocs, tradeDocs, bookDocs] = await Promise.all([
    p.find({}).toArray(),
    t.find({}).sort({ closed_at: -1 }).limit(2_000).toArray(),
    b.find({}).toArray(),
  ]);

  const priceBy = new Map((snap?.rows ?? []).map((x) => [x.symbol, x]));

  // Open positions are marked to the live price, and the mark is NET of what it
  // would cost to close — the exit taker fee, the spread, and the funding
  // already accrued. An unrealised figure quoted gross is the number that makes
  // a desk look profitable right up until it closes something.
  const marked = openDocs.map((o) => {
    const row = priceBy.get(o.symbol);
    const px = row?.price ?? null;
    if (px === null) return { ...o, mark: null, unrealisedUsd: null, unrealisedPct: null };
    const halfSpread = row ? halfSpreadFraction(row) : 0.0025;
    const exitFill = slipped(px, o.side, false, halfSpread);
    const gross =
      o.side === "long" ? (exitFill - o.entry) * o.quantity : (o.entry - exitFill) * o.quantity;
    const exitFee = exitFill * o.quantity * PERP_TAKER_FEE_RATE;
    const net = gross - o.entry_fee_usd - exitFee - o.funding_usd;
    return {
      ...o,
      mark: px,
      unrealisedUsd: r(net, 2),
      unrealisedPct: r((net / Math.max(1e-9, o.margin_usd)) * 100, 2),
    };
  });

  const openBySymbol = new Map<string, typeof marked>();
  for (const m of marked) {
    const l = openBySymbol.get(m.symbol) ?? [];
    l.push(m);
    openBySymbol.set(m.symbol, l);
  }

  const bookRows = bookDocs
    .map((bk: BookDoc) => {
      const mine = tradeDocs.filter((x) => x.symbol === bk.symbol);
      const s = stats(mine);
      const openHere = openBySymbol.get(bk.symbol) ?? [];
      const unreal = openHere.reduce((acc, x) => acc + (x.unrealisedUsd ?? 0), 0);
      const equity = bk.starting_equity_usd + bk.realised_pnl_usd;
      return {
        symbol: bk.symbol,
        sectorLabel: priceBy.get(bk.symbol)?.sectorLabel ?? null,
        startingEquityUsd: bk.starting_equity_usd,
        realisedPnlUsd: r(bk.realised_pnl_usd, 2),
        unrealisedPnlUsd: r(unreal, 2),
        equityUsd: r(equity, 2),
        markToMarketUsd: r(equity + unreal, 2),
        roiPct: r((bk.realised_pnl_usd / bk.starting_equity_usd) * 100, 2),
        openPositions: openHere.length,
        marginPostedUsd: r(openHere.reduce((acc, x) => acc + x.margin_usd, 0), 2),
        ...s,
      };
    })
    .sort((a, b2) => b2.markToMarketUsd - a.markToMarketUsd);

  const familyRows = FAMILIES.map((f) => {
    const mine = tradeDocs.filter((x) => x.family === f);
    const openHere = marked.filter((x) => x.family === f);
    return {
      family: f,
      label: FAMILY_LABELS[f],
      maxHoldHours: FAMILY_MAX_HOLD_HOURS[f],
      openPositions: openHere.length,
      unrealisedPnlUsd: r(openHere.reduce((acc, x) => acc + (x.unrealisedUsd ?? 0), 0), 2),
      ...stats(mine),
    };
  }).sort((a, b2) => (b2.netPnlUsd ?? 0) - (a.netPnlUsd ?? 0));

  const all = stats(tradeDocs);
  const totalStarting = bookDocs.reduce((s, x) => s + x.starting_equity_usd, 0);
  const totalRealised = bookDocs.reduce((s, x) => s + x.realised_pnl_usd, 0);
  const totalUnrealised = marked.reduce((s, x) => s + (x.unrealisedUsd ?? 0), 0);

  const ambiguous = tradeDocs.filter((x) => x.same_bar_ambiguity).length;
  const assumed = tradeDocs.filter((x) => x.ambiguity_resolved_by === "assumed-stop-first").length;
  const gapped = tradeDocs.filter((x) => x.exit !== x.exit_level && x.exit_reason !== "TIME").length;

  return {
    configured: true,
    books: bookRows,
    families: familyRows,
    totals: {
      booksOpened: bookDocs.length,
      startingEquityUsd: totalStarting,
      realisedPnlUsd: r(totalRealised, 2),
      unrealisedPnlUsd: r(totalUnrealised, 2),
      equityUsd: r(totalStarting + totalRealised, 2),
      markToMarketUsd: r(totalStarting + totalRealised + totalUnrealised, 2),
      roiPct: totalStarting > 0 ? r((totalRealised / totalStarting) * 100, 2) : null,
      openPositions: openDocs.length,
      ...all,
    },
    perSymbolEquityUsd: BOOK_STARTING_EQUITY_USD,
    maxOpenTotal: MAX_OPEN_TOTAL,
    maxOpenPerSymbol: MAX_OPEN_PER_SYMBOL,
    integrity: {
      sameBarAmbiguities: ambiguous,
      assumedStopFirst: assumed,
      gappedFills: gapped,
      note:
        "A same-bar ambiguity is a trade whose stop and target both fell inside one replay bar. " +
        "The desk drops to 1-minute bars to find out which came first; where even that is " +
        "unavailable it assumes the STOP — the unfavourable branch — and counts it here. Gapped " +
        "fills are exits where price opened beyond the level, so the fill is the open rather than " +
        "the level, which is what a stop order actually does.",
    },
    tick: {
      lastTickAt: st?.last_tick_at ?? null,
      lastTickMs: st?.last_tick_ms ?? null,
      ticks: st?.ticks ?? 0,
      lastOpened: st?.last_opened ?? 0,
      lastClosed: st?.last_closed ?? 0,
      lastError: st?.last_error ?? null,
      thisRequest: cycle,
      note:
        "This desk has no scheduled tick — the project's two Vercel cron slots are already taken, " +
        "so a cycle runs when the page is read. That is safe because positions are managed by " +
        "REPLAYING the bars since the last check, not by comparing the current price to a stop: a " +
        "stop touched two days ago is found in the bar that touched it and closed at that level, " +
        "at that time. Sparse reads delay discovery, not correctness. The exception is funding, " +
        "which accrues at the rate observed when the desk next looks.",
    },
  };
}

export async function listPositions(status: "OPEN" | "CLOSED", family: string | null, symbol: string | null, limit: number) {
  await maybeTick();
  const priceBy = new Map(((await getSnapshot().catch(() => null))?.rows ?? []).map((x) => [x.symbol, x]));

  if (status === "OPEN") {
    const p = await positions();
    const q: Record<string, unknown> = {};
    if (family) q.family = family;
    if (symbol) q.symbol = symbol.toUpperCase();
    const rows = await p.find(q).sort({ opened_at: -1 }).limit(limit).toArray();
    return {
      status,
      count: rows.length,
      rows: rows.map((o) => {
        const row = priceBy.get(o.symbol);
        const px = row?.price ?? null;
        const halfSpread = row ? halfSpreadFraction(row) : 0.0025;
        let unrealised: number | null = null;
        if (px !== null) {
          const exitFill = slipped(px, o.side, false, halfSpread);
          const gross =
            o.side === "long" ? (exitFill - o.entry) * o.quantity : (o.entry - exitFill) * o.quantity;
          unrealised = r(gross - o.entry_fee_usd - exitFill * o.quantity * PERP_TAKER_FEE_RATE - o.funding_usd, 2);
        }
        return {
          ...o,
          _id: undefined,
          familyLabel: FAMILY_LABELS[o.family],
          sectorLabel: row?.sectorLabel ?? null,
          mark: px,
          unrealisedUsd: unrealised,
          unrealisedPct: unrealised === null ? null : r((unrealised / Math.max(1e-9, o.margin_usd)) * 100, 2),
          toTargetPct: px ? r((o.target / px - 1) * 100, 2) : null,
          toStopPct: px ? r((o.stop / px - 1) * 100, 2) : null,
          ageHours: r((nowSec() - o.opened_at) / 3_600, 1),
        };
      }),
    };
  }

  const t = await trades();
  const q: Record<string, unknown> = {};
  if (family) q.family = family;
  if (symbol) q.symbol = symbol.toUpperCase();
  const rows = await t.find(q).sort({ closed_at: -1 }).limit(limit).toArray();
  return {
    status,
    count: rows.length,
    rows: rows.map((x) => ({
      ...x,
      _id: undefined,
      familyLabel: FAMILY_LABELS[x.family],
      sectorLabel: priceBy.get(x.symbol)?.sectorLabel ?? null,
    })),
  };
}

export async function manualRun(): Promise<CycleResult> {
  return runCycle(true);
}

export async function reset(): Promise<{ books: number; positions: number; trades: number }> {
  return resetAll();
}

export { paperConfigured, PaperUnavailableError };
export type { SignalFamily };
