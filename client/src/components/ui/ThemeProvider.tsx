"use client";

import { useEffect, useState } from "react";

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) return <>{children}</>;
  return <>{children}</>;
}

export function useThemeToggle() {
  const [theme, setTheme] = useState<"light" | "dark">("light");

  useEffect(() => {
    const saved = localStorage.getItem("m3-theme");
    const current = document.documentElement.getAttribute("data-theme");
    const next = current === "light" || current === "dark"
      ? current
      : saved === "light" || saved === "dark"
        ? saved
        : "light";

    document.documentElement.setAttribute("data-theme", next);
    setTheme(next);
  }, []);

  const toggle = () => {
    const next = theme === "dark" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    localStorage.setItem("m3-theme", next);
    setTheme(next);
  };

  return { theme, toggle };
}
