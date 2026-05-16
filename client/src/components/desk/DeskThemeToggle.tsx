"use client";

type DeskThemeToggleProps = {
  dark: boolean;
  onToggle: () => void;
};

export function DeskThemeToggle({ dark, onToggle }: DeskThemeToggleProps) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-label={dark ? "Switch to light theme" : "Switch to dark theme"}
      className="combat-toggle-off"
      style={{ minHeight: 44 }}
    >
      {dark ? "Light" : "Dark"}
    </button>
  );
}
