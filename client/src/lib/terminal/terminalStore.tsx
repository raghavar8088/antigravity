"use client";

import { useEffect, useMemo, useState } from "react";
import { initialTerminalSnapshot } from "./terminalSnapshot";
import type { TerminalSnapshot } from "./terminalTypes";

type TerminalDelta = Partial<TerminalSnapshot>;

function mergeSnapshot(prev: TerminalSnapshot, delta: TerminalDelta): TerminalSnapshot {
  return {
    ...prev,
    ...delta,
    risk: delta.risk ? { ...prev.risk, ...delta.risk } : prev.risk,
    analytics: delta.analytics ? { ...prev.analytics, ...delta.analytics } : prev.analytics,
    updatedAt: delta.updatedAt ?? new Date().toISOString(),
  };
}

export function useTerminalSnapshot() {
  const [snapshot, setSnapshot] = useState<TerminalSnapshot>(initialTerminalSnapshot);
  const wsUrl = process.env.NEXT_PUBLIC_TERMINAL_WS_URL;

  useEffect(() => {
    if (!wsUrl) return;
    let closed = false;
    let socket: WebSocket | null = null;

    const connect = () => {
      if (closed) return;
      socket = new WebSocket(wsUrl);
      socket.onopen = () => setSnapshot((prev) => ({ ...prev, connected: true }));
      socket.onclose = () => {
        setSnapshot((prev) => ({ ...prev, connected: false }));
        if (!closed) window.setTimeout(connect, 1500);
      };
      socket.onerror = () => setSnapshot((prev) => ({ ...prev, connected: false }));
      socket.onmessage = (event) => {
        try {
          const delta = JSON.parse(String(event.data)) as TerminalDelta;
          setSnapshot((prev) => mergeSnapshot(prev, delta));
        } catch {
          // Ignore malformed deltas; the socket remains connected for the next frame.
        }
      };
    };

    connect();
    return () => {
      closed = true;
      socket?.close();
    };
  }, [wsUrl]);

  return useMemo(() => snapshot, [snapshot]);
}
