"use client";

import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "./cn";

type DeskButtonVariant = "filled" | "tonal" | "outlined" | "text" | "danger-tonal";

type DeskButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: DeskButtonVariant;
  children: ReactNode;
  icon?: ReactNode;
};

const variantStyle: Record<DeskButtonVariant, React.CSSProperties> = {
  filled: {
    background: "var(--desk-primary)",
    color: "var(--desk-on-primary)",
    border: "none",
  },
  tonal: {
    background: "var(--desk-primary-container)",
    color: "var(--desk-on-primary-container)",
    border: "none",
  },
  outlined: {
    background: "transparent",
    color: "var(--desk-primary)",
    border: "1px solid var(--desk-outline)",
  },
  text: {
    background: "transparent",
    color: "var(--desk-primary)",
    border: "none",
  },
  "danger-tonal": {
    background: "var(--desk-error-container)",
    color: "var(--desk-error)",
    border: "none",
  },
};

export function DeskButton({
  variant = "tonal",
  children,
  icon,
  className,
  disabled,
  type = "button",
  style,
  ...rest
}: DeskButtonProps) {
  return (
    <button
      type={type}
      disabled={disabled}
      className={cn("desk-button", className)}
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        gap: 8,
        minHeight: 44,
        minWidth: 44,
        padding: "0 20px",
        borderRadius: "var(--desk-radius-button)",
        fontFamily: "var(--desk-font-display)",
        fontSize: "0.875rem",
        fontWeight: 500,
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.38 : 1,
        transition: "background 0.15s ease, box-shadow 0.15s ease",
        ...variantStyle[variant],
        ...style,
      }}
      {...rest}
    >
      {icon}
      {children}
    </button>
  );
}
