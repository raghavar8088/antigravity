import { EventBus } from "@/internal/events";
import { orderToExchangeRequest, type ExchangeAdapter, type ExchangeOrderAck } from "@/internal/exchange";
import { OMSV2, type Order } from "@/internal/oms";
import { ExecutionLatencyTracker } from "@/internal/execution/latency_tracker";

export interface ExecutionEngineV2Options {
  exchange: ExchangeAdapter;
  oms: OMSV2;
  eventBus?: EventBus;
  latencyTracker?: ExecutionLatencyTracker;
  maxRetries?: number;
  circuitBreakerFailureLimit?: number;
}

export interface ExecutionResult {
  order: Order;
  ack: ExchangeOrderAck;
  latencyMs: number;
}

class CircuitBreaker {
  private failures = 0;
  private open = false;

  constructor(private readonly failureLimit: number) {}

  canSend(): boolean {
    return !this.open;
  }

  recordSuccess(): void {
    this.failures = 0;
    this.open = false;
  }

  recordFailure(): void {
    this.failures += 1;
    if (this.failures >= this.failureLimit) this.open = true;
  }
}

export class ExecutionEngineV2 {
  private readonly exchange: ExchangeAdapter;
  private readonly oms: OMSV2;
  private readonly eventBus: EventBus;
  private readonly latencyTracker: ExecutionLatencyTracker;
  private readonly maxRetries: number;
  private readonly breaker: CircuitBreaker;

  constructor(opts: ExecutionEngineV2Options) {
    this.exchange = opts.exchange;
    this.oms = opts.oms;
    this.eventBus = opts.eventBus ?? new EventBus();
    this.latencyTracker = opts.latencyTracker ?? new ExecutionLatencyTracker();
    this.maxRetries = Math.max(0, opts.maxRetries ?? 1);
    this.breaker = new CircuitBreaker(Math.max(1, opts.circuitBreakerFailureLimit ?? 3));
  }

  async sendOrder(order: Order): Promise<ExecutionResult> {
    if (!this.breaker.canSend()) {
      const rejected = this.oms.transition(order.orderId, "REJECTED", { reason: "exchange circuit breaker open" });
      this.eventBus.publish("RiskViolation", { order: rejected, reason: rejected.rejectReason }, { correlationId: order.orderId });
      return {
        order: rejected,
        ack: {
          exchangeOrderId: "",
          status: "REJECTED",
          filledQuantity: 0,
          reason: rejected.rejectReason,
          receivedAt: Date.now(),
        },
        latencyMs: 0,
      };
    }

    const started = performance.now();
    this.latencyTracker.mark(order.orderId, "OrderSent");
    let submitted = this.oms.transition(order.orderId, "SUBMITTED");
    this.eventBus.publish("OrderSubmitted", { order: submitted }, { correlationId: order.orderId });

    let ack: ExchangeOrderAck | null = null;
    let lastError: unknown;
    for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
      try {
        ack = await this.exchange.placeOrder(orderToExchangeRequest(submitted));
        break;
      } catch (err) {
        lastError = err;
      }
    }

    if (!ack) {
      this.breaker.recordFailure();
      const reason = lastError instanceof Error ? lastError.message : "exchange order failed";
      const rejected = this.oms.transition(order.orderId, "REJECTED", { reason });
      return {
        order: rejected,
        ack: { exchangeOrderId: "", status: "REJECTED", filledQuantity: 0, reason, receivedAt: Date.now() },
        latencyMs: performance.now() - started,
      };
    }

    this.latencyTracker.mark(order.orderId, "OrderAcknowledged");
    this.breaker.recordSuccess();

    if (ack.status === "REJECTED") {
      submitted = this.oms.transition(order.orderId, "REJECTED", {
        reason: ack.reason,
        exchangeOrderId: ack.exchangeOrderId,
      });
    } else if (ack.status === "PARTIAL") {
      submitted = this.oms.transition(order.orderId, "PARTIAL", {
        exchangeOrderId: ack.exchangeOrderId,
        filledQuantity: ack.filledQuantity,
        averageFillPrice: ack.averageFillPrice,
      });
    } else if (ack.status === "FILLED") {
      this.latencyTracker.mark(order.orderId, "OrderFilled");
      submitted = this.oms.transition(order.orderId, "FILLED", {
        exchangeOrderId: ack.exchangeOrderId,
        filledQuantity: ack.filledQuantity,
        averageFillPrice: ack.averageFillPrice,
      });
      this.eventBus.publish("OrderFilled", { order: submitted, ack }, { correlationId: order.orderId });
    }

    return { order: submitted, ack, latencyMs: performance.now() - started };
  }

  getLatencyTracker(): ExecutionLatencyTracker {
    return this.latencyTracker;
  }
}
