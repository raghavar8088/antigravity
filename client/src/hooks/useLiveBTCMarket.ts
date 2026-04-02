"use client";

import { useEffect, useRef, useState } from "react";

export type MarketExchange = "binance" | "bybit";

export type MarketCandle = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

type PricePoint = {
  time: number;
  price: number;
};

type ConnectionState = "bootstrapping" | "connecting" | "live" | "reconnecting" | "error";

export type LiveBTCMarketState = {
  exchange: MarketExchange;
  setExchange: (exchange: MarketExchange) => void;
  price: number;
  prevPrice: number;
  change24h: number;
  high24h: number;
  low24h: number;
  volume24h: number;
  ticksPerSecond: number;
  connected: boolean;
  recentPrices: PricePoint[];
  candles: MarketCandle[];
  connectionState: ConnectionState;
  connectionError: string;
  lastMarketEventAt: number | null;
};

const STORAGE_KEY = "raig.market.exchange";
const MARKET_SYMBOL = "BTCUSDT";
const MAX_RECENT_PRICES = 240;
const MAX_CANDLES = 240;
const RECONNECT_MAX_MS = 15000;

function readStoredExchange(): MarketExchange {
  if (typeof window === "undefined") {
    return "binance";
  }

  const value = window.localStorage.getItem(STORAGE_KEY);
  return value === "bybit" ? "bybit" : "binance";
}

