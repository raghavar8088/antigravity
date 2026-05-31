export type ExecutionEventType =
  | "SignalCreated"
  | "SignalRejected"
  | "OrderSubmitted"
  | "OrderFilled"
  | "PositionOpened"
  | "PositionClosed"
  | "RiskViolation";

export interface ExecutionEvent<TPayload = unknown> {
  type: ExecutionEventType;
  payload: TPayload;
  at: number;
  correlationId?: string;
}

export type EventHandler<TPayload = unknown> = (event: ExecutionEvent<TPayload>) => void | Promise<void>;

export class EventBus {
  private readonly handlers = new Map<ExecutionEventType, Set<EventHandler>>();
  private readonly deadLetters: ExecutionEvent[] = [];

  subscribe<TPayload>(
    type: ExecutionEventType,
    handler: EventHandler<TPayload>,
  ): () => void {
    const set = this.handlers.get(type) ?? new Set<EventHandler>();
    set.add(handler as EventHandler);
    this.handlers.set(type, set);
    return () => {
      set.delete(handler as EventHandler);
      if (set.size === 0) this.handlers.delete(type);
    };
  }

  publish<TPayload>(
    type: ExecutionEventType,
    payload: TPayload,
    opts: { at?: number; correlationId?: string } = {},
  ): ExecutionEvent<TPayload> {
    const event: ExecutionEvent<TPayload> = {
      type,
      payload,
      at: opts.at ?? Date.now(),
      correlationId: opts.correlationId,
    };

    const handlers = [...(this.handlers.get(type) ?? [])];
    for (const handler of handlers) {
      queueMicrotask(() => {
        try {
          void handler(event);
        } catch {
          this.deadLetters.push(event);
        }
      });
    }
    return event;
  }

  getDeadLetters(): readonly ExecutionEvent[] {
    return this.deadLetters;
  }

  clearDeadLetters(): void {
    this.deadLetters.length = 0;
  }
}
