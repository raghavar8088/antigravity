"use client";
import { useState, useEffect, useRef } from "react";

interface LivePriceState {
  price: number;
  prevPrice: number;
  change24h: number;
  high24h: number;
  low24h: number;
  volume24h: number;
  ticksPerSecond: number;
  connected: boolean;
}

export default function useLiveBTCPrice(): LivePriceState {
  const [state, setState] = useState<LivePriceState>({
    price: 0,
    prevPrice: 0,
    change24h: 0,
    high24h: 0,
    low24h: 0,
    volume24h: 0,
    ticksPerSecond: 0,
    connected: false,
  });

  const tickCounter = useRef(0);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    // 1. Fetch 24h stats from Binance REST API for change%, high, low, volume
    fetch("https://api.binance.com/api/v3/ticker/24hr?symbol=BTCUSDT")
      .then((res) => res.json())
      .then((data) => {
        setState((prev) => ({
          ...prev,
          price: parseFloat(data.lastPrice),
          prevPrice: parseFloat(data.lastPrice),
          change24h: parseFloat(data.priceChangePercent),
          high24h: parseFloat(data.highPrice),
          low24h: parseFloat(data.lowPrice),
          volume24h: parseFloat(data.volume),
        }));
      })
      .catch(console.error);

    // 2. Connect to Binance Live WebSocket for real-time trade stream
    const ws = new WebSocket("wss://stream.binance.com:9443/ws/btcusdt@trade");
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("[ANTIGRAVITY] ✅ Connected to Binance Live BTC Stream");
      setState((prev) => ({ ...prev, connected: true }));
    };

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      const newPrice = parseFloat(data.p);
      tickCounter.current++;

      setState((prev) => ({
        ...prev,
        prevPrice: prev.price,
        price: newPrice,
      }));
    };

    ws.onerror = () => {
      console.error("[ANTIGRAVITY] ❌ WebSocket connection error");
      setState((prev) => ({ ...prev, connected: false }));
    };

    ws.onclose = () => {
      console.log("[ANTIGRAVITY] WebSocket disconnected");
      setState((prev) => ({ ...prev, connected: false }));
    };

    // 3. Track ticks-per-second for the throughput indicator
    const tpsInterval = setInterval(() => {
      setState((prev) => ({
        ...prev,
        ticksPerSecond: tickCounter.current,
      }));
      tickCounter.current = 0;
    }, 1000);

    return () => {
      ws.close();
      clearInterval(tpsInterval);
    };
  }, []);

  return state;
}
