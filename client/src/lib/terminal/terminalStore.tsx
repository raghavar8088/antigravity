"use client";

import { createContext, useContext, useEffect, useMemo, useReducer, useRef, type ReactNode } from "react";
import { initialTerminalSnapshot } from "./terminalSnapshot";
import {
  mapSnapshotToTerminalDelta,
  type PaperDeskSnapshotPayload,
  type StrategyIntelRow,
} from "./mapSnapshotToTerminalDelta";
import {
  terminalHasAuthority,
  type AuthoritySource,
  type TerminalAuthorityState,
} from "./terminalAuthority";
import type { TerminalSnapshot } from "./terminalTypes";

type TerminalDelta = Partial<TerminalSnapshot>;

type StoreState = TerminalAuthorityState;

type StoreAction =
  | { type: "WS_OPEN" }
  | { type: "WS_CLOSE" }
  | { type: "WS_DELTA"; delta: TerminalDelta }
  | { type: "REST_OK"; delta: TerminalDelta }
  | { type: "REST_FAIL" }
  | { type: "REST_UNAVAILABLE" };

function mergeSnapshot(prev: TerminalSnapshot, delta: TerminalDelta): TerminalSnapshot {
  return {
    ...prev,
    ...delta,
    risk: delta.risk ? { ...prev.risk, ...delta.risk } : prev.risk,
    analytics: delta.analytics ? { ...prev.analytics, ...delta.analytics } : prev.analytics,
    updatedAt: delta.updatedAt ?? new Date().toISOString(),
  };
}

const initialState: StoreState = {
  ...initialTerminalSnapshot,
  authoritySource: "none",
  restUnavailable: false,
  hasAuthority: false,
};

function reducer(state: StoreState, action: StoreAction): StoreState {
  switch (action.type) {
    case "WS_OPEN":
      return {
        ...state,
        connected: true,
        loading: false,
        authoritySource: "ws",
        restUnavailable: false,
        hasAuthority: false,
      };
    case "WS_CLOSE":
      return { ...state, connected: false, hasAuthority: state.authoritySource === "rest" && state.updatedAt !== "" };
    case "WS_DELTA": {
      const next = {
        ...mergeSnapshot(state, action.delta),
        authoritySource: "ws" as AuthoritySource,
        loading: false,
        restUnavailable: false,
      };
      return { ...next, hasAuthority: terminalHasAuthority(next) };
    }
    case "REST_OK": {
      const next = {
        ...mergeSnapshot(state, action.delta),
        authoritySource: (state.connected ? "ws" : "rest") as AuthoritySource,
        loading: false,
        restUnavailable: false,
      };
      return { ...next, hasAuthority: terminalHasAuthority(next) };
    }
    case "REST_FAIL":
      return { ...state, loading: false };
    case "REST_UNAVAILABLE":
      return { ...state, restUnavailable: true, loading: false, hasAuthority: false };
    default:
      return state;
  }
}

const REST_POLL_INTERVAL_MS = 5000;
const REST_POLL_DISCONNECTED_MS = 3000;
const CIRCUIT_BREAKER_THRESHOLD = 3;
const CIRCUIT_BREAKER_BACKOFF_MS = 30_000;

async function fetchRestAuthority(): Promise<TerminalDelta | null> {
  const [snapshotRes, intelRes, equityRes, priceRes] = await Promise.all([
    fetch("/api/paper-desk/snapshot", { cache: "no-store" }),
    fetch("/api/strategy-intelligence?view=all&limit=100", { cache: "no-store" }),
    fetch("/api/paper-desk/equity?points=30&days=7", { cache: "no-store" }),
    fetch("/api/btc/price", { cache: "no-store" }),
  ]);

  if (!snapshotRes.ok) return null;

  const snapshot = (await snapshotRes.json()) as PaperDeskSnapshotPayload & { ok?: boolean };
  if (snapshot.ok === false) return null;

  let strategies: StrategyIntelRow[] | undefined;
  if (intelRes.ok) {
    const intel = await intelRes.json();
    if (intel.ok && Array.isArray(intel.strategies)) strategies = intel.strategies;
  }

  let equity: { curve?: Array<{ snapped_at?: string; ts?: string; equity?: number }> } | undefined;
  if (equityRes.ok) {
    const eq = await equityRes.json();
    if (eq.ok) equity = eq;
  }

  let btcPrice: { price?: number; change24h?: number } | undefined;
  if (priceRes.ok) {
    const px = await priceRes.json();
    if (px.ok) btcPrice = px;
  }

  return mapSnapshotToTerminalDelta({ snapshot, strategies, btcPrice, equity });
}

