"use client";

import * as Tabs from "@radix-ui/react-tabs";
import type { ReactNode } from "react";
import { cn } from "./cn";

export function M3Tabs({
  tabs,
  defaultValue,
  value,
  onValueChange,
  /** "pill" matches the reference design system's rounded-pill tab switcher (purple active state). */
  variant = "underline",
}: {
  tabs: Array<{ value: string; label: string; content: ReactNode }>;
  defaultValue?: string;
  value?: string;
  onValueChange?: (v: string) => void;
  variant?: "underline" | "pill";
}) {
  const isPill = variant === "pill";
  return (
    <Tabs.Root
      defaultValue={defaultValue ?? tabs[0]?.value}
      value={value}
      onValueChange={onValueChange}
      className={cn("m3-tabs", isPill && "m3-tabs--pill")}
    >
      <Tabs.List className={cn("m3-tabs__list", isPill && "m3-tabs__list--pill")} aria-label="Tabs">
        {tabs.map((tab) => (
          <Tabs.Trigger
            key={tab.value}
            value={tab.value}
            className={cn("m3-tabs__trigger", isPill && "m3-tabs__trigger--pill")}
          >
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
