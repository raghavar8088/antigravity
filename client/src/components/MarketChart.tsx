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

export default function MarketChart({
  candles,
  positions,
  currentPrice,
  height = 320,
}: MarketChartProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [width, setWidth] = useState(860);

  useEffect(() => {
    const node = canvasRef.current?.parentElement;
    if (!node) {
      return undefined;
    }

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setWidth(Math.max(320, entry.contentRect.width));
      }
    });

    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || candles.length < 20) {
      return;
    }

    const context = canvas.getContext("2d");
    if (!context) {
      return;
    }

    const visible = candles.slice(-80);
    const closes = visible.map((candle) => candle.close);
    const ema9 = calcEMA(closes, 9);
    const ema21 = calcEMA(closes, 21);

    const devicePixelRatio = window.devicePixelRatio || 1;
    canvas.width = width * devicePixelRatio;
    canvas.height = height * devicePixelRatio;
    context.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0);
    context.clearRect(0, 0, width, height);

    const padding = { top: 14, right: 14, bottom: 22, left: 58 };
    const chartWidth = width - padding.left - padding.right;
    const chartHeight = height - padding.top - padding.bottom;
    const candleWidth = chartWidth / Math.max(visible.length, 1);

    let minPrice = Math.min(...visible.flatMap((candle) => [candle.low, candle.high]));
    let maxPrice = Math.max(...visible.flatMap((candle) => [candle.low, candle.high]));

    for (const position of positions) {
      minPrice = Math.min(minPrice, position.entry, position.stopLoss, position.takeProfit);
      maxPrice = Math.max(maxPrice, position.entry, position.stopLoss, position.takeProfit);
    }

    if (currentPrice > 0) {
      minPrice = Math.min(minPrice, currentPrice);
      maxPrice = Math.max(maxPrice, currentPrice);
    }

    const paddingAmount = Math.max((maxPrice - minPrice) * 0.08, 40);
    minPrice -= paddingAmount;
    maxPrice += paddingAmount;
    const priceRange = maxPrice - minPrice || 1;

    const toY = (price: number) => (
      padding.top + chartHeight - ((price - minPrice) / priceRange) * chartHeight
    );

    context.fillStyle = "#050816";
    context.fillRect(0, 0, width, height);

    for (let index = 0; index <= 4; index += 1) {
      const y = padding.top + (chartHeight / 4) * index;
      context.beginPath();
      context.moveTo(padding.left, y);
      context.lineTo(width - padding.right, y);
      context.strokeStyle = "rgba(148, 163, 184, 0.12)";
      context.lineWidth = 1;
      context.stroke();

      const labelPrice = maxPrice - (priceRange / 4) * index;
      context.fillStyle = "rgba(148, 163, 184, 0.85)";
      context.font = "11px ui-monospace, SFMono-Regular, Menlo, monospace";
      context.textAlign = "right";
      context.fillText(labelPrice.toFixed(0), padding.left - 8, y + 4);
    }

    visible.forEach((candle, index) => {
      const x = padding.left + index * candleWidth + candleWidth / 2;
      const color = candle.close >= candle.open ? "#22c55e" : "#ef4444";

      context.strokeStyle = color;
      context.lineWidth = 1;
      context.beginPath();
      context.moveTo(x, toY(candle.high));
      context.lineTo(x, toY(candle.low));
      context.stroke();

      const bodyTop = toY(Math.max(candle.open, candle.close));
      const bodyBottom = toY(Math.min(candle.open, candle.close));
      context.fillStyle = color;
      context.fillRect(
        x - candleWidth * 0.3,
        bodyTop,
        Math.max(3, candleWidth * 0.6),
        Math.max(1.6, bodyBottom - bodyTop),
      );
    });

    const drawLine = (values: number[], color: string) => {
      context.beginPath();
      values.forEach((value, index) => {
        const x = padding.left + index * candleWidth + candleWidth / 2;
        const y = toY(value);
        if (index === 0) {
          context.moveTo(x, y);
        } else {
          context.lineTo(x, y);
        }
      });
      context.strokeStyle = color;
      context.lineWidth = 1.4;
      context.stroke();
    };

    drawLine(ema9, "rgba(56, 189, 248, 0.95)");
    drawLine(ema21, "rgba(251, 191, 36, 0.9)");

    const overlayColors = ["#38bdf8", "#a78bfa", "#22c55e", "#f97316", "#f472b6"];
    positions.forEach((position, index) => {
      const color = overlayColors[index % overlayColors.length];
      const levels = [
        { price: position.entry, label: `E${index + 1}`, color },
        { price: position.stopLoss, label: "SL", color: "#ef4444" },
        { price: position.takeProfit, label: "TP", color: "#22c55e" },
      ];

      for (const level of levels) {
        const y = toY(level.price);
        context.beginPath();
        context.setLineDash([6, 4]);
        context.moveTo(padding.left, y);
        context.lineTo(width - padding.right, y);
        context.strokeStyle = `${level.color}aa`;
        context.lineWidth = 1;
        context.stroke();
        context.setLineDash([]);
        context.fillStyle = level.color;
        context.font = "bold 10px ui-monospace, SFMono-Regular, Menlo, monospace";
        context.textAlign = "right";
        context.fillText(level.label, width - 8, y - 3);
      }
    });

    if (currentPrice > 0) {
      const priceY = toY(currentPrice);
      context.beginPath();
      context.setLineDash([3, 5]);
      context.moveTo(padding.left, priceY);
      context.lineTo(width - padding.right, priceY);
      context.strokeStyle = "rgba(255, 255, 255, 0.45)";
      context.lineWidth = 1;
      context.stroke();
      context.setLineDash([]);

      context.fillStyle = "#e2e8f0";
      context.font = "bold 11px ui-monospace, SFMono-Regular, Menlo, monospace";
      context.textAlign = "left";
      context.fillText(`PX ${currentPrice.toFixed(2)}`, padding.left + 6, priceY - 6);
    }

    const startTime = new Date(visible[0].time).toLocaleTimeString([], {
      hour12: false,
      hour: "2-digit",
      minute: "2-digit",
    });
    const endTime = new Date(visible[visible.length - 1].time).toLocaleTimeString([], {
      hour12: false,
      hour: "2-digit",
      minute: "2-digit",
    });
    context.fillStyle = "rgba(148, 163, 184, 0.8)";
    context.font = "11px ui-monospace, SFMono-Regular, Menlo, monospace";
    context.textAlign = "left";
    context.fillText(startTime, padding.left, height - 6);
    context.textAlign = "right";
    context.fillText(endTime, width - padding.right, height - 6);
  }, [candles, currentPrice, height, positions, width]);

  if (candles.length < 20) {
    return (
      <div className="h-[360px] rounded-2xl border border-zinc-800/80 bg-[#050816] flex items-center justify-center text-sm text-zinc-500 overflow-hidden">
        <div className="flex items-center gap-3">
          <div className="w-2 h-2 rounded-full bg-blue-500 animate-pulse"></div>
          Waiting for live BTC candles to stabilize layout...
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-zinc-800/80 bg-[#050816] p-3">
      <canvas ref={canvasRef} style={{ width: "100%", height, display: "block" }} />
    </div>
  );
}
