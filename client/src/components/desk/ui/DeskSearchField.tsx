"use client";

import type { InputHTMLAttributes } from "react";
import { cn } from "./cn";

type DeskSearchFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
  label?: string;
};

export function DeskSearchField({ label, className, ...rest }: DeskSearchFieldProps) {
  return (
    <label
      className={cn("desk-search-field", className)}
      style={{ display: "flex", flexDirection: "column", gap: 6, flex: "1 1 200px", maxWidth: 280 }}
    >
      {label ? <span className="desk-label-md">{label}</span> : null}
      <span style={{ position: "relative", display: "block" }}>
        <span
          aria-hidden
          style={{
            position: "absolute",
            left: 12,
            top: "50%",
            transform: "translateY(-50%)",
            color: "var(--desk-on-surface-variant)",
            pointerEvents: "none",
          }}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
            <path d="M15.5 14h-.79l-.28-.27A6.471 6.471 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z" />
          </svg>
        </span>
        <input
          type="search"
          {...rest}
          style={{
            width: "100%",
            minHeight: 44,
            padding: "0 14px 0 40px",
            borderRadius: "var(--desk-radius-input)",
            border: "1px solid var(--desk-outline)",
            background: "var(--desk-surface)",
            color: "var(--desk-on-surface)",
            fontSize: "0.875rem",
            outline: "none",
          }}
        />
      </span>
    </label>
  );
}
