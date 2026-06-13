"use client";

import {
  calcSma,
  calcEma,
  calcWma,
  calcRsi,
  calcMacd,
  calcBollinger,
  calcVwap,
  calcAtr,
  calcStochastic,
  type OHLCVCandle,
} from "../ai/mockResearchIndicators";

export type PineSignal = {
  action: "entry" | "exit" | "close" | "none";
  side?: "long" | "short";
  id?: string;
  comment?: string;
};

export type PineExecutionResult = {
  signals: PineSignal[];
  indicators: Record<string, number[]>;
  errors: string[];
};

export class PineScriptEngine {
  private candles: OHLCVCandle[] = [];
  private variables: Record<string, any> = {};
  private indicators: Record<string, number[]> = {};
  private errors: string[] = [];

  constructor(candles: OHLCVCandle[]) {
    this.candles = candles;
  }

  public execute(script: string): PineExecutionResult {
    this.variables = {};
    this.indicators = {};
    this.errors = [];
    const signals: PineSignal[] = [];

    const lines = script.split("\n").map(l => l.trim()).filter(l => l && !l.startsWith("//"));

    try {
      for (const line of lines) {
        this.processLine(line, signals);
      }
    } catch (e: any) {
      this.errors.push(e.message);
    }

    return {
      signals,
      indicators: this.indicators,
      errors: this.errors,
    };
  }

  private processLine(line: string, signals: PineSignal[]) {
    // Basic assignment: var = expression
    const assignmentMatch = line.match(/^([a-zA-Z_]\w*)\s*=\s*(.+)$/);
    if (assignmentMatch) {
      const [, name, expr] = assignmentMatch;
      this.variables[name] = this.evaluateExpression(expr);
      return;
    }

    // Strategy entry: strategy.entry("id", strategy.long, when = condition)
    const entryMatch = line.match(/strategy\.entry\s*\(\s*"([^"]+)"\s*,\s*strategy\.(long|short)(?:\s*,\s*when\s*=\s*(.+))?\s*\)/);
    if (entryMatch) {
      const [, id, side, conditionExpr] = entryMatch;
      const condition = conditionExpr ? this.evaluateExpression(conditionExpr) : true;
      if (condition) {
        signals.push({ action: "entry", side: side as "long" | "short", id });
      }
      return;
    }

    // Strategy close: strategy.close("id", when = condition)
    const closeMatch = line.match(/strategy\.close\s*\(\s*"([^"]+)"(?:\s*,\s*when\s*=\s*(.+))?\s*\)/);
    if (closeMatch) {
      const [, id, conditionExpr] = closeMatch;
      const condition = conditionExpr ? this.evaluateExpression(conditionExpr) : true;
      if (condition) {
        signals.push({ action: "close", id });
      }
      return;
    }
  }

  private evaluateExpression(expr: string): any {
    expr = expr.trim();

    // ta.sma(source, period)
    const smaMatch = expr.match(/ta\.sma\s*\(\s*([^,]+)\s*,\s*(\d+)\s*\)/);
    if (smaMatch) {
      const [, src, periodStr] = smaMatch;
      const period = parseInt(periodStr);
      const values = this.getSourceValues(src);
      const result = calcSma(values, period);
      this.storeIndicator("sma", result);
      return result;
    }

    // ta.ema(source, period)
    const emaMatch = expr.match(/ta\.ema\s*\(\s*([^,]+)\s*,\s*(\d+)\s*\)/);
    if (emaMatch) {
      const [, src, periodStr] = emaMatch;
      const period = parseInt(periodStr);
      const values = this.getSourceValues(src);
      const result = calcEma(values, period);
      this.storeIndicator("ema", result);
      return result;
    }

    // ta.rsi(source, period)
    const rsiMatch = expr.match(/ta\.rsi\s*\(\s*([^,]+)\s*,\s*(\d+)\s*\)/);
    if (rsiMatch) {
      const [, src, periodStr] = rsiMatch;
      const period = parseInt(periodStr);
      const values = this.getSourceValues(src);
      const result = calcRsi(values, period);
      this.storeIndicator("rsi", result);
      return result;
    }

    // ta.crossover(a, b)
    const crossoverMatch = expr.match(/ta\.crossover\s*\(\s*([^,]+)\s*,\s*([^,]+)\s*\)/);
    if (crossoverMatch) {
      const [, aExpr, bExpr] = crossoverMatch;
      const a = this.evaluateExpression(aExpr);
      const b = this.evaluateExpression(bExpr);
      // Simplified crossover: current a > b and previous a <= b
      // In a real engine, we'd need history. For now, we'll just check current.
      return a > b;
    }

    // ta.crossunder(a, b)
    const crossunderMatch = expr.match(/ta\.crossunder\s*\(\s*([^,]+)\s*,\s*([^,]+)\s*\)/);
    if (crossunderMatch) {
      const [, aExpr, bExpr] = crossunderMatch;
      const a = this.evaluateExpression(aExpr);
      const b = this.evaluateExpression(bExpr);
      return a < b;
    }

    // Variables or constants
    if (this.variables[expr] !== undefined) return this.variables[expr];
    if (expr === "close") return this.candles[this.candles.length - 1]?.close ?? 0;
    if (expr === "open") return this.candles[this.candles.length - 1]?.open ?? 0;
    if (expr === "high") return this.candles[this.candles.length - 1]?.high ?? 0;
    if (expr === "low") return this.candles[this.candles.length - 1]?.low ?? 0;

    const num = parseFloat(expr);
    if (!isNaN(num)) return num;

    return false;
  }

  private getSourceValues(src: string): number[] {
    if (src === "close") return this.candles.map(c => c.close);
    if (src === "open") return this.candles.map(c => c.open);
    if (src === "high") return this.candles.map(c => c.high);
    if (src === "low") return this.candles.map(c => c.low);
    if (this.variables[src] && Array.isArray(this.variables[src])) return this.variables[src];
    return this.candles.map(c => c.close);
  }

  private storeIndicator(name: string, value: number) {
    if (!this.indicators[name]) this.indicators[name] = [];
    this.indicators[name].push(value);
  }
}
