"use client";

import type { StrategyStatus } from "@/lib/strategyAuthority/types";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface TowerLayerData {
  status: StrategyStatus;
  count: number;
  promotionEventsTotal: number;
  demotionEventsTotal: number;
  avgProfitFactor?: number;
  avgExpectancy?: number;
  avgWinRate?: number;
  avgDrawdown?: number;
  avgSharpe?: number;
}

// ── Style maps ────────────────────────────────────────────────────────────────

const LAYER_CONFIG: Record<StrategyStatus, {
  label: string;
  sublabel: string;
  borderClass: string;
  bgClass: string;
  textClass: string;
  countClass: string;
  width: string;
}> = {
  MAIN_ENGINE: {
    label: "MAIN ENGINE",
    sublabel: "575 trades · PF ≥ 1.30 · Sharpe ≥ 0.75",
    borderClass: "border-emerald-500",
    bgClass: "bg-emerald-950/60",
    textClass: "text-emerald-400",
    countClass: "text-emerald-300",
    width: "w-full",
  },
  GRADE_1: {
    label: "GRADE 1",
    sublabel: "Capital Candidate · 200T · PF ≥ 1.30",
    borderClass: "border-amber-500",
    bgClass: "bg-amber-950/40",
    textClass: "text-amber-400",
    countClass: "text-amber-300",
    width: "w-11/12",
  },
  GRADE_2: {
    label: "GRADE 2",
    sublabel: "Institutional Screening · 150T · PF ≥ 1.25",
    borderClass: "border-violet-500",
    bgClass: "bg-violet-950/30",
    textClass: "text-violet-400",
    countClass: "text-violet-300",
    width: "w-10/12",
  },
  GRADE_3: {
    label: "GRADE 3",
    sublabel: "Robustness Testing · 100T · PF ≥ 1.20",
    borderClass: "border-blue-500",
    bgClass: "bg-blue-950/30",
    textClass: "text-blue-400",
    countClass: "text-blue-300",
    width: "w-9/12",
  },
  GRADE_4: {
    label: "GRADE 4",
    sublabel: "Initial Validation · 75T · PF ≥ 1.15",
    borderClass: "border-sky-600",
    bgClass: "bg-sky-950/30",
    textClass: "text-sky-400",
    countClass: "text-sky-300",
    width: "w-11/12",
  },
  GRADE_5: {
    label: "GRADE 5",
    sublabel: "Discovery · 50T · PF ≥ 1.10",
    borderClass: "border-zinc-600",
    bgClass: "bg-zinc-900/60",
    textClass: "text-zinc-400",
    countClass: "text-zinc-300",
    width: "w-full",
  },
  RETIRED: {
    label: "RETIRED",
    sublabel: "Permanently retired",
    borderClass: "border-rose-900",
    bgClass: "bg-rose-950/20",
    textClass: "text-rose-500",
    countClass: "text-rose-400",
    width: "w-full",
  },
};

const TOWER_ORDER: StrategyStatus[] = [
  "MAIN_ENGINE",
  "GRADE_1",
  "GRADE_2",
  "GRADE_3",
  "GRADE_4",
  "GRADE_5",
];

function fmt(n: number | undefined, dec = 2) {
  if (n === undefined || n === null || isNaN(n)) return "—";
  return n.toFixed(dec);
}

// ── Flow Connector ────────────────────────────────────────────────────────────

function FlowArrow({ color = "text-zinc-700" }: { color?: string }) {
  return (
    <div className={`flex flex-col items-center py-0.5 ${color}`}>
      <div className="h-3 w-px bg-current opacity-60" />
      <svg viewBox="0 0 8 6" className="h-1.5 w-2 fill-current opacity-60">
        <path d="M4 6L0 0h8z" />
      </svg>
    </div>
  );
}

// ── Tower Layer ───────────────────────────────────────────────────────────────

