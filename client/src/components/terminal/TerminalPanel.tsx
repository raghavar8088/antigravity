"use client";

import type { ReactNode } from "react";
import { cn } from "../desk/ui/cn";

type TerminalPanelProps = {
  title: string;
  subtitle?: string;
  children: ReactNode;
  headerActions?: ReactNode;
  className?: string;
  padding?: "none" | "sm" | "md";
  height?: string | number;
  maxHeight?: string | number;
  overflow?: "auto" | "hidden" | "visible";
  onClose?: () => void;
  onMaximize?: () => void;
  isMaximized?: boolean;
};

export function TerminalPanel({
  title,
  subtitle,
  children,
  headerActions,
  className,
  padding = "md",
  height,
  maxHeight,
  overflow = "auto",
  onClose,
  onMaximize,
  isMaximized = false,
}: TerminalPanelProps) {
  return (
    <section
      className={cn("terminal-card animate-fade-in-up", className, isMaximized && "maximized")}
      style={isMaximized ? {
        position: "fixed",
        top: "var(--topbar-height)",
        left: "var(--sidebar-width)",
        right: 0,
        bottom: 0,
        zIndex: 1000,
        background: "var(--bg)",
        borderRadius: 0,
        border: "none",
      } : {
        display: "flex",
        flexDirection: "column",
        height: height ?? "100%",
        maxHeight: maxHeight,
        overflow: "hidden",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "8px 12px",
          borderBottom: "1px solid var(--border)",
          background: "var(--surface-2)",
          flexShrink: 0,
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 1 }}>
          <h3
            style={{
              fontSize: 10,
              fontWeight: 700,
              color: "var(--text-primary)",
              textTransform: "uppercase",
              letterSpacing: "0.05em",
              margin: 0,
              lineHeight: 1.2,
            }}
          >
            {title}
          </h3>
          {subtitle && (
            <span
              style={{
                fontSize: 9,
                color: "var(--text-muted)",
                fontWeight: 500,
                letterSpacing: "0.02em",
              }}
            >
              {subtitle}
            </span>
          )}
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          {headerActions}
          <div style={{ display: "flex", alignItems: "center", gap: 2, marginLeft: 4, paddingLeft: 6, borderLeft: "1px solid var(--border)" }}>
            {onMaximize && (
              <button
                onClick={onMaximize}
                style={{
                  background: "transparent",
                  border: "none",
                  color: "var(--text-muted)",
                  cursor: "pointer",
                  padding: 2,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
                title={isMaximized ? "Restore" : "Maximize"}
              >
                <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
                  {isMaximized ? (
                    <path d="M3 10h3v3M13 6h-3V3M6 10L2 14M10 6l4-4" />
                  ) : (
                    <path d="M10 2h4v4M6 14H2v-4M14 2L10 6M2 14l4-4" />
                  )}
                </svg>
              </button>
            )}
            {onClose && (
              <button
                onClick={onClose}
                style={{
                  background: "transparent",
                  border: "none",
                  color: "var(--text-muted)",
                  cursor: "pointer",
                  padding: 2,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
                title="Close"
              >
                <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M4 4l8 8M12 4l-8 8" />
                </svg>
              </button>
            )}
          </div>
        </div>
      </div>
      <div
        style={{
          flex: 1,
          overflow: overflow,
          padding: padding === "none" ? 0 : padding === "sm" ? 8 : 12,
        }}
      >
        {children}
      </div>
    </section>
  );
}
