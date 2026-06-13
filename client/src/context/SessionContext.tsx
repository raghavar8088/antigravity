"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

export interface SessionState {
  /** Current UTC time */
  nowUtc: Date;
  /** BTC/crypto markets trade 24/7 */
  cryptoOpen: true;
  sessionLabel: "24/7 LIVE";
  nextEventLabel: string;
}

const DUMMY: SessionState = {
  nowUtc: new Date(),
  cryptoOpen: true,
  sessionLabel: "24/7 LIVE",
  nextEventLabel: "BTC futures — always open",
};

const SessionCtx = createContext<SessionState>(DUMMY);

function computeSession(now: Date): SessionState {
  return {
    nowUtc: now,
    cryptoOpen: true,
    sessionLabel: "24/7 LIVE",
    nextEventLabel: "BTC futures — always open",
  };
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<SessionState>(() => computeSession(new Date()));

  useEffect(() => {
    const tick = () => setState(computeSession(new Date()));
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, []);

  return <SessionCtx.Provider value={state}>{children}</SessionCtx.Provider>;
}

export function useSession(): SessionState {
  return useContext(SessionCtx);
}
