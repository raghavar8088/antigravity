/**
 * Sector rotation across 24h / 7d / 30d / 6 months, and what is driving it.
 *
 * THE TAXONOMY IS THE VENUE'S, NOT OURS. Delta tags each contract — `layer_1`,
 * `defi`, `meme`, `ai`, `gaming`, `nft`, `layer_2`, `xStock`, `metal` — and
 * this board partitions on the primary tag chosen in `universe.ts`. Two things
 * follow and both are stated on the page rather than left for the reader to
 * discover:
 *
 *   The tags OVERLAP. A contract tagged both `layer_1` and `smart_contracts`
 *   appears in exactly one bucket here. The full tag list travels with every
 *   row so the choice can be audited.
 *
 *   Thirty contracts carry NO tag, XRPUSD among them. They form an
 *   Unclassified bucket rather than being distributed into plausible homes. A
 *   sector board that quietly filed XRP under "layer_1" would be inventing the
 *   venue's opinion.
 *
 * TWO WEIGHTINGS, SHOWN SIDE BY SIDE, because they disagree and the
 * disagreement is the useful part. Equal-weighted says what the average
 * contract in the sector did. Turnover-weighted says where the money actually
 * went. A meme sector can be +9% equal-weighted because thirty micro-caps
 * bounced, while turnover-weighted it is +0.4% because all the volume sat in
 * one name that did nothing. Neither number is wrong; showing only one would be.
 *
 * MARKET CAP IS NOT AVAILABLE AND IS NOT INVENTED. Delta publishes turnover and
 * open interest, not circulating supply or float. So the weighted column is
 * labelled as TURNOVER-weighted and never as cap-weighted, which is a different
 * measurement that this app has no way to source.
 */

import * as H from "./horizons";
import type { HorizonKey } from "./horizons";
import { sectorLabel, type ScreenerRow, type Snapshot } from "./universe";

/**
 * A sector needs a few contracts before its mean means anything. Below this the
 * roll-up is still computed but flagged `thin`, so a two-contract "sector"
 * cannot top the board on one token's move.
 */
export const MIN_CONSTITUENTS = 3;
const BROAD_BREADTH_PCT = 70;
const NARROW_TOP_SHARE = 60;

export type SectorRow = {
  sector: string;
  label: string;
  count: number;
  thin: boolean;
  returnPct: number;
  returnWeightedPct: number | null;
  breadthUp: number;
  breadthPct: number;
  rsBenchmark: number | null;
  volumeX: number | null;
  turnoverUsd24h: number;
  oiUsd: number;
  /** Mean funding across the sector — is the whole theme crowded, or one name? */
  medianFundingPct8h: number | null;
  /** Mean 30-day correlation to BTC. A sector at 0.95 is not really a sector. */
  medianBtcCorrelation: number | null;
  leader: { symbol: string; returnPct: number };
  laggard: { symbol: string; returnPct: number };
  rank: number;
};

export type SectorBoard = {
  horizon: HorizonKey;
  horizonLabel: string;
  benchmarkPct: number | null;
  count: number;
  sectors: SectorRow[];
  basis: string;
};

export const SECTOR_BASIS =
  "Delta's own contract tags, partitioned on a primary tag. Returns are the equal-weighted " +
  "mean of constituent returns; the weighted column is TURNOVER-weighted, not cap-weighted — " +
  "this venue publishes no supply or float data, and a cap weighting we cannot source would " +
  "be a fiction.";

function median(values: number[]): number | null {
  if (values.length === 0) return null;
  const s = [...values].sort((a, b) => a - b);
  const mid = Math.floor(s.length / 2);
  return s.length % 2 === 1 ? s[mid]! : (s[mid - 1]! + s[mid]!) / 2;
}

/**
 * Sector board for one horizon, computed from the snapshot.
 *
 * Pure and synchronous — it reads the snapshot the universe already built, so
 * the board, the drill-down and the row drawer agree by construction instead of
 * each recomputing sector returns slightly differently.
 */
