"use client";

import { cn } from "./cn";

export type DeskTabItem<T extends string = string> = {
  key: T;
  label: string;
};

type DeskTabsProps<T extends string> = {
  items: DeskTabItem<T>[];
  active: T;
  onChange: (key: T) => void;
  variant?: "primary" | "secondary";
  className?: string;
};

export function DeskTabs<T extends string>({
  items,
  active,
  onChange,
  variant = "secondary",
  className,
}: DeskTabsProps<T>) {
  return (
    <div
      className={cn("desk-tabs", className)}
      role="tablist"
      style={{
        display: "flex",
        flexWrap: "wrap",
        gap: variant === "primary" ? 0 : 8,
        borderBottom: variant === "primary" ? "1px solid var(--desk-outline)" : undefined,
      }}
    >
      {items.map((item) => {
        const isActive = item.key === active;
        return (
          <button
            key={item.key}
            type="button"
            role="tab"
            aria-selected={isActive}
            onClick={() => onChange(item.key)}
            className="desk-tab"
            style={{
              position: "relative",
              padding: variant === "primary" ? "12px 20px" : "8px 16px",
              border: variant === "secondary" && isActive ? "1px solid var(--desk-outline)" : "none",
              borderRadius: variant === "secondary" ? "var(--desk-radius-chip)" : 0,
              background:
                variant === "secondary"
                  ? isActive
                    ? "var(--desk-primary-container)"
                    : "transparent"
                  : "transparent",
              color: isActive ? "var(--desk-primary)" : "var(--desk-on-surface-variant)",
              fontFamily: "var(--desk-font-display)",
              fontSize: "0.875rem",
              fontWeight: isActive ? 600 : 500,
              cursor: "pointer",
              minHeight: 44,
            }}
          >
            {item.label}
            {variant === "primary" && isActive ? (
              <span
                style={{
                  position: "absolute",
                  bottom: -1,
                  left: 0,
                  right: 0,
                  height: 3,
                  background: "var(--desk-primary)",
                  borderRadius: "3px 3px 0 0",
                }}
              />
            ) : null}
          </button>
        );
      })}
    </div>
  );
}
