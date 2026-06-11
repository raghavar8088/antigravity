"use client";

import { Card } from "@/components/ui/Card";
import { M3Tabs } from "@/components/ui/Tabs";
import { PageHeader } from "@/components/ui/PageHeader";
import { Switch } from "@/components/ui/FormControls";
import { useThemeToggle } from "@/components/ui/ThemeProvider";
import { useDensity } from "@/components/ui/DensityProvider";
import { useState } from "react";

export default function SettingsPage() {
  const { theme, toggle } = useThemeToggle();
  const { density, setDensity } = useDensity();
  const [compactTables, setCompactTables] = useState(false);

  return (
    <div className="m3-page-stack">
      <PageHeader title="Settings" subtitle="Appearance and display preferences" />

      <div className="m3-settings-layout">
        <nav className="m3-settings-nav" aria-label="Settings sections">
          <button type="button" className="m3-settings-nav__item m3-settings-nav__item--active">Appearance</button>
          <button type="button" className="m3-settings-nav__item">Environment</button>
          <button type="button" className="m3-settings-nav__item">Display</button>
        </nav>

        <div className="m3-settings-content">
          <Card title="Theme" subtitle="Light or dark appearance">
            <p className="m3-settings-desc">Current theme: <strong>{theme}</strong></p>
            <button type="button" className="m3-btn m3-btn--tonal" onClick={toggle}>Toggle theme</button>
          </Card>

          <Card title="Density" subtitle="Table and panel spacing">
            <M3Tabs
              tabs={[
                {
                  value: "comfortable",
                  label: "Comfortable",
                  content: (
                    <div className="m3-settings-tab-panel">
                      <p>Default dashboard spacing — Google Analytics style.</p>
                      <button type="button" className="m3-btn m3-btn--filled" onClick={() => setDensity("comfortable")}>Apply</button>
                    </div>
                  ),
                },
                {
                  value: "compact",
                  label: "Compact",
                  content: (
                    <div className="m3-settings-tab-panel">
                      <p>Trade tables and execution panels.</p>
                      <button type="button" className="m3-btn m3-btn--filled" onClick={() => setDensity("compact")}>Apply</button>
                    </div>
                  ),
                },
                {
                  value: "ultra-compact",
                  label: "Ultra-compact",
                  content: (
                    <div className="m3-settings-tab-panel">
                      <p>Signal trace and diagnostics only.</p>
                      <button type="button" className="m3-btn m3-btn--filled" onClick={() => setDensity("ultra-compact")}>Apply</button>
                    </div>
                  ),
                },
              ]}
              value={density}
              onValueChange={(v) => setDensity(v as typeof density)}
            />
          </Card>

          <Card title="Display" subtitle="Table preferences">
            <Switch checked={compactTables} onCheckedChange={setCompactTables} label="Compact trade tables by default" />
          </Card>

          <Card title="Environment" subtitle="Runtime configuration (read-only)">
            <dl className="m3-settings-env">
              <dt>MONGODB_URI</dt><dd>Configured</dd>
              <dt>INTERNAL_API_URL</dt><dd>Engine proxy</dd>
              <dt>NEXT_PUBLIC_UI_M3</dt><dd>{process.env.NEXT_PUBLIC_UI_M3 ?? "1 (default)"}</dd>
            </dl>
          </Card>
        </div>
      </div>
    </div>
  );
}