function useTerminalSnapshotState(): TerminalAuthorityState {
  const wsUrl = process.env.NEXT_PUBLIC_TERMINAL_WS_URL;
  const [state, dispatch] = useReducer(reducer, initialState);
  const restFailCount = useRef(0);
  const circuitOpen = useRef(false);
  const wsConnectedRef = useRef(false);

  useEffect(() => {
    wsConnectedRef.current = state.connected;
  }, [state.connected]);

  useEffect(() => {
    if (!wsUrl) return;
    let closed = false;
    let socket: WebSocket | null = null;

    const connect = () => {
      if (closed) return;
      socket = new WebSocket(wsUrl);
      socket.onopen = () => dispatch({ type: "WS_OPEN" });
      socket.onclose = () => {
        dispatch({ type: "WS_CLOSE" });
        if (!closed) window.setTimeout(connect, 2000);
      };
      socket.onerror = () => dispatch({ type: "WS_CLOSE" });
      socket.onmessage = (event) => {
        try {
          const delta = JSON.parse(String(event.data)) as TerminalDelta;
          dispatch({ type: "WS_DELTA", delta });
        } catch {
          // ignore malformed frame
        }
      };
    };

    connect();
    return () => {
      closed = true;
      socket?.close();
    };
  }, [wsUrl]);

  useEffect(() => {
    let cancelled = false;
    let timerId: ReturnType<typeof setTimeout>;

    const poll = async () => {
      if (cancelled) return;

      if (circuitOpen.current) {
        timerId = setTimeout(poll, CIRCUIT_BREAKER_BACKOFF_MS);
        return;
      }

      if (!wsConnectedRef.current) {
        try {
          const delta = await fetchRestAuthority();
          if (cancelled) return;
          if (!delta) {
            restFailCount.current += 1;
            if (restFailCount.current >= CIRCUIT_BREAKER_THRESHOLD) {
              circuitOpen.current = true;
              dispatch({ type: "REST_UNAVAILABLE" });
            } else {
              dispatch({ type: "REST_FAIL" });
            }
          } else {
            restFailCount.current = 0;
            circuitOpen.current = false;
            dispatch({ type: "REST_OK", delta });
          }
        } catch {
          if (!cancelled) {
            restFailCount.current += 1;
            if (restFailCount.current >= CIRCUIT_BREAKER_THRESHOLD) {
              circuitOpen.current = true;
              dispatch({ type: "REST_UNAVAILABLE" });
            } else {
              dispatch({ type: "REST_FAIL" });
            }
          }
        }
      }

      if (!cancelled) {
        const interval = wsConnectedRef.current ? REST_POLL_INTERVAL_MS : REST_POLL_DISCONNECTED_MS;
        timerId = setTimeout(poll, interval);
      }
    };

    poll();
    return () => {
      cancelled = true;
      clearTimeout(timerId);
    };
  }, []);

  return useMemo(() => state, [state]);
}

const TerminalSnapshotContext = createContext<TerminalAuthorityState | null>(null);

export function TerminalSnapshotProvider({ children }: { children: ReactNode }) {
  const snapshot = useTerminalSnapshotState();
  return (
    <TerminalSnapshotContext.Provider value={snapshot}>
      {children}
    </TerminalSnapshotContext.Provider>
  );
}

export function useTerminalSnapshot(): TerminalAuthorityState {
  const ctx = useContext(TerminalSnapshotContext);
  if (ctx) return ctx;
  return useTerminalSnapshotState();
}

export type { TerminalAuthorityState, AuthoritySource };
