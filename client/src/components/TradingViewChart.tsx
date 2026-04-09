"use client";

import { useEffect, useRef, useState } from "react";

const SYMBOLS = [
  { label: "NIFTY 50", value: "NSE:NIFTY" },
  { label: "BANK NIFTY", value: "NSE:BANKNIFTY" },
  { label: "SENSEX", value: "BSE:SENSEX" },
  { label: "BTC/USD", value: "BINANCE:BTCUSDT" },
  { label: "ETH/USD", value: "BINANCE:ETHUSDT" },
  { label: "CRUDE OIL", value: "MCX:CRUDEOIL1!" },
  { label: "GOLD", value: "MCX:GOLD1!" },
  { label: "SILVER", value: "MCX:SILVER1!" },
  { label: "NATURAL GAS", value: "MCX:NATURALGAS1!" },
  { label: "COPPER", value: "MCX:COPPER1!" },
];

const INTERVALS = [
  { label: "1m", value: "1" },
  { label: "3m", value: "3" },
  { label: "5m", value: "5" },
  { label: "15m", value: "15" },
  { label: "30m", value: "30" },
  { label: "1h", value: "60" },
  { label: "4h", value: "240" },
  { label: "1D", value: "D" },
  { label: "1W", value: "W" },
];

const CHART_STYLES = [
  { label: "Candles", value: "1" },
  { label: "Bars", value: "0" },
  { label: "Line", value: "2" },
  { label: "Area", value: "3" },
  { label: "Heikin Ashi", value: "8" },
];

export default function TradingViewChart() {
  const containerRef = useRef<HTMLDivElement>(null);
  const scriptRef = useRef<HTMLScriptElement | null>(null);

  const [symbol, setSymbol] = useState("NSE:NIFTY");
  const [interval, setInterval] = useState("5");
  const [chartStyle, setChartStyle] = useState("1");
  const [theme, setTheme] = useState<"dark" | "light">("dark");

  useEffect(() => {
    if (!containerRef.current) return;

    // Remove any previous widget
    const widgetDiv = containerRef.current.querySelector(".tradingview-widget-container__widget");
    if (widgetDiv) widgetDiv.innerHTML = "";
    if (scriptRef.current) {
      scriptRef.current.remove();
      scriptRef.current = null;
    }

    const config = {
      autosize: true,
      symbol,
      interval,
      style: chartStyle,
      theme,
      locale: "en",
      timezone: "Asia/Kolkata",
      toolbar_bg: theme === "dark" ? "#1a1a2e" : "#ffffff",
      backgroundColor: theme === "dark" ? "#0f0f23" : "#ffffff",
      gridColor: theme === "dark" ? "rgba(255,255,255,0.04)" : "rgba(0,0,0,0.06)",
      hide_side_toolbar: false,
      hide_top_toolbar: false,
      hide_legend: false,
      hide_volume: false,
      allow_symbol_change: true,
      save_image: true,
      withdateranges: true,
      calendar: false,
      hotlist: false,
      details: false,
      studies: [
        "RSI@tv-basicstudies",
        "MACD@tv-basicstudies",
      ],
      compareSymbols: [],
      watchlist: [],
    };

    const script = document.createElement("script");
    script.type = "text/javascript";
    script.src = "https://s3.tradingview.com/external-embedding/embed-widget-advanced-chart.js";
    script.async = true;
    script.innerHTML = JSON.stringify(config);

    const widgetContainer = containerRef.current.querySelector(".tradingview-widget-container__widget");
    if (widgetContainer) widgetContainer.appendChild(script);
    scriptRef.current = script;

    return () => {
      if (scriptRef.current) scriptRef.current.remove();
    };
  }, [symbol, interval, chartStyle, theme]);

  const btnBase = "px-3 py-1 rounded text-xs font-medium transition-colors";
  const btnActive = "bg-blue-600 text-white";
  const btnInactive = "bg-[#1e1e3a] text-[#a0a0c0] hover:bg-[#252545]";

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", gap: 0 }}>
      {/* Toolbar */}
      <div style={{
        display: "flex",
        alignItems: "center",
        gap: 12,
        padding: "10px 16px",
        background: "#0f0f23",
        borderBottom: "1px solid #1e1e3a",
        flexWrap: "wrap",
      }}>
        {/* Symbol selector */}
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <span style={{ fontSize: 11, color: "#6b6b8a", fontWeight: 600, letterSpacing: 1 }}>SYMBOL</span>
          <select
            value={symbol}
            onChange={(e) => setSymbol(e.target.value)}
            style={{
              background: "#1e1e3a",
              border: "1px solid #2a2a4a",
              color: "#e0e0ff",
              fontSize: 12,
              padding: "4px 8px",
              borderRadius: 6,
              cursor: "pointer",
            }}
          >
            {SYMBOLS.map((s) => (
              <option key={s.value} value={s.value}>{s.label}</option>
            ))}
          </select>
        </div>

        {/* Interval pills */}
        <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
          <span style={{ fontSize: 11, color: "#6b6b8a", fontWeight: 600, letterSpacing: 1, marginRight: 4 }}>TF</span>
          {INTERVALS.map((iv) => (
            <button
              key={iv.value}
              onClick={() => setInterval(iv.value)}
              className={`${btnBase} ${interval === iv.value ? btnActive : btnInactive}`}
            >
              {iv.label}
            </button>
          ))}
        </div>

        {/* Chart style */}
        <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
          <span style={{ fontSize: 11, color: "#6b6b8a", fontWeight: 600, letterSpacing: 1, marginRight: 4 }}>STYLE</span>
          {CHART_STYLES.map((cs) => (
            <button
              key={cs.value}
              onClick={() => setChartStyle(cs.value)}
              className={`${btnBase} ${chartStyle === cs.value ? btnActive : btnInactive}`}
            >
              {cs.label}
            </button>
          ))}
        </div>

        {/* Theme toggle */}
        <button
          onClick={() => setTheme((t) => t === "dark" ? "light" : "dark")}
          className={`${btnBase} ml-auto`}
          style={{ background: "#1e1e3a", color: "#a0a0c0", border: "1px solid #2a2a4a" }}
        >
          {theme === "dark" ? "☀ Light" : "🌙 Dark"}
        </button>
      </div>

      {/* TradingView Widget */}
      <div
        ref={containerRef}
        className="tradingview-widget-container"
        style={{ flex: 1, minHeight: 0 }}
      >
        <div
          className="tradingview-widget-container__widget"
          style={{ height: "100%", width: "100%" }}
        />
      </div>
    </div>
  );
}
