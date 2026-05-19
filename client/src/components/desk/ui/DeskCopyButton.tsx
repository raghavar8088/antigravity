"use client";

import { useCallback, useState } from "react";

type DeskCopyButtonProps = {
  /** Text to copy to clipboard when the button is clicked. */
  text: string;
  /** Accessible label / tooltip ("Copy link to X"). */
  ariaLabel: string;
  /** Visual size in px (icon glyph scales). Default 16. */
  size?: number;
  /** Optional className for custom positioning. */
  className?: string;
};

/**
 * Small inline copy-to-clipboard button. Shows a transient "Copied" tooltip
 * for ~1.5s after a successful clipboard write. Stops event propagation so
 * the button can be safely nested next to a clickable tab without firing
 * the tab's onClick.
 *
 * Falls back to a textarea + execCommand path when navigator.clipboard
 * isn't available (older browsers, http contexts).
 */
export function DeskCopyButton({ text, ariaLabel, size = 16, className }: DeskCopyButtonProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(
    async (e: React.MouseEvent<HTMLButtonElement>) => {
      e.preventDefault();
      e.stopPropagation();
      try {
        if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(text);
        } else if (typeof document !== "undefined") {
          const ta = document.createElement("textarea");
          ta.value = text;
          ta.style.position = "fixed";
          ta.style.opacity = "0";
          document.body.appendChild(ta);
          ta.select();
          try {
            document.execCommand("copy");
          } finally {
            document.body.removeChild(ta);
          }
        }
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1400);
      } catch {
        // Silent fail — copy is best-effort UX, not a critical path.
      }
    },
    [text],
  );

  return (
    <button
      type="button"
      onClick={handleCopy}
      aria-label={ariaLabel}
      title={copied ? "Copied!" : ariaLabel}
      className={className}
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        width: size + 12,
        height: size + 12,
        padding: 0,
        marginLeft: 6,
        border: "1px solid var(--desk-outline-variant)",
        borderRadius: "var(--desk-radius-chip)",
        background: copied ? "var(--desk-primary-container)" : "var(--desk-surface-container)",
        color: copied ? "var(--desk-primary)" : "var(--desk-on-surface-variant)",
        cursor: "pointer",
        transition: "background 120ms ease, color 120ms ease",
        flexShrink: 0,
      }}
    >
      {copied ? (
        // Check mark when copied
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M3 8l3.5 3.5L13 5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      ) : (
        // Clipboard icon (two overlapping squares)
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <rect x="4" y="2" width="8" height="10" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
          <rect x="2" y="4" width="8" height="10" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
        </svg>
      )}
    </button>
  );
}
