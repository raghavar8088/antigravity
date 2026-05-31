import { describe, expect, it } from "vitest";
import { OMSV2, OrderStateMachine } from "@/internal/oms";

describe("OMSV2", () => {
  it("enforces institutional order state transitions", () => {
    const oms = new OMSV2();
    const order = oms.createOrder({
      symbol: "BTCUSD",
      side: "BUY",
      quantity: 1,
      now: 1,
    });

    const submitted = oms.transition(order.orderId, "SUBMITTED", { now: 2 });
    const partial = oms.transition(order.orderId, "PARTIAL", { now: 3, filledQuantity: 0.4 });
    const filled = oms.transition(order.orderId, "FILLED", { now: 4, filledQuantity: 1, averageFillPrice: 100 });
    const closed = oms.transition(order.orderId, "CLOSED", { now: 5 });

    expect(submitted.state).toBe("SUBMITTED");
    expect(partial.state).toBe("PARTIAL");
    expect(filled.state).toBe("FILLED");
    expect(closed.state).toBe("CLOSED");
    expect(closed.stateLog.map((row) => row.to)).toEqual(["SUBMITTED", "PARTIAL", "FILLED", "CLOSED"]);
  });

  it("rejects invalid transitions", () => {
    const machine = new OrderStateMachine();
    const oms = new OMSV2();
    const order = oms.createOrder({ symbol: "BTCUSD", side: "SELL", quantity: 1 });

    expect(machine.canTransition("PENDING", "FILLED")).toBe(false);
    expect(() => oms.transition(order.orderId, "FILLED")).toThrow("invalid order transition");
  });
});