function TowerLayer({
  data,
  totalStrategies,
  onClick,
  selected,
}: {
  data: TowerLayerData;
  totalStrategies: number;
  onClick?: () => void;
  selected?: boolean;
}) {
  const cfg = LAYER_CONFIG[data.status];
  const barPct = totalStrategies > 0 ? Math.min(100, (data.count / totalStrategies) * 100) : 0;

  return (
    <button
      onClick={onClick}
      className={`mx-auto block rounded border transition-all duration-200 ${cfg.width} ${cfg.bgClass} ${cfg.borderClass} ${selected ? "ring-1 ring-white/20 shadow-lg" : "hover:brightness-110"} text-left px-4 py-2.5`}
    >
      <div className="flex items-center justify-between">
        {/* Grade label */}
        <div className="min-w-0">
          <div className={`text-[10px] font-bold uppercase tracking-widest ${cfg.textClass}`}>
            {cfg.label}
          </div>
          <div className="mt-0.5 text-[9px] text-zinc-600 truncate">{cfg.sublabel}</div>
        </div>

        {/* Count */}
        <div className={`ml-4 text-2xl font-bold tabular-nums flex-shrink-0 ${cfg.countClass}`}>
          {data.count}
        </div>
      </div>

      {/* Population bar */}
      <div className="mt-2 h-1 rounded-full bg-zinc-800/60">
        <div
          className={`h-full rounded-full transition-all duration-500 ${
            data.status === "MAIN_ENGINE" ? "bg-emerald-500" :
            data.status === "GRADE_1" ? "bg-amber-500" :
            data.status === "GRADE_2" ? "bg-violet-500" :
            data.status === "GRADE_3" ? "bg-blue-500" :
            data.status === "GRADE_4" ? "bg-sky-600" :
            data.status === "RETIRED" ? "bg-rose-700" :
            "bg-zinc-600"
          }`}
          style={{ width: `${barPct}%` }}
        />
      </div>

      {/* Metrics row */}
      {(data.avgProfitFactor !== undefined || data.promotionEventsTotal > 0) && (
        <div className="mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[9px] tabular-nums text-zinc-500">
          {data.avgWinRate !== undefined && <span>WR {fmt(data.avgWinRate, 1)}%</span>}
          {data.avgProfitFactor !== undefined && <span>PF {fmt(data.avgProfitFactor, 2)}</span>}
          {data.avgExpectancy !== undefined && <span>E {fmt(data.avgExpectancy, 2)}</span>}
          {data.avgSharpe !== undefined && data.avgSharpe !== 0 && <span>Sharpe {fmt(data.avgSharpe, 2)}</span>}
          {data.promotionEventsTotal > 0 && (
            <span className="text-emerald-600">↑{data.promotionEventsTotal}</span>
          )}
          {data.demotionEventsTotal > 0 && (
            <span className="text-rose-700">↓{data.demotionEventsTotal}</span>
          )}
        </div>
      )}
    </button>
  );
}

// ── Main Tower Component ──────────────────────────────────────────────────────

export function PromotionTower({
  layers,
  retiredCount = 0,
  onSelectStatus,
  selectedStatus,
  totalStrategies = 305,
}: {
  layers: TowerLayerData[];
  retiredCount?: number;
  onSelectStatus?: (status: StrategyStatus | null) => void;
  selectedStatus?: StrategyStatus | null;
  totalStrategies?: number;
}) {
  const layerMap = new Map(layers.map((l) => [l.status, l]));

  return (
    <div className="flex flex-col items-center w-full space-y-0">
      {/* Tower header */}
      <div className="mb-3 text-center">
        <div className="text-[9px] uppercase tracking-widest text-zinc-600">
          Institutional Promotion Tower
        </div>
        <div className="text-[10px] text-zinc-500 mt-0.5">
          {totalStrategies} strategies · click a layer to inspect
        </div>
      </div>

      {/* Tower layers — top = Main Engine, bottom = Grade 5 */}
      {TOWER_ORDER.map((status, i) => {
        const layerData = layerMap.get(status) ?? {
          status,
          count: 0,
          promotionEventsTotal: 0,
          demotionEventsTotal: 0,
        };
        const isSelected = selectedStatus === status;
        const isLast = i === TOWER_ORDER.length - 1;

        return (
          <div key={status} className="w-full flex flex-col items-center">
            <TowerLayer
              data={layerData}
              totalStrategies={totalStrategies}
              onClick={() => onSelectStatus?.(isSelected ? null : status)}
              selected={isSelected}
            />
            {!isLast && (
              <FlowArrow
                color={
                  status === "MAIN_ENGINE" ? "text-emerald-700" :
                  status === "GRADE_1" ? "text-amber-700" :
                  status === "GRADE_2" ? "text-violet-700" :
                  status === "GRADE_3" ? "text-blue-700" :
                  status === "GRADE_4" ? "text-sky-700" :
                  "text-zinc-700"
                }
              />
            )}
          </div>
        );
      })}

      {/* Retired indicator */}
      {retiredCount > 0 && (
        <div className="mt-2 w-full flex items-center gap-2 rounded border border-rose-900/40 bg-rose-950/10 px-4 py-1.5">
          <span className="text-[9px] uppercase tracking-widest text-rose-600">Retired</span>
          <span className="ml-auto text-sm font-bold tabular-nums text-rose-500">{retiredCount}</span>
        </div>
      )}
    </div>
  );
}
