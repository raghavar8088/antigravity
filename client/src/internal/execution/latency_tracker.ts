export type LatencyMark =
  | "SignalGenerated"
  | "SignalApproved"
  | "OrderSent"
  | "OrderAcknowledged"
  | "OrderFilled"
  | "PositionClosed";

export interface LatencyTrace {
  correlationId: string;
  marks: Partial<Record<LatencyMark, number>>;
}

export interface LatencyMetrics {
  signalToOrderMs: number | null;
  orderToFillMs: number | null;
  fillToCloseMs: number | null;
  totalMs: number | null;
}

function delta(start?: number, end?: number): number | null {
  if (start == null || end == null) return null;
  return Math.max(0, end - start);
}

export class ExecutionLatencyTracker {
  private readonly traces = new Map<string, LatencyTrace>();

  mark(correlationId: string, mark: LatencyMark, at = performance.now()): LatencyTrace {
    const trace = this.traces.get(correlationId) ?? { correlationId, marks: {} };
    trace.marks[mark] = at;
    this.traces.set(correlationId, trace);
    return trace;
  }

  getTrace(correlationId: string): LatencyTrace | undefined {
    return this.traces.get(correlationId);
  }

  metrics(correlationId: string): LatencyMetrics {
    const marks = this.traces.get(correlationId)?.marks ?? {};
    return {
      signalToOrderMs: delta(marks.SignalGenerated, marks.OrderSent),
      orderToFillMs: delta(marks.OrderSent, marks.OrderFilled),
      fillToCloseMs: delta(marks.OrderFilled, marks.PositionClosed),
      totalMs: delta(marks.SignalGenerated, marks.PositionClosed ?? marks.OrderFilled ?? marks.OrderAcknowledged),
    };
  }

  snapshot(): LatencyTrace[] {
    return [...this.traces.values()];
  }
}
