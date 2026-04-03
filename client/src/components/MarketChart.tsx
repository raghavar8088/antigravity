"use client";

import { useEffect, useRef, useState } from "react";
import type { MarketCandle } from "@/hooks/useLiveBTCMarket";
import { calcEMA } from "@/lib/marketSignal";

type ChartPosition = {
  id: string;
  strategy: string;
  side: "LONG" | "SHORT";
  entry: number;
  stopLoss: number;
  takeProfit: number;
};

type MarketChartProps = {
  candles: MarketCandle[];
  positions: ChartPosition[];
  currentPrice: number;
  height?: number;
};

export default function MarketChart({ candles, positions, currentPrice, height = 320 }: MarketChartProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [width, setWidth] = useState(860);

  useEffect(() => {
    const node = canvasRef.current?.parentElement;
    if (!node) return undefined;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) setWidth(Math.max(320, entry.contentRect.width));
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || candles.length < 20) return;
    const context = canvas.getContext("2d");
    if (!context) return;

    const visible = candles.slice(-80);
    const closes = visible.map((c) => c.close);
    const ema9 = calcEMA(closes, 9);
    const ema21 = calcEMA(closes, 21);
    const ratio = window.devicePixelRatio || 1;
    canvas.width = width * ratio;
    canvas.height = height * ratio;
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    context.clearRect(0, 0, width, height);

    const padding = { top: 18, right: 14, bottom: 24, left: 58 };
    const chartWidth = width - padding.left - padding.right;
    const chartHeight = height - padding.top - padding.bottom;
    const candleWidth = chartWidth / Math.max(visible.length, 1);

    let minPrice = Math.min(...visible.flatMap((c) => [c.low, c.high]));
    let maxPrice = Math.max(...visible.flatMap((c) => [c.low, c.high]));
    for (const position of positions) {
      minPrice = Math.min(minPrice, position.entry, position.stopLoss, position.takeProfit);
      maxPrice = Math.max(maxPrice, position.entry, position.stopLoss, position.takeProfit);
    }
    if (currentPrice > 0) {
      minPrice = Math.min(minPrice, currentPrice);
      maxPrice = Math.max(maxPrice, currentPrice);
    }

    const rangePad = Math.max((maxPrice - minPrice) * 0.08, 30);
    minPrice -= rangePad;
    maxPrice += rangePad;
    const priceRange = maxPrice - minPrice || 1;
    const toY = (price: number) => padding.top + chartHeight - ((price - minPrice) / priceRange) * chartHeight;

    const cssVars = getComputedStyle(document.documentElement);
    const bgColor = cssVars.getPropertyValue("--surface").trim() || "#ffffff";
    const gridColor = cssVars.getPropertyValue("--border-subtle").trim() || "rgba(60,64,67,0.10)";
    const labelColor = cssVars.getPropertyValue("--text-muted").trim() || "#80868b";
    const priceTextColor = cssVars.getPropertyValue("--text-primary").trim() || "#202124";

    context.fillStyle = bgColor;
    context.fillRect(0, 0, width, height);

    for (let index = 0; index <= 4; index += 1) {
      const y = padding.top + (chartHeight / 4) * index;
      context.beginPath();
      context.moveTo(padding.left, y);
      context.lineTo(width - padding.right, y);
      context.strokeStyle = gridColor;
      context.stroke();
      const labelPrice = maxPrice - (priceRange / 4) * index;
      context.fillStyle = labelColor;
      context.font = "11px Roboto Mono, monospace";
      context.textAlign = "right";
      context.fillText(labelPrice.toFixed(0), padding.left - 8, y + 4);
    }

    visible.forEach((candle, index) => {
      const x = padding.left + index * candleWidth + candleWidth / 2;
      const color = candle.close >= candle.open ? "#188038" : "#d93025";
      context.strokeStyle = color;
      context.beginPath();
      context.moveTo(x, toY(candle.high));
      context.lineTo(x, toY(candle.low));
      context.stroke();
      const top = toY(Math.max(candle.open, candle.close));
      const bottom = toY(Math.min(candle.open, candle.close));
      context.fillStyle = color;
      context.fillRect(x - candleWidth * 0.28, top, Math.max(3, candleWidth * 0.56), Math.max(1.6, bottom - top));
    });

    const drawLine = (values: number[], color: string) => {
      context.beginPath();
      values.forEach((value, index) => {
        const x = padding.left + index * candleWidth + candleWidth / 2;
        const y = toY(value);
        if (index === 0) context.moveTo(x, y);
        else context.lineTo(x, y);
      });
      context.strokeStyle = color;
      context.lineWidth = 1.5;
      context.stroke();
    };

    drawLine(ema9, "#1a73e8");
    drawLine(ema21, "#c58b00");

    positions.forEach((position) => {
      [
        { price: position.entry, label: "ENTRY", color: "#1a73e8" },
        { price: position.stopLoss, label: "SL", color: "#d93025" },
        { price: position.takeProfit, label: "TP", color: "#188038" },
      ].forEach((level) => {
        const y = toY(level.price);
        context.beginPath();
        context.setLineDash([5, 4]);
        context.moveTo(padding.left, y);
        context.lineTo(width - padding.right, y);
        context.strokeStyle = `${level.color}88`;
        context.stroke();
        context.setLineDash([]);
        context.fillStyle = level.color;
        context.font = "10px Roboto Mono, monospace";
        context.textAlign = "right";
        context.fillText(level.label, width - 8, y - 3);
      });
    });

    if (currentPrice > 0) {
      const y = toY(currentPrice);
      context.beginPath();
      context.setLineDash([3, 5]);
      context.moveTo(padding.left, y);
      context.lineTo(width - padding.right, y);
      context.strokeStyle = "rgba(32, 33, 36, 0.36)";
      context.stroke();
      context.setLineDash([]);
      context.fillStyle = priceTextColor;
      context.font = "bold 11px Roboto Mono, monospace";
      context.textAlign = "left";
      context.fillText(`PX ${currentPrice.toFixed(2)}`, padding.left + 6, y - 6);
    }
  }, [candles, currentPrice, height, positions, width]);

  if (candles.length < 20) {
    return (
      <div className="flex items-center justify-center rounded-[20px] border text-sm" style={{ height: 360, borderColor: "var(--border)", background: "var(--surface-2)", color: "var(--text-secondary)" }}>
        Waiting for live BTC candles...
      </div>
    );
  }

  return (
    <div className="rounded-[20px] border p-3" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
      <canvas ref={canvasRef} style={{ width: "100%", height, display: "block" }} />
    </div>
  );
}
