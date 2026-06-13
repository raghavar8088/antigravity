"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import holidays from "@/config/market-holidays.json";

export interface SessionState {
  /** Current IST time as a Date */
  nowIST: Date;
  /** Whether NSE/BSE equity market is currently open */
  nseOpen: boolean;
  /** Descriptive label for NSE session state */
  sessionLabel: "PRE-MARKET" | "MARKET OPEN" | "POST-MARKET" | "CLOSED" | "HOLIDAY";
  /** Whether today is a market holiday */
  isHoliday: boolean;
  /** Crypto is always open */
  cryptoOpen: true;
  /** Seconds until next session event (open or close) */
  timeToNextEvent: number;
  /** Human-readable description of next event */
  nextEventLabel: string;
}

const DUMMY: SessionState = {
  nowIST: new Date(),
  nseOpen: false,
  sessionLabel: "CLOSED",
  isHoliday: false,
  cryptoOpen: true,
  timeToNextEvent: 0,
  nextEventLabel: "—",
};

const SessionCtx = createContext<SessionState>(DUMMY);

function toIST(d: Date): Date {
  return new Date(d.toLocaleString("en-US", { timeZone: "Asia/Kolkata" }));
}

function pad(n: number) {
  return String(n).padStart(2, "0");
}

function hhmm(h: number, m: number) {
  return h * 60 + m;
}

function secsToLabel(secs: number): string {
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const s = secs % 60;
  return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

function computeSession(now: Date): SessionState {
  const ist = toIST(now);
  const yyyy = ist.getFullYear();
  const mm = pad(ist.getMonth() + 1);
  const dd = pad(ist.getDate());
  const todayStr = `${yyyy}-${mm}-${dd}`;

  const holidayList: string[] = (holidays.nse as Record<string, string[]>)[String(yyyy)] ?? [];
  const isHoliday = holidayList.includes(todayStr);

  const totalMins = ist.getHours() * 60 + ist.getMinutes();
  const nowSecs = ist.getSeconds();

  // Session windows (in minutes from midnight)
  const PRE_START  = hhmm(9,  0);  // 09:00
  const NSE_OPEN   = hhmm(9, 15);  // 09:15
  const NSE_CLOSE  = hhmm(15, 30); // 15:30
  const POST_CLOSE = hhmm(16, 0);  // 16:00

  let sessionLabel: SessionState["sessionLabel"];
  let nseOpen = false;
  let timeToNextEvent = 0;
  let nextEventLabel = "—";

  if (isHoliday) {
    sessionLabel = "HOLIDAY";
  } else if (totalMins < PRE_START) {
    sessionLabel = "CLOSED";
    const secsUntilPre = (PRE_START - totalMins) * 60 - nowSecs;
    timeToNextEvent = secsUntilPre;
    nextEventLabel = `Pre-market in ${secsToLabel(secsUntilPre)}`;
  } else if (totalMins < NSE_OPEN) {
    sessionLabel = "PRE-MARKET";
    const secsUntilOpen = (NSE_OPEN - totalMins) * 60 - nowSecs;
    timeToNextEvent = secsUntilOpen;
    nextEventLabel = `Opens in ${secsToLabel(secsUntilOpen)}`;
  } else if (totalMins < NSE_CLOSE) {
    sessionLabel = "MARKET OPEN";
    nseOpen = true;
    const secsUntilClose = (NSE_CLOSE - totalMins) * 60 - nowSecs;
    timeToNextEvent = secsUntilClose;
    nextEventLabel = `Closes in ${secsToLabel(secsUntilClose)}`;
  } else if (totalMins < POST_CLOSE) {
    sessionLabel = "POST-MARKET";
    const secsUntilClose = (POST_CLOSE - totalMins) * 60 - nowSecs;
    timeToNextEvent = secsUntilClose;
    nextEventLabel = `Post-market ends in ${secsToLabel(secsUntilClose)}`;
  } else {
    sessionLabel = "CLOSED";
    // Next open is 09:00 next trading day — simplified to next day
    const secsUntilMidnight = (24 * 60 - totalMins) * 60 - nowSecs;
    const secsUntilNextPre = secsUntilMidnight + PRE_START * 60;
    timeToNextEvent = secsUntilNextPre;
    nextEventLabel = `Next pre-market tomorrow`;
  }

  return {
    nowIST: ist,
    nseOpen,
    sessionLabel,
    isHoliday,
    cryptoOpen: true,
    timeToNextEvent,
    nextEventLabel,
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
