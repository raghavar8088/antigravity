"use client";

import type { ReactNode } from "react";
import { DeskCard } from "@/components/desk/ui/DeskCard";
import { DeskChip } from "@/components/desk/ui/DeskChip";
import { DeskMetricTile } from "@/components/desk/ui/DeskMetricTile";
import { DeskSectionHeader } from "@/components/desk/ui/DeskSectionHeader";

type HeatSnapshot = {
  currentHeatPct: number;
  averageHeatPct: number;
  maxHeatPct: number;
  heatTrend: number;
  level: string;
  action: string;
};

type VaRReport = {
  dailyVaR95USD: number;
  dailyVaR99USD: number;
  weeklyVaR95USD: number;
  monthlyVaR95USD: number;
  dailyVaRPct: number;
  breached: boolean;
};

type CVaRReport = {
  cVaR95USD: number;
  cVaR99USD: number;
  cVaR95Pct: number;
  cVaR99Pct: number;
};

type TailRisk = {
  score: number;
  action: string;
  reason: string;
};

type Exposure = {
  longExposureUSD: number;
  shortExposureUSD: number;
  netExposureUSD: number;
  grossExposureUSD: number;
  netExposurePct: number;
  grossExposurePct: number;
};

type Forecast = {
  futureVaRPct: number;
  futureDrawdownPct: number;
  futureHeatPct: number;
  futureExposurePct: number;
  riskForecastScore: number;
};

type Kelly = {
  selectedFraction: number;
  kellyConfidence: number;
  kellyStability: number;
  kellyDrawdownRisk: number;
  recommendedSizeBTC: number;
};

type Attribution = {
  byStrategy: Record<string, number>;
  byFamily: Record<string, number>;
  fundingRisk: number;
  volatilityRisk: number;
  executionRisk: number;
  correlationRisk: number;
  regimeRisk: number;
};

type RiskAlert = {
  severity: "INFO" | "WARNING" | "CRITICAL";
  title: string;
  message: string;
};

export type InstitutionalRiskCenterData = {
  portfolioHeatGauge: HeatSnapshot;
  vaRDashboard: VaRReport[];
  cVaRDashboard: CVaRReport[];
  correlationHeatmap: Record<string, Record<string, number>>;
  capitalAllocation: Record<string, number>;
  riskBudgetDashboard: Record<string, number>;
  drawdownDashboard: { drawdownPct: number; severity: string; action: string; recoveryProgressPct: number };
  leverageDashboard: number;
  tailRiskMonitor: TailRisk;
  riskAttributionView: Attribution;
  exposureMonitor: Exposure;
  forecastDashboard: Forecast;
  kellySizingWidget: Kelly;
  alerts: RiskAlert[];
};

type Props = {
  data: InstitutionalRiskCenterData;
};

function fmtUsd(value: number) {
  const abs = Math.abs(value).toLocaleString("en-US", { minimumFractionDigits: 0, maximumFractionDigits: 0 });
  return `${value >= 0 ? "+" : "-"}$${abs}`;
}

function fmtPct(value: number) {
  return `${value.toFixed(2)}%`;
}

function toneForRisk(score: number) {
  if (score >= 80) return "success" as const;
  if (score >= 60) return "warning" as const;
  return "error" as const;
}

