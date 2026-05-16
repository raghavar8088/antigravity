"use client";

import { cn } from "./cn";

type DeskSwitchProps = {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  disabled?: boolean;
  id?: string;
  ariaLabel?: string;
};

export function DeskSwitch({ checked, onChange, label, disabled = false, id, ariaLabel }: DeskSwitchProps) {
  const switchId = id ?? `desk-switch-${(label || ariaLabel || "toggle").replace(/\s+/g, "-").toLowerCase()}`;
  return (
    <label
      htmlFor={switchId}
      className={cn("desk-switch")}
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 8,
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.38 : 1,
        minHeight: 44,
      }}
    >
      {label ? (
        <span className="desk-label-md" style={{ fontWeight: 500 }}>
          {label}
        </span>
      ) : null}
      <button
        id={switchId}
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={ariaLabel ?? label ?? "Toggle"}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        style={{
          position: "relative",
          width: 52,
          height: 32,
          flexShrink: 0,
          borderRadius: 16,
          border: "none",
          padding: 0,
          cursor: disabled ? "not-allowed" : "pointer",
          background: checked ? "var(--desk-primary)" : "var(--desk-surface-container-high)",
          transition: "background 0.2s ease",
        }}
      >
        <span
          aria-hidden
          style={{
            position: "absolute",
            top: 4,
            left: checked ? 24 : 4,
            width: 24,
            height: 24,
            borderRadius: "50%",
            background: checked ? "var(--desk-on-primary)" : "var(--desk-outline)",
            boxShadow: "var(--desk-elevation-1)",
            transition: "left 0.2s ease",
          }}
        />
      </button>
    </label>
  );
}
