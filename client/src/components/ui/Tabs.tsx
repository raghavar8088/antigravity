"use client";

import * as Tabs from "@radix-ui/react-tabs";
import type { ReactNode } from "react";

export function M3Tabs({
  tabs,
  defaultValue,
  value,
  onValueChange,
}: {
  tabs: Array<{ value: string; label: string; content: ReactNode }>;
  defaultValue?: string;
  value?: string;
  onValueChange?: (v: string) => void;
}) {
  return (
    <Tabs.Root defaultValue={defaultValue ?? tabs[0]?.value} value={value} onValueChange={onValueChange} className="m3-tabs">
      <Tabs.List className="m3-tabs__list" aria-label="Tabs">
        {tabs.map((tab) => (
          <Tabs.Trigger key={tab.value} value={tab.value} className="m3-tabs__trigger">
            {tab.label}
          </Tabs.Trigger>
        ))}
      </Tabs.List>
      {tabs.map((tab) => (
        <Tabs.Content key={tab.value} value={tab.value} className="m3-tabs__content">
          {tab.content}
        </Tabs.Content>
      ))}
    </Tabs.Root>
  );
}
