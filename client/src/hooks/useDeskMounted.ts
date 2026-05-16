"use client";

import { useEffect, useState } from "react";

/** Avoid hydration mismatch for client-only UI affordances (not required for en-US formatters). */
export function useDeskMounted(): boolean {
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);
  return mounted;
}