export function rollUp(snap: Snapshot, horizon: HorizonKey): SectorBoard {
  const rows = snap.rows.filter((r) => r.returns[horizon] !== null);
  const bench = snap.benchmark.returns[horizon];

  const bySector = new Map<string, ScreenerRow[]>();
  for (const r of rows) {
    const list = bySector.get(r.sector);
    if (list) list.push(r);
    else bySector.set(r.sector, [r]);
  }

  const out: SectorRow[] = [];
  for (const [sector, members] of bySector) {
    const rets = members.map((m) => m.returns[horizon]!);
    const equalWeighted = rets.reduce((s, v) => s + v, 0) / rets.length;

    const weights = members.map((m) => m.turnoverUsd24h ?? 0);
    const totalW = weights.reduce((s, v) => s + v, 0);
    const weighted =
      totalW > 0 ? rets.reduce((s, r, i) => s + r * weights[i]!, 0) / totalW : null;

    const up = rets.filter((r) => r > 0).length;
    const volX = members.map((m) => m.volumeX).filter((v): v is number => v !== null);
    const fund = members
      .map((m) => m.funding.ratePct8h)
      .filter((v): v is number => v !== null);
    const corr = members
      .map((m) => m.btcCorrelation30d)
      .filter((v): v is number => v !== null);

    const ranked = [...members].sort((a, b) => b.returns[horizon]! - a.returns[horizon]!);

    out.push({
      sector,
      label: sectorLabel(sector),
      count: members.length,
      thin: members.length < MIN_CONSTITUENTS,
      returnPct: H.round(equalWeighted)!,
      returnWeightedPct: H.round(weighted),
      breadthUp: up,
      breadthPct: H.round((up / members.length) * 100, 1)!,
      rsBenchmark: H.round(H.relativeStrength(equalWeighted, bench)),
      volumeX: volX.length ? H.round(volX.reduce((s, v) => s + v, 0) / volX.length) : null,
      turnoverUsd24h: totalW,
      oiUsd: members.reduce((s, m) => s + (m.oi.oiValueUsd ?? 0), 0),
      medianFundingPct8h: H.round(median(fund), 5),
      medianBtcCorrelation: H.round(median(corr), 3),
      leader: { symbol: ranked[0]!.symbol, returnPct: ranked[0]!.returns[horizon]! },
      laggard: {
        symbol: ranked[ranked.length - 1]!.symbol,
        returnPct: ranked[ranked.length - 1]!.returns[horizon]!,
      },
      rank: 0,
    });
  }

  out.sort((a, b) => b.returnPct - a.returnPct);
  out.forEach((s, i) => {
    s.rank = i + 1;
  });

  return {
    horizon,
    horizonLabel: H.HORIZON_LABELS[horizon],
    benchmarkPct: bench,
    count: out.length,
    sectors: out,
    basis: SECTOR_BASIS,
  };
}

export type RotationLabel = "leading" | "weakening" | "improving" | "lagging" | "unknown";

export type SectorRotationRow = {
  sector: string;
  label: string;
  count: number;
  thin: boolean;
  returns: Partial<Record<HorizonKey, number>>;
  breadth: Partial<Record<HorizonKey, number>>;
  ranks: Partial<Record<HorizonKey, number>>;
  rs: Partial<Record<HorizonKey, number | null>>;
  leaders: Partial<Record<HorizonKey, { symbol: string; returnPct: number }>>;
  laggards: Partial<Record<HorizonKey, { symbol: string; returnPct: number }>>;
  volumeX: number | null;
  turnoverUsd24h: number;
  oiUsd: number;
  medianFundingPct8h: number | null;
  medianBtcCorrelation: number | null;
  rankChange: number | null;
  rotation: RotationLabel;
};

/**
 * Every sector on every horizon, plus the rank change that shows rotation.
 *
 * Rank change is the point of the whole board: a sector at rank 9 over six
 * months and rank 2 today is money moving INTO it, which the four raw return
 * columns do not say on their own.
 */
export function allHorizons(snap: Snapshot): {
  count: number;
  sectors: SectorRotationRow[];
  benchmark: Record<HorizonKey, number | null>;
  horizons: { key: HorizonKey; label: string }[];
  basis: string;
} {
  const boards = {} as Record<HorizonKey, SectorBoard>;
  for (const h of H.HORIZON_ORDER) boards[h] = rollUp(snap, h);

  const merged = new Map<string, SectorRotationRow>();
  for (const h of H.HORIZON_ORDER) {
    for (const s of boards[h].sectors) {
      let m = merged.get(s.sector);
      if (!m) {
        m = {
          sector: s.sector,
          label: s.label,
          count: s.count,
          thin: s.thin,
          returns: {},
          breadth: {},
          ranks: {},
          rs: {},
          leaders: {},
          laggards: {},
          volumeX: null,
          turnoverUsd24h: s.turnoverUsd24h,
          oiUsd: s.oiUsd,
          medianFundingPct8h: s.medianFundingPct8h,
          medianBtcCorrelation: s.medianBtcCorrelation,
          rankChange: null,
          rotation: "unknown",
        };
        merged.set(s.sector, m);
      }
      m.returns[h] = s.returnPct;
      m.breadth[h] = s.breadthPct;
      m.ranks[h] = s.rank;
      m.rs[h] = s.rsBenchmark;
      // PER HORIZON, not just the daily board. Carrying one horizon's leader
      // across every column puts a contract's 24-hour leadership next to its
      // sector's six-month return, two measurements in one row implying they
      // describe the same window.
      m.leaders[h] = s.leader;
      m.laggards[h] = s.laggard;
      if (h === "1d") m.volumeX = s.volumeX;
    }
  }

  const rows = [...merged.values()];
  for (const r of rows) {
    const longRank = r.ranks["6m"];
    const shortRank = r.ranks["1w"];
    // Positive = climbing the table over the last week relative to its 6-month
    // standing.
    r.rankChange = longRank && shortRank ? longRank - shortRank : null;
    r.rotation = rotationLabel(r.rankChange, r.returns["6m"] ?? null);
  }

  rows.sort((a, b) => (b.returns["1m"] ?? -999) - (a.returns["1m"] ?? -999));

  return {
    count: rows.length,
    sectors: rows,
    benchmark: {
      "1d": boards["1d"].benchmarkPct,
      "1w": boards["1w"].benchmarkPct,
      "1m": boards["1m"].benchmarkPct,
      "6m": boards["6m"].benchmarkPct,
    },
    horizons: H.HORIZON_ORDER.map((h) => ({ key: h, label: H.HORIZON_LABELS[h] })),
    basis: SECTOR_BASIS,
  };
}