function ensureNumber(value: unknown): number | null {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function normalizeCandle(input: {
  time?: unknown;
  open?: unknown;
  high?: unknown;
  low?: unknown;
  close?: unknown;
  volume?: unknown;
}): MarketCandle | null {
  const time = ensureNumber(input.time);
  const open = ensureNumber(input.open);
  const high = ensureNumber(input.high);
  const low = ensureNumber(input.low);
  const close = ensureNumber(input.close);
  const volume = ensureNumber(input.volume ?? 0);

  if ([time, open, high, low, close, volume].some((value) => value === null)) {
    return null;
  }

  return {
    time: time as number,
    open: open as number,
    high: high as number,
    low: low as number,
    close: close as number,
    volume: volume as number,
  };
}

function minuteBucket(timestamp: number): number {
  return Math.floor(timestamp / 60000) * 60000;
}

function applyTickerToCandles(candles: MarketCandle[], tickPrice: number): MarketCandle[] {
  if (!candles.length || !Number.isFinite(tickPrice)) {
    return candles;
  }

  const nowBucket = minuteBucket(Date.now());
  const last = candles[candles.length - 1];

  if (last.time === nowBucket) {
    const updatedLast = {
      ...last,
      close: tickPrice,
      high: Math.max(last.high, tickPrice),
      low: Math.min(last.low, tickPrice),
    };
    return [...candles.slice(0, -1), updatedLast];
  }

  if (nowBucket > last.time) {
    const open = last.close;
    const nextCandle: MarketCandle = {
      time: nowBucket,
      open,
      close: tickPrice,
      high: Math.max(open, tickPrice),
      low: Math.min(open, tickPrice),
      volume: 0,
    };
    return [...candles.slice(-(MAX_CANDLES - 1)), nextCandle];
  }

  return candles;
}

function upsertCandle(candles: MarketCandle[], input: Partial<MarketCandle>): MarketCandle[] {
  const candle = normalizeCandle(input);
  if (!candle) {
    return candles;
  }

  if (!candles.length) {
    return [candle];
  }

  const existingIndex = candles.findIndex((entry) => entry.time === candle.time);
  if (existingIndex >= 0) {
    const next = candles.slice();
    next[existingIndex] = candle;
    return next;
  }

  if (candle.time > candles[candles.length - 1].time) {
    return [...candles.slice(-(MAX_CANDLES - 1)), candle];
  }

  const insertAt = candles.findIndex((entry) => entry.time > candle.time);
  if (insertAt === -1) {
    return [...candles.slice(-(MAX_CANDLES - 1)), candle];
  }

  const next = candles.slice();
  next.splice(insertAt, 0, candle);
  return next.slice(-MAX_CANDLES);
}

function parseBinanceSocketMessage(message: unknown): { ticker?: number; candle?: MarketCandle } {
  const payload = typeof message === "object" && message !== null && "data" in message
    ? (message as { data?: unknown }).data
    : message;

  if (!payload || typeof payload !== "object") {
    return {};
  }

  const binance = payload as {
    e?: string;
    c?: unknown;
    k?: {
      t?: unknown;
      o?: unknown;
      h?: unknown;
      l?: unknown;
      c?: unknown;
      v?: unknown;
    };
  };

  if (binance.e === "24hrTicker") {
    const ticker = ensureNumber(binance.c);
    return ticker === null ? {} : { ticker };
  }

  if (binance.e === "kline" && binance.k) {
    const candle = normalizeCandle({
      time: binance.k.t,
      open: binance.k.o,
      high: binance.k.h,
      low: binance.k.l,
      close: binance.k.c,
      volume: binance.k.v,
    });
    const ticker = ensureNumber(binance.k.c);
    return {
      ticker: ticker ?? undefined,
      candle: candle ?? undefined,
    };
  }

  return {};
}

function parseBybitSocketMessage(message: unknown): { ticker?: number; candle?: MarketCandle } {
  if (!message || typeof message !== "object") {
    return {};
  }

  const payload = message as {
    op?: string;
    ret_msg?: string;
    topic?: string;
    data?: unknown;
  };

  if (payload.op === "pong" || payload.ret_msg === "pong") {
    return {};
  }

  if (payload.topic === `tickers.${MARKET_SYMBOL}`) {
    const data = Array.isArray(payload.data) ? payload.data[0] : payload.data;
    const ticker = data && typeof data === "object"
      ? ensureNumber((data as { lastPrice?: unknown }).lastPrice)
      : null;
    return ticker === null ? {} : { ticker };
  }

  if (payload.topic === `kline.1.${MARKET_SYMBOL}`) {
    const data = Array.isArray(payload.data) ? payload.data[0] : payload.data;
    if (!data || typeof data !== "object") {
      return {};
    }

    const candle = normalizeCandle({
      time: (data as { start?: unknown; startTime?: unknown; timestamp?: unknown }).start
        ?? (data as { startTime?: unknown }).startTime
        ?? (data as { timestamp?: unknown }).timestamp,
      open: (data as { open?: unknown }).open,
      high: (data as { high?: unknown }).high,
      low: (data as { low?: unknown }).low,
      close: (data as { close?: unknown }).close,
      volume: (data as { volume?: unknown }).volume,
    });
    const ticker = ensureNumber((data as { close?: unknown }).close);

    return {
      ticker: ticker ?? undefined,
      candle: candle ?? undefined,
    };
  }

  return {};
}

async function fetchBootstrapCandles(exchange: MarketExchange): Promise<MarketCandle[]> {
  if (exchange === "binance") {
    const response = await fetch(
      `https://api.binance.com/api/v3/klines?symbol=${MARKET_SYMBOL}&interval=1m&limit=${MAX_CANDLES}`,
    );
    if (!response.ok) {
      throw new Error(`Binance candle bootstrap failed (${response.status})`);
    }

    const rows = (await response.json()) as unknown[];
    return rows
      .map((row) => {
        if (!Array.isArray(row)) {
          return null;
        }
        return normalizeCandle({
          time: row[0],
          open: row[1],
          high: row[2],
          low: row[3],
          close: row[4],
          volume: row[5],
        });
      })
      .filter((row): row is MarketCandle => row !== null)
      .sort((left, right) => left.time - right.time);
  }

  const response = await fetch(
    `https://api.bybit.com/v5/market/kline?category=linear&symbol=${MARKET_SYMBOL}&interval=1&limit=${MAX_CANDLES}`,
  );
  if (!response.ok) {
    throw new Error(`Bybit candle bootstrap failed (${response.status})`);
  }

  const payload = await response.json() as {
    result?: { list?: unknown[] };
  };

  return (payload.result?.list ?? [])
    .map((row) => {
      if (!Array.isArray(row)) {
        return null;
      }
      return normalizeCandle({
        time: row[0],
        open: row[1],
        high: row[2],
        low: row[3],
        close: row[4],
        volume: row[5],
      });
    })
    .filter((row): row is MarketCandle => row !== null)
    .sort((left, right) => left.time - right.time);
}

async function fetchTickerSnapshot(exchange: MarketExchange): Promise<{
  price: number;
  change24h: number;
  high24h: number;
  low24h: number;
  volume24h: number;
}> {
  if (exchange === "binance") {
    const response = await fetch(`https://api.binance.com/api/v3/ticker/24hr?symbol=${MARKET_SYMBOL}`);
    if (!response.ok) {
      throw new Error(`Binance ticker snapshot failed (${response.status})`);
    }

    const payload = await response.json() as {
      lastPrice?: unknown;
      priceChangePercent?: unknown;
      highPrice?: unknown;
      lowPrice?: unknown;
      volume?: unknown;
    };

    return {
      price: ensureNumber(payload.lastPrice) ?? 0,
      change24h: ensureNumber(payload.priceChangePercent) ?? 0,
      high24h: ensureNumber(payload.highPrice) ?? 0,
      low24h: ensureNumber(payload.lowPrice) ?? 0,
      volume24h: ensureNumber(payload.volume) ?? 0,
    };
  }

  const response = await fetch(`https://api.bybit.com/v5/market/tickers?category=linear&symbol=${MARKET_SYMBOL}`);
  if (!response.ok) {
    throw new Error(`Bybit ticker snapshot failed (${response.status})`);
  }

  const payload = await response.json() as {
    result?: {
      list?: Array<{
        lastPrice?: unknown;
        price24hPcnt?: unknown;
        highPrice24h?: unknown;
        lowPrice24h?: unknown;
        volume24h?: unknown;
      }>;
    };
  };
  const ticker = payload.result?.list?.[0];

  return {
    price: ensureNumber(ticker?.lastPrice) ?? 0,
    change24h: (ensureNumber(ticker?.price24hPcnt) ?? 0) * 100,
    high24h: ensureNumber(ticker?.highPrice24h) ?? 0,
    low24h: ensureNumber(ticker?.lowPrice24h) ?? 0,
    volume24h: ensureNumber(ticker?.volume24h) ?? 0,
  };
}

export default function useLiveBTCMarket(): LiveBTCMarketState {
  const [exchange, setExchange] = useState<MarketExchange>(() => readStoredExchange());
  const [state, setState] = useState<Omit<LiveBTCMarketState, "exchange" | "setExchange">>({
    price: 0,
    prevPrice: 0,
    change24h: 0,
    high24h: 0,
    low24h: 0,
    volume24h: 0,
    ticksPerSecond: 0,
    connected: false,
    recentPrices: [],
    candles: [],
    connectionState: "bootstrapping",
    connectionError: "",
    lastMarketEventAt: null,
  });

  const tickCounter = useRef(0);

  useEffect(() => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(STORAGE_KEY, exchange);
    }
  }, [exchange]);

  useEffect(() => {
    let cancelled = false;
    let bootstrapMarker: number | null = null;

    bootstrapMarker = window.setTimeout(() => {
      setState((previous) => ({
        ...previous,
        connectionState: "bootstrapping",
        connectionError: "",
        connected: false,
        ticksPerSecond: 0,
      }));
    }, 0);

    Promise.all([fetchTickerSnapshot(exchange), fetchBootstrapCandles(exchange)])
      .then(([snapshot, candles]) => {
        if (cancelled) {
          return;
        }

        const lastPrice = snapshot.price || candles[candles.length - 1]?.close || 0;
        setState((previous) => ({
          ...previous,
          price: lastPrice,
          prevPrice: lastPrice,
          change24h: snapshot.change24h,
          high24h: snapshot.high24h,
          low24h: snapshot.low24h,
          volume24h: snapshot.volume24h,
          candles,
          recentPrices: candles.slice(-MAX_RECENT_PRICES).map((candle) => ({
            time: candle.time,
            price: candle.close,
          })),
          lastMarketEventAt: candles.length > 0 ? Date.now() : previous.lastMarketEventAt,
        }));
      })
      .catch((error) => {
        if (cancelled) {
          return;
        }

        const message = error instanceof Error ? error.message : "Market bootstrap failed";
        setState((previous) => ({
          ...previous,
          connectionState: "error",
          connectionError: message,
        }));
      });

    return () => {
      cancelled = true;
      if (bootstrapMarker !== null) {
        window.clearTimeout(bootstrapMarker);
      }
    };
  }, [exchange]);

  useEffect(() => {
    let socket: WebSocket | null = null;
    let pingInterval: number | null = null;
    let reconnectTimer: number | null = null;
    let closedByHook = false;
    let attempt = 0;

    const clearTimers = () => {
      if (pingInterval !== null) {
        window.clearInterval(pingInterval);
      }
      if (reconnectTimer !== null) {
        window.clearTimeout(reconnectTimer);
      }
      pingInterval = null;
      reconnectTimer = null;
    };

    const connect = () => {
      if (closedByHook) {
        return;
      }

      setState((previous) => ({
        ...previous,
        connectionState: attempt > 0 ? "reconnecting" : "connecting",
        connectionError: "",
        connected: false,
      }));

      const isBinance = exchange === "binance";
      const url = isBinance
        ? `wss://stream.binance.com:9443/stream?streams=${MARKET_SYMBOL.toLowerCase()}@ticker/${MARKET_SYMBOL.toLowerCase()}@kline_1m`
        : "wss://stream.bybit.com/v5/public/linear";

      socket = new WebSocket(url);

      socket.onopen = () => {
        if (closedByHook || !socket) {
          return;
        }

        attempt = 0;
        setState((previous) => ({
          ...previous,
          connectionState: "live",
          connectionError: "",
          connected: true,
        }));

        if (!isBinance) {
          socket.send(JSON.stringify({
            op: "subscribe",
            args: [`tickers.${MARKET_SYMBOL}`, `kline.1.${MARKET_SYMBOL}`],
          }));

          pingInterval = window.setInterval(() => {
            if (socket?.readyState === WebSocket.OPEN) {
              socket.send(JSON.stringify({ op: "ping" }));
            }
          }, 20000);
        }
      };

      socket.onmessage = (event) => {
        let parsedPayload: { ticker?: number; candle?: MarketCandle };

        try {
          const payload = JSON.parse(event.data) as unknown;
          parsedPayload = exchange === "binance"
            ? parseBinanceSocketMessage(payload)
            : parseBybitSocketMessage(payload);
        } catch {
          return;
        }

        if (parsedPayload.ticker !== undefined) {
          const ticker = parsedPayload.ticker;
          tickCounter.current += 1;
          setState((previous) => ({
            ...previous,
            prevPrice: previous.price || ticker || 0,
            price: ticker,
            connected: true,
            lastMarketEventAt: Date.now(),
            recentPrices: [
              ...previous.recentPrices,
              { time: Date.now(), price: ticker },
            ].slice(-MAX_RECENT_PRICES),
            candles: applyTickerToCandles(previous.candles, ticker),
          }));
        }

        if (parsedPayload.candle) {
          const candle = parsedPayload.candle;
          setState((previous) => ({
            ...previous,
            candles: upsertCandle(previous.candles, candle),
            lastMarketEventAt: Date.now(),
          }));
        }
      };

      socket.onerror = () => {
        if (closedByHook) {
          return;
        }

        setState((previous) => ({
          ...previous,
          connectionState: "error",
          connectionError: `${exchange} websocket error`,
          connected: false,
        }));
      };

      socket.onclose = (event) => {
        clearTimers();
        if (closedByHook) {
          return;
        }

        setState((previous) => ({
          ...previous,
          connectionState: "reconnecting",
          connectionError: `${exchange} socket closed (${event.code})`,
          connected: false,
        }));

        const delay = Math.min(RECONNECT_MAX_MS, 1200 * 1.6 ** attempt);
        attempt += 1;
        reconnectTimer = window.setTimeout(connect, delay);
      };
    };

    connect();

    const tpsInterval = window.setInterval(() => {
      setState((previous) => ({
        ...previous,
        ticksPerSecond: tickCounter.current,
      }));
      tickCounter.current = 0;
    }, 1000);

    return () => {
      closedByHook = true;
      clearTimers();
      window.clearInterval(tpsInterval);
      if (socket && socket.readyState <= WebSocket.OPEN) {
        socket.close();
      }
    };
  }, [exchange]);

  return {
    exchange,
    setExchange,
    ...state,
  };
}
