import type { Signal } from "@/internal/strategy/evaluator";

export type OrderSide = "BUY" | "SELL";
export type OrderType = "MARKET" | "LIMIT";
export type OrderMode = "PAPER" | "LIVE";
export type OrderState =
  | "PENDING"
  | "SUBMITTED"
  | "PARTIAL"
  | "FILLED"
  | "REJECTED"
  | "CANCELLED"
  | "CLOSED";

export interface OrderStateChange {
  from: OrderState;
  to: OrderState;
  at: number;
  reason?: string;
}

export interface Order {
  orderId: string;
  clientOrderId: string;
  symbol: string;
  side: OrderSide;
  type: OrderType;
  mode: OrderMode;
  state: OrderState;
  quantity: number;
  price?: number;
  signal?: Signal;
  createdAt: number;
  updatedAt: number;
  exchangeOrderId?: string;
  filledQuantity: number;
  averageFillPrice?: number;
  rejectReason?: string;
  stateLog: OrderStateChange[];
}

export interface OrderCreateRequest {
  symbol: string;
  side: OrderSide;
  type?: OrderType;
  mode?: OrderMode;
  quantity: number;
  price?: number;
  signal?: Signal;
  now?: number;
}

const VALID_TRANSITIONS: Readonly<Record<OrderState, readonly OrderState[]>> = {
  PENDING: ["SUBMITTED", "REJECTED", "CANCELLED"],
  SUBMITTED: ["PARTIAL", "FILLED", "REJECTED", "CANCELLED"],
  PARTIAL: ["FILLED", "CANCELLED", "REJECTED"],
  FILLED: ["CLOSED"],
  REJECTED: [],
  CANCELLED: [],
  CLOSED: [],
};

function id(prefix: string): string {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `${prefix}_${random}`;
}

export class OrderStateMachine {
  canTransition(from: OrderState, to: OrderState): boolean {
    return VALID_TRANSITIONS[from].includes(to);
  }

  transition(
    order: Order,
    to: OrderState,
    opts: {
      now?: number;
      reason?: string;
      exchangeOrderId?: string;
      filledQuantity?: number;
      averageFillPrice?: number;
    } = {},
  ): Order {
    if (!this.canTransition(order.state, to)) {
      throw new Error(`invalid order transition ${order.state} -> ${to}`);
    }

    const now = opts.now ?? Date.now();
    return {
      ...order,
      state: to,
      updatedAt: now,
      exchangeOrderId: opts.exchangeOrderId ?? order.exchangeOrderId,
      filledQuantity: opts.filledQuantity ?? order.filledQuantity,
      averageFillPrice: opts.averageFillPrice ?? order.averageFillPrice,
      rejectReason: to === "REJECTED" ? opts.reason : order.rejectReason,
      stateLog: [
        ...order.stateLog,
        { from: order.state, to, at: now, reason: opts.reason },
      ],
    };
  }
}

export class OMSV2 {
  private readonly machine = new OrderStateMachine();
  private readonly orders = new Map<string, Order>();

  createOrder(req: OrderCreateRequest): Order {
    const now = req.now ?? Date.now();
    const order: Order = {
      orderId: id("ord"),
      clientOrderId: id("cl"),
      symbol: req.symbol,
      side: req.side,
      type: req.type ?? "MARKET",
      mode: req.mode ?? "PAPER",
      state: "PENDING",
      quantity: req.quantity,
      price: req.price,
      signal: req.signal,
      createdAt: now,
      updatedAt: now,
      filledQuantity: 0,
      stateLog: [],
    };
    this.orders.set(order.orderId, order);
    return order;
  }

  transition(orderId: string, to: OrderState, opts: Parameters<OrderStateMachine["transition"]>[2] = {}): Order {
    const order = this.orders.get(orderId);
    if (!order) throw new Error(`unknown order ${orderId}`);
    const next = this.machine.transition(order, to, opts);
    this.orders.set(orderId, next);
    return next;
  }

  getOrder(orderId: string): Order | undefined {
    return this.orders.get(orderId);
  }

  listOrders(): Order[] {
    return [...this.orders.values()];
  }
}