/**
 * The RRG quadrant, named in words rather than plotted.
 *
 *   Leading    strong over six months and still climbing the table
 *   Weakening  strong over six months but slipping in the last week
 *   Improving  weak over six months but turning up — where rotation starts
 *   Lagging    weak and still weak
 */
function rotationLabel(rankChange: number | null, long: number | null): RotationLabel {
  if (long === null) return "unknown";
  const strongLong = long > 0;
  const improving = (rankChange ?? 0) > 0;
  if (strongLong && improving) return "leading";
  if (strongLong) return "weakening";
  if (improving) return "improving";
  return "lagging";
}

export type SectorDrilldown = {
  sector: string;
  label: string;
  horizon: HorizonKey;
  horizonLabel: string;
  summary: SectorRow;
  shape: "thin" | "narrow" | "broad" | "mixed";
  breadthPct: number;
  top2SharePct: number;
  drivers: string[];
  contributions: {
    symbol: string;
    name: string;
    returnPct: number;
    weightPct: number;
    contributionPp: number;
    volumeX: number | null;
    price: number | null;
    turnoverUsd24h: number | null;
  }[];
  constituents: Omit<ScreenerRow, "_closes">[];
  note: string;
};

/** One sector: its constituents ranked, and the decomposition of its move. */
export function drillDown(snap: Snapshot, sector: string, horizon: HorizonKey): SectorDrilldown {
  const board = rollUp(snap, horizon);
  const row = board.sectors.find((s) => s.sector === sector);
  if (!row) throw new ScreenerSectorNotFound(sector);

  const members = snap.rows.filter((r) => r.sector === sector && r.returns[horizon] !== null);
  const rets = members.map((m) => m.returns[horizon]!);
  const weights = members.map((m) => m.turnoverUsd24h ?? 0);
  const totalW = weights.reduce((s, v) => s + v, 0);

  // Contribution: each contract's share of the sector's weighted move. Signed,
  // so a token that dragged the sector down shows as a negative contributor
  // rather than being dropped from the story.
  const contributions = members.map((m, i) => {
    const share = totalW > 0 ? weights[i]! / totalW : 1 / members.length;
    return {
      symbol: m.symbol,
      name: m.name,
      returnPct: m.returns[horizon]!,
      weightPct: H.round(share * 100)!,
      contributionPp: H.round(m.returns[horizon]! * share, 3)!,
      volumeX: m.volumeX,
      price: m.price,
      turnoverUsd24h: m.turnoverUsd24h,
    };
  });
  contributions.sort((a, b) => Math.abs(b.contributionPp) - Math.abs(a.contributionPp));

  const totalContrib =
    contributions.reduce((s, c) => s + Math.abs(c.contributionPp), 0) || 1;
  const top2Share =
    (contributions.slice(0, 2).reduce((s, c) => s + Math.abs(c.contributionPp), 0) / totalContrib) *
    100;

  const up = rets.filter((r) => r > 0).length;
  const breadthPct = (up / members.length) * 100;

  // CONCENTRATION IS TESTED BEFORE BREADTH, and the order matters more than it
  // looks. The two measure different things: breadth counts how many contracts
  // are up, contribution measures how much of the MOVE they account for. A
  // sector can be broad by count and narrow by contribution at the same time —
  // most of the meme book green, but one name carrying 99% of the turnover and
  // therefore of the weighted return. Checking breadth first reports that as
  // "the whole sector moving", which is the opposite of the truth.
  const names = contributions.slice(0, 2).map((c) => c.symbol).join(", ");
  let shape: SectorDrilldown["shape"];
  let shapeText: string;
  if (members.length < 4) {
    shape = "thin";
    shapeText =
      `Only ${members.length} contracts in this sector — too few to tell a broad move from a ` +
      `concentrated one, so neither is claimed`;
  } else if (top2Share >= NARROW_TOP_SHARE) {
    shape = "narrow";
    const caveat =
      breadthPct >= BROAD_BREADTH_PCT
        ? ` — and note ${up} of ${members.length} are green, so this is concentrated by TURNOVER, ` +
          `not by direction`
        : `, and the average contract in it is not doing this`;
    shapeText =
      `${top2Share.toFixed(0)}% of the move is ${names} — the sector number is carried by two ` +
      `contracts${caveat}`;
  } else if (breadthPct >= BROAD_BREADTH_PCT) {
    shape = "broad";
    shapeText =
      `${up} of ${members.length} contracts are up and none dominates the move (top two are ` +
      `${top2Share.toFixed(0)}% of it) — this is the whole sector`;
  } else {
    shape = "mixed";
    shapeText =
      `${up} of ${members.length} up, top two are ${top2Share.toFixed(0)}% of the move — ` +
      `a mixed picture`;
  }

  const drivers = [shapeText];

  const volX = members.map((m) => m.volumeX).filter((v): v is number => v !== null);
  const avgVolX = volX.length ? volX.reduce((s, v) => s + v, 0) / volX.length : null;
  if (avgVolX !== null && avgVolX >= 1.3) {
    drivers.push(
      `Sector volume is running ${avgVolX.toFixed(1)}x its 20-day average — real participation ` +
        `behind the move`,
    );
  } else if (avgVolX !== null && avgVolX < 0.8) {
    drivers.push(
      `Sector volume is only ${avgVolX.toFixed(1)}x its average — the move is happening on thin ` +
        `participation`,
    );
  }

  if (row.rsBenchmark !== null) {
    drivers.push(
      `${row.rsBenchmark > 0 ? "Outperforming" : "Underperforming"} ${snap.benchmark.symbol} by ` +
        `${Math.abs(row.rsBenchmark).toFixed(1)} points over this horizon`,
    );
  }

  // Funding and OI are the readings an equity sector board cannot make. A whole
  // theme with lopsided funding is a positioning fact about the sector, not
  // about any one contract in it.
  if (row.medianFundingPct8h !== null && Math.abs(row.medianFundingPct8h) >= 0.02) {
    const side = row.medianFundingPct8h > 0 ? "long" : "short";
    drivers.push(
      `Median funding across the sector is ${row.medianFundingPct8h > 0 ? "+" : ""}` +
        `${row.medianFundingPct8h.toFixed(3)}% per 8h — the crowded side of this whole theme is ` +
        `${side}, not just one contract in it`,
    );
  }

  const buildups = members.filter((m) => m.oi.buildup === "long_buildup").length;
  const unwinds = members.filter(
    (m) => m.oi.buildup === "long_unwinding" || m.oi.buildup === "short_covering",
  ).length;
  if (buildups >= 3 || unwinds >= 3) {
    drivers.push(
      `Open interest over the last 6h: ${buildups} contracts building longs, ${unwinds} ` +
        `unwinding or covering`,
    );
  }

  if (row.medianBtcCorrelation !== null && row.medianBtcCorrelation >= 0.85) {
    drivers.push(
      `Median 30-day correlation to BTC is ${row.medianBtcCorrelation.toFixed(2)} — this sector ` +
        `mostly moves when BTC does, so read its "rotation" with that in mind`,
    );
  }

  const breakouts = members.filter((m) => m.breakout).map((m) => m.symbol);
  if (breakouts.length > 0) {
    drivers.push(
      `${breakouts.length} constituent(s) broke a multi-month high: ${breakouts.slice(0, 5).join(", ")}`,
    );
  }

  const ranked = [...members].sort((a, b) => b.returns[horizon]! - a.returns[horizon]!);

  return {
    sector,
    label: sectorLabel(sector),
    horizon,
    horizonLabel: H.HORIZON_LABELS[horizon],
    summary: row,
    shape,
    breadthPct: H.round(breadthPct, 1)!,
    top2SharePct: H.round(top2Share, 1)!,
    drivers,
    contributions: contributions.slice(0, 15),
    constituents: ranked.map(({ _closes: _d, ...rest }) => {
      void _d;
      return rest;
    }),
    note:
      "Weights are 24h TURNOVER share, not market cap — this venue publishes no supply or float " +
      "data, and a weighting we cannot source would be a fiction. It answers 'where did the money " +
      "go', not 'what would an index do'.",
  };
}

export class ScreenerSectorNotFound extends Error {
  constructor(sector: string) {
    super(
      `no sector named ${sector} in this universe — sector labels come from Delta's own contract ` +
        `tags, not from any external taxonomy`,
    );
  }
}
