"use client";

import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { useMockTradingEngine } from "@/hooks/useMockTradingEngine";
import {
  getMockConfigForPipelineStage,
  mockAccountKeyForStage,
} from "@/lib/strategyAuthority/gradeStageMockConfig";
import type { StrategyStatus } from "@/lib/strategyAuthority/types";
import { Metric, TerminalCard } from "./TerminalCard";

function fmtUsd(value: number) {
  if (!Number.isFinite(value)) return "—";
  const sign = value >= 0 ? "+" : "";
  return `${sign}$${Math.abs(value).toLocaleString("en-US", { maximumFractionDigits: 0 })}`;
}

export function GradeStageExecutionPanel({ status }: { status: StrategyStatus }) {
  const live = useLiveBTCPrice();
  const engine = useMockTradingEngine({
    price: live.price,
    accountKey: mockAccountKeyForStage(status),
    pipelineStage: status,
    initialConfig: getMockConfigForPipelineStage(status),
  });

  const dash = "—";
  const priceReady = Number.isFinite(live.price) && live.price > 0;
  const openCount = engine.account.openCount;
  const created = engine.diagnostics.funnel.tradesCreated;
  const candidates = engine.diagnostics.funnel.signalsGenerated;
  const rejected = engine.diagnostics.recentRejections.length;
  const maxOpen = engine.config.maxOpenMockTrades;

  return (
    <TerminalCard
      title={`${status.replace("_", " ")} Discovery Engine`}
      subtitle="Live mock execution — signals fan out to the full ISPAP catalog when desk candidates fire"
    >
      <div className="m3-kpi-strip">
        <Metric
          label="BTC Mark"
          value={priceReady ? `$${live.price.toLocaleString("en-US", { maximumFractionDigits: 0 })}` : dash}
          tone={priceReady ? "positive" : "warning"}
        />
        <Metric
          label="Open Positions"
          value={String(openCount)}
          tone={openCount > 0 ? "positive" : "neutral"}
        />
        <Metric label="Max Open Cap" value={String(maxOpen)} />
        <Metric
          label="Candidates / Tick"
          value={String(candidates)}
          tone={candidates > 0 ? "positive" : "neutral"}
        />
        <Metric
          label="Trades Created"
          value={String(created)}
          tone={created > 0 ? "positive" : "warning"}
        />
        <Metric
          label="Equity"
          value={fmtUsd(engine.account.equity)}
        />
        <Metric
          label="Persistence"
          value={engine.persistence.status === "mongo" ? "MongoDB" : engine.persistence.status}
          tone={engine.persistence.status === "mongo" ? "positive" : "warning"}
        />
      </div>

      <div className="mt-3 grid gap-2 text-[10px] text-zinc-500">
        {!priceReady ? (
          <p className="text-amber-400">Waiting for live BTC price before trade creation can start.</p>
        ) : null}
        {engine.error ? (
          <p className="text-rose-400">Signal tick error: {engine.error}</p>
        ) : null}
        {engine.persistence.error ? (
          <p className="text-rose-400">Persistence: {engine.persistence.error}</p>
        ) : null}
        {rejected > 0 ? (
          <p>
            Recent rejections: {rejected}. Latest:{" "}
            {engine.diagnostics.recentRejections[0]?.message ?? "—"}
          </p>
        ) : (
          <p>
            Grade 5 discovery mode — unlimited open positions, no cooldown, no portfolio risk gates.
            Polling <span className="font-mono text-zinc-400">/api/mock-trading/signal-tick?grade=GRADE_5</span> every 5s.
          </p>
        )}
      </div>
    </TerminalCard>
  );
}