export default function InstitutionalRiskCenter({ data }: Props) {
  const latestVar = data.vaRDashboard.at(-1);
  const latestCvar = data.cVaRDashboard.at(-1);
  const topFamilies = Object.entries(data.riskAttributionView.byFamily).slice(0, 6);
  const allocationRows = Object.entries(data.capitalAllocation).slice(0, 6);
  const budgetRows = Object.entries(data.riskBudgetDashboard).slice(0, 8);

  return (
    <DeskCard>
      <DeskSectionHeader
        title="Institutional Risk Center"
        subtitle="Portfolio heat, VaR/CVaR, allocation, attribution, leverage, tail risk, and forecasts"
        actions={
          <>
            <DeskChip tone={toneForRisk(data.forecastDashboard.riskForecastScore)}>
              Forecast {fmtPct(data.forecastDashboard.riskForecastScore)}
            </DeskChip>
            <DeskChip tone={data.tailRiskMonitor.action === "NORMAL" ? "success" : "error"}>
              {data.tailRiskMonitor.action}
            </DeskChip>
          </>
        }
      />

      <div className="grid gap-3 md:grid-cols-4">
        <DeskMetricTile label="Portfolio Heat" value={fmtPct(data.portfolioHeatGauge.currentHeatPct)} sub={data.portfolioHeatGauge.action} />
        <DeskMetricTile label="VaR 95" value={latestVar ? fmtUsd(-latestVar.dailyVaR95USD) : "$0"} />
        <DeskMetricTile label="CVaR 95" value={latestCvar ? fmtUsd(-latestCvar.cVaR95USD) : "$0"} />
        <DeskMetricTile label="Leverage" value={`${data.leverageDashboard.toFixed(1)}x`} />
      </div>

      <div className="mt-4 grid gap-3 lg:grid-cols-3">
        <Panel title="Exposure Monitor">
          <Metric label="Long" value={fmtUsd(data.exposureMonitor.longExposureUSD)} />
          <Metric label="Short" value={fmtUsd(data.exposureMonitor.shortExposureUSD)} />
          <Metric label="Net" value={`${fmtUsd(data.exposureMonitor.netExposureUSD)} · ${fmtPct(data.exposureMonitor.netExposurePct)}`} />
          <Metric label="Gross" value={`${fmtUsd(data.exposureMonitor.grossExposureUSD)} · ${fmtPct(data.exposureMonitor.grossExposurePct)}`} />
        </Panel>

        <Panel title="Kelly Sizing">
          <Metric label="Selected Kelly" value={fmtPct(data.kellySizingWidget.selectedFraction * 100)} />
          <Metric label="Recommended BTC" value={data.kellySizingWidget.recommendedSizeBTC.toFixed(4)} />
          <Metric label="Confidence" value={fmtPct(data.kellySizingWidget.kellyConfidence * 100)} />
          <Metric label="Stability" value={fmtPct(data.kellySizingWidget.kellyStability * 100)} />
          <Metric label="Drawdown risk" value={fmtPct(data.kellySizingWidget.kellyDrawdownRisk * 100)} />
        </Panel>

        <Panel title="Drawdown & Tail Risk">
          <Metric label="Drawdown" value={`${fmtPct(data.drawdownDashboard.drawdownPct)} · ${data.drawdownDashboard.severity}`} />
          <Metric label="Recovery" value={fmtPct(data.drawdownDashboard.recoveryProgressPct)} />
          <Metric label="Tail score" value={fmtPct(data.tailRiskMonitor.score)} />
          <Metric label="Tail action" value={data.tailRiskMonitor.reason} />
        </Panel>
      </div>

      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        <Panel title="Risk Attribution">
          {topFamilies.map(([family, risk]) => (
            <Metric key={family} label={family} value={fmtPct(risk)} />
          ))}
          <Metric label="Volatility" value={fmtPct(data.riskAttributionView.volatilityRisk)} />
          <Metric label="Correlation" value={fmtPct(data.riskAttributionView.correlationRisk)} />
          <Metric label="Execution" value={fmtPct(data.riskAttributionView.executionRisk)} />
        </Panel>

        <Panel title="Forecast Dashboard">
          <Metric label="Future VaR" value={fmtPct(data.forecastDashboard.futureVaRPct)} />
          <Metric label="Future drawdown" value={fmtPct(data.forecastDashboard.futureDrawdownPct)} />
          <Metric label="Future heat" value={fmtPct(data.forecastDashboard.futureHeatPct)} />
          <Metric label="Future exposure" value={fmtPct(data.forecastDashboard.futureExposurePct)} />
        </Panel>
      </div>

      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        <Panel title="Capital Allocation">
          {allocationRows.length === 0 ? <Metric label="Allocation" value="No active allocation" /> : null}
          {allocationRows.map(([strategy, pct]) => (
            <Metric key={strategy} label={strategy} value={fmtPct(pct)} />
          ))}
        </Panel>

        <Panel title="Risk Budget">
          {budgetRows.map(([family, pct]) => (
            <Metric key={family} label={family} value={fmtPct(pct)} />
          ))}
        </Panel>
      </div>

      {data.alerts.length > 0 ? (
        <div className="mt-4">
          <Panel title="Risk Alerts">
            {data.alerts.slice(0, 5).map((alert) => (
              <Metric key={`${alert.title}-${alert.message}`} label={alert.severity} value={`${alert.title}: ${alert.message}`} />
            ))}
          </Panel>
        </div>
      ) : null}
    </DeskCard>
  );
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rounded-[16px] border border-zinc-200 bg-zinc-50/80 p-4">
      <h3 className="mb-3 text-sm font-semibold text-zinc-900">{title}</h3>
      <div className="space-y-2">{children}</div>
    </section>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 text-xs">
      <span className="text-zinc-500">{label}</span>
      <span className="text-right font-mono font-medium text-zinc-900">{value}</span>
    </div>
  );
}
