"use client";

import {
  Badge,
  Button,
  Card,
  Chip,
  EmptyState,
  Metric,
  Skeleton,
  SkeletonBlock,
  SkeletonCard,
  StatusChip,
} from "@/components/ui";
import { RegimeBadge } from "@/components/terminal/institutional/MockStageTradingSuite";
import { Metric as TerminalMetric, TerminalCard } from "@/components/terminal/institutional/TerminalCard";
import { M3Tabs } from "@/components/ui/Tabs";
import { PageHeader, SectionHeader } from "@/components/ui/PageHeader";
import { TextField, Select, Switch } from "@/components/ui/FormControls";
import { useDensity } from "@/components/ui/DensityProvider";
import { useSnackbar } from "@/components/ui/Snackbar";

export default function DesignSystemPage() {
  const { density, setDensity } = useDensity();
  const { show } = useSnackbar();

  return (
    <div className="m3-page-stack">
      <PageHeader
        title="Design System"
        subtitle="Material Design 3 — ICC institutional tokens and components"
        actions={<Badge label="GM3-ICCTP" tone="info" />}
      />

      <Card title="Color tokens" subtitle="Semantic M3 roles">
        <div className="m3-ds-swatches">
          {[
            ["Primary", "var(--m3-primary)"],
            ["Surface", "var(--m3-surface-container)"],
            ["Profit", "var(--m3-profit)"],
            ["Loss", "var(--m3-loss)"],
            ["Warning", "var(--m3-warning)"],
          ].map(([name, color]) => (
            <div key={name} className="m3-ds-swatch">
              <div className="m3-ds-swatch__chip" style={{ background: color }} />
              <span>{name}</span>
            </div>
          ))}
        </div>
      </Card>

      <Card title="Typography" subtitle="Display → Label scale">
        <div className="m3-ds-type">
          <p style={{ font: "var(--m3-headline-md)" }}>Headline — Command Center</p>
          <p style={{ font: "var(--m3-title-md)" }}>Title — Portfolio analytics</p>
          <p style={{ font: "var(--m3-body-md)" }}>Body — Strategy health and expectancy metrics</p>
          <p style={{ font: "var(--m3-label-sm)" }}>Label — Last updated 3s ago</p>
          <p className="m3-data-table__cell--tabular" style={{ font: "var(--m3-body-md)" }}>$67,432.18 +2.4%</p>
        </div>
      </Card>

      <SectionHeader title="Components" subtitle="Shared ui/ primitives" />

      <div className="m3-ds-grid">
        <Card title="Buttons">
          <div className="m3-ds-row">
            <Button variant="filled">Filled</Button>
            <Button variant="tonal">Tonal</Button>
            <Button variant="outlined">Outlined</Button>
            <Button variant="text">Text</Button>
          </div>
        </Card>

        <Card title="Status">
          <div className="m3-ds-row">
            <StatusChip label="Live" tone="success" />
            <StatusChip label="Info" tone="info" />
            <StatusChip label="Warning" tone="warning" />
            <StatusChip label="Critical" tone="error" />
            <StatusChip label="Offline" tone="neutral" />
            <Chip label="Filter" selected />
          </div>
        </Card>

        <Card title="Metrics">
          <div className="m3-kpi-strip">
            <Metric label="Equity" value="$1.02M" tone="positive" delta="+1.8%" size="lg" />
            <Metric label="Drawdown" value="-2.4%" tone="negative" delta="-0.6%" size="md" />
            <Metric label="Heat" value="18%" tone="warning" delta="Stable" size="sm" />
          </div>
        </Card>

        <Card title="Regime Badge">
          <div className="m3-ds-row">
            <RegimeBadge regime="RANGING" />
            <RegimeBadge regime="TRENDING" />
            <RegimeBadge regime="VOLATILE" />
          </div>
        </Card>

        <TerminalCard title="Terminal Card Accent" subtitle="Left border follows the strongest metric tone">
          <div className="m3-kpi-strip">
            <TerminalMetric label="Net PnL" value="+$12.4K" tone="positive" delta="+3.2%" size="lg" />
            <TerminalMetric label="Risk Heat" value="18%" tone="warning" size="md" />
          </div>
        </TerminalCard>

        <Card title="Forms">
          <div className="m3-ds-form">
            <TextField label="Strategy name" placeholder="Search…" />
            <Select label="Regime" defaultValue=""><option value="">All</option><option>TREND</option></Select>
            <Switch checked label="Auto-scroll" onCheckedChange={() => {}} />
          </div>
        </Card>

        <Card title="Feedback">
          <div className="m3-ds-row">
            <Button variant="tonal" onClick={() => show("Settings saved", { tone: "success" })}>Show snackbar</Button>
            <Skeleton width={120} height={16} />
            <SkeletonBlock width={160} height={16} rounded={4} />
          </div>
          <EmptyState title="No trades" subtitle="Trades appear when strategies fire" />
        </Card>

        <Card title="Density" subtitle={`Current: ${density}`}>
          <div className="m3-ds-row">
            {(["comfortable", "compact", "ultra-compact"] as const).map((d) => (
              <Button key={d} variant={density === d ? "filled" : "outlined"} onClick={() => setDensity(d)}>{d}</Button>
            ))}
          </div>
        </Card>
      </div>

      <Card title="Tabs">
        <M3Tabs
          tabs={[
            { value: "overview", label: "Overview", content: <p className="m3-ds-tab-content">Analytics overview panel</p> },
            { value: "trades", label: "Trades", content: <p className="m3-ds-tab-content">Trade history table</p> },
          ]}
        />
      </Card>

      <Card title="Loading">
        <div className="m3-page-stack">
          <SkeletonCard rows={4} />
          <div className="m3-kpi-strip">
            {Array.from({ length: 3 }).map((_, index) => (
              <SkeletonBlock key={index} height={52} rounded={8} />
            ))}
          </div>
        </div>
      </Card>
    </div>
  );
}
