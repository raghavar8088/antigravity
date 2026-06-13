"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { submitExecutionRequest } from "@/lib/trading/executionRequest";

const CONFIG_LS_KEY = "delta_api_config_v1";

type ApiConfig = { apiKey: string; apiSecret: string; testnet: boolean };
type OrderType = "market_order" | "limit_order";
type Side = "buy" | "sell";

type OrderResult = {
  ok: boolean;
  orderId?: string;
  symbol?: string;
  productId?: number;
  contracts?: number;
  fillPrice?: number;
  state?: string;
  side?: string;
  orderType?: string;
  error?: string;
  debug?: string;
};

type SpotProduct = {
  productId: number;
  symbol: string;
  contractSize: number;
  debugInfo?: string;
};

function loadStoredConfig(): ApiConfig {
  if (typeof window === "undefined") {
    return { apiKey: "", apiSecret: "", testnet: false };
  }
  try {
    const stored = localStorage.getItem(CONFIG_LS_KEY);
    if (stored) return JSON.parse(stored) as ApiConfig;
  } catch {
    // ignore
  }
  return { apiKey: "", apiSecret: "", testnet: false };
}

function fmt(n: number, dp = 2) {
  return n.toLocaleString("en-US", { minimumFractionDigits: dp, maximumFractionDigits: dp });
}

const CARD: React.CSSProperties = {
  background: "var(--surface-2)",
  border: "1px solid var(--border-subtle)",
  borderRadius: "var(--radius-card)",
  padding: "16px",
};

const INPUT: React.CSSProperties = {
  width: "100%",
  background: "var(--surface-3)",
  border: "1px solid var(--border-subtle)",
  borderRadius: "var(--radius-sm)",
  color: "var(--text-primary)",
  padding: "8px 10px",
  fontSize: 13,
  fontFamily: "var(--font-mono)",
  outline: "none",
  boxSizing: "border-box",
};

const LABEL: React.CSSProperties = {
  fontSize: 11,
  color: "var(--text-muted)",
  fontWeight: 600,
  letterSpacing: "0.05em",
  marginBottom: 4,
  display: "block",
  fontFamily: "var(--font-display)",
};

function Tab({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      style={{
        padding: "6px 16px",
        fontSize: 12,
        fontWeight: 600,
        fontFamily: "var(--font-display)",
        background: active ? "var(--accent)" : "transparent",
        color: active ? "#fff" : "var(--text-secondary)",
        border: `1px solid ${active ? "var(--accent)" : "var(--border-subtle)"}`,
        borderRadius: "var(--radius-chip)",
        cursor: "pointer",
        transition: "all 0.15s",
      }}
    >
      {label}
    </button>
  );
}

function SideTab({ label, active, isBuy, onClick }: { label: string; active: boolean; isBuy: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      style={{
        flex: 1,
        padding: "8px 0",
        fontSize: 13,
        fontWeight: 700,
        fontFamily: "var(--font-display)",
        background: active
          ? isBuy ? "rgba(30, 142, 62, 0.25)" : "rgba(217, 48, 37, 0.25)"
          : "transparent",
        color: active
          ? isBuy ? "var(--green)" : "var(--red)"
          : "var(--text-muted)",
        border: `1px solid ${active ? (isBuy ? "rgba(30,142,62,0.4)" : "rgba(217,48,37,0.4)") : "var(--border-subtle)"}`,
        borderRadius: "var(--radius-sm)",
        cursor: "pointer",
        transition: "all 0.15s",
      }}
    >
      {label}
    </button>
  );
}

export default function DeltaSpotBuy() {
  const [config, setConfig] = useState<ApiConfig>(() => loadStoredConfig());
  const [showConfig, setShowConfig] = useState(false);
  const [configInput, setConfigInput] = useState<ApiConfig>(() => loadStoredConfig());

  const [side, setSide] = useState<Side>("buy");
  const [orderType, setOrderType] = useState<OrderType>("market_order");
  const [usdtAmount, setUsdtAmount] = useState("100");
  const [limitPrice, setLimitPrice] = useState("");

  const [btcPrice, setBtcPrice] = useState<number | null>(null);
  const [priceTs, setPriceTs] = useState<number>(0);
  const [nowTs, setNowTs] = useState<number>(() => Date.now());

  const [spotProduct, setSpotProduct] = useState<SpotProduct | null>(null);
  const [probing, setProbing] = useState(false);
  const [probeError, setProbeError] = useState("");

  const [placing, setPlacing] = useState(false);
  const [lastOrder, setLastOrder] = useState<OrderResult | null>(null);
  const [orders, setOrders] = useState<OrderResult[]>([]);

  const wsRef = useRef<WebSocket | null>(null);

  // Live BTC price via Binance WebSocket
  useEffect(() => {
    const connect = () => {
      const ws = new WebSocket("wss://stream.binance.com:9443/ws/btcusdt@trade");
      wsRef.current = ws;
      ws.onmessage = (e) => {
        try {
          const d = JSON.parse(e.data as string) as { p?: string };
          const p = parseFloat(d.p ?? "0");
          if (p > 0) { setBtcPrice(p); setPriceTs(Date.now()); }
        } catch { /* ignore */ }
      };
      ws.onclose = () => { setTimeout(connect, 3000); };
    };
    connect();
    return () => { wsRef.current?.close(); };
  }, []);

  useEffect(() => {
    const interval = setInterval(() => setNowTs(Date.now()), 1000);
    return () => clearInterval(interval);
  }, []);

  const authHeaders = useCallback(() => ({
    "x-delta-api-key": config.apiKey,
    "x-delta-api-secret": config.apiSecret,
    "x-delta-testnet": config.testnet ? "true" : "false",
  }), [config]);

  const saveConfig = useCallback(() => {
    setConfig(configInput);
    localStorage.setItem(CONFIG_LS_KEY, JSON.stringify(configInput));
    setShowConfig(false);
  }, [configInput]);

  const probeProduct = useCallback(async () => {
    setProbing(true);
    setProbeError("");
    try {
      const res = await fetch("/api/delta/spot", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ action: "probe" }),
      });
      const data = await res.json() as { ok: boolean; product?: SpotProduct };
      if (data.ok && data.product?.productId) {
        setSpotProduct(data.product);
      } else {
        setProbeError((data.product as { debugInfo?: string } | undefined)?.debugInfo ?? "Product not found");
      }
    } catch (e) {
      setProbeError(String(e));
    }
    setProbing(false);
  }, [authHeaders]);

  const placeOrder = useCallback(async () => {
    const amount = parseFloat(usdtAmount);
    if (!amount || amount <= 0) return;
    if (!config.apiKey || !config.apiSecret) {
      setLastOrder({ ok: false, error: "API keys not configured — click ⚙ to set them" });
      return;
    }
    setPlacing(true);
    setLastOrder(null);
    try {
      const payload: Record<string, unknown> = {
        action: side,
        orderType,
        usdtAmount: amount,
      };
      if (orderType === "limit_order") {
        const lp = parseFloat(limitPrice);
        if (!lp || lp <= 0) {
          setLastOrder({ ok: false, error: "Enter a valid limit price" });
          setPlacing(false);
          return;
        }
        payload.limitPrice = lp;
      }
      if (spotProduct?.productId) {
        payload.productId = spotProduct.productId;
      }

      const result = await submitExecutionRequest({
        venue: "delta",
        symbol: spotProduct?.symbol ?? "BTCUSDT",
        side,
        size: amount / 100,
        strategyName: "DELTA_SPOT_UI",
        reason: "spot_buy_panel",
      });
      const data: OrderResult = {
        ok: result.ok,
        error: result.ok ? undefined : result.message,
        orderId: result.clientOrderId,
        side,
        orderType,
      };
      setLastOrder(data);
      if (data.ok) {
        setOrders((prev) => [{ ...data, side }, ...prev.slice(0, 49)]);
      }
    } catch (e) {
      setLastOrder({ ok: false, error: String(e) });
    }
    setPlacing(false);
  }, [usdtAmount, side, orderType, limitPrice, spotProduct, config, authHeaders]);

  const priceStale = nowTs - priceTs > 10000;
  const estimatedBTC = btcPrice && parseFloat(usdtAmount) > 0
    ? parseFloat(usdtAmount) / btcPrice
    : null;

  const configured = !!(config.apiKey && config.apiSecret);

  return (
    <div style={{ padding: "20px", maxWidth: 680, margin: "0 auto", display: "flex", flexDirection: "column", gap: 16 }}>

      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <div>
          <div style={{ fontSize: 18, fontWeight: 700, color: "var(--text-primary)", fontFamily: "var(--font-display)" }}>
            BTC Spot Order
          </div>
          <div style={{ fontSize: 12, color: "var(--text-muted)", marginTop: 2 }}>
            Delta Exchange India · Live Market
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          {/* Live price badge */}
          {btcPrice && (
            <div style={{
              padding: "6px 12px",
              borderRadius: "var(--radius-chip)",
              background: priceStale ? "var(--surface-3)" : "var(--green-dim)",
              border: `1px solid ${priceStale ? "var(--border-subtle)" : "rgba(30,142,62,0.3)"}`,
              fontSize: 13,
              fontWeight: 700,
              fontFamily: "var(--font-mono)",
              color: priceStale ? "var(--text-muted)" : "var(--green)",
            }}>
              ${fmt(btcPrice, 1)}
            </div>
          )}
          <button
            type="button"
            onClick={() => { setShowConfig((v) => !v); setConfigInput(config); }}
            title="API Settings"
            style={{
              width: 32, height: 32, borderRadius: "var(--radius-sm)",
              background: configured ? "var(--green-dim)" : "var(--red-dim)",
              border: `1px solid ${configured ? "rgba(30,142,62,0.3)" : "rgba(217,48,37,0.3)"}`,
              color: configured ? "var(--green)" : "var(--red)",
              cursor: "pointer", fontSize: 14, display: "flex", alignItems: "center", justifyContent: "center",
            }}
          >
            ⚙
          </button>
        </div>
      </div>

      {/* Config panel */}
      {showConfig && (
        <div style={{ ...CARD, border: "1px solid var(--accent-dim)" }}>
          <div style={{ fontSize: 13, fontWeight: 700, color: "var(--accent)", marginBottom: 12, fontFamily: "var(--font-display)" }}>
            Delta Exchange API Keys
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            <div>
              <span style={LABEL}>API Key</span>
              <input
                style={INPUT}
                type="text"
                placeholder="Your Delta India API key"
                value={configInput.apiKey}
                onChange={(e) => setConfigInput((c) => ({ ...c, apiKey: e.target.value }))}
              />
            </div>
            <div>
              <span style={LABEL}>API Secret</span>
              <input
                style={INPUT}
                type="password"
                placeholder="Your Delta India API secret"
                value={configInput.apiSecret}
                onChange={(e) => setConfigInput((c) => ({ ...c, apiSecret: e.target.value }))}
              />
            </div>
            <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12, color: "var(--text-secondary)", cursor: "pointer" }}>
              <input
                type="checkbox"
                checked={configInput.testnet}
                onChange={(e) => setConfigInput((c) => ({ ...c, testnet: e.target.checked }))}
              />
              Use Testnet
            </label>
            <div style={{ display: "flex", gap: 8 }}>
              <button type="button" onClick={saveConfig} style={{ ...INPUT, width: "auto", padding: "7px 18px", background: "var(--accent)", color: "#fff", border: "none", cursor: "pointer", fontWeight: 700, fontSize: 12, borderRadius: "var(--radius-sm)" }}>
                Save
              </button>
              <button type="button" onClick={() => setShowConfig(false)} style={{ ...INPUT, width: "auto", padding: "7px 18px", cursor: "pointer", fontSize: 12, borderRadius: "var(--radius-sm)" }}>
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Product probe */}
      <div style={{ ...CARD }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
          <span style={{ fontSize: 12, fontWeight: 700, color: "var(--text-secondary)", fontFamily: "var(--font-display)" }}>
            BTC/USDT Spot Product
          </span>
          <button
            type="button"
            onClick={probeProduct}
            disabled={probing}
            style={{
              padding: "4px 12px", fontSize: 11, fontWeight: 600,
              background: "var(--surface-3)", border: "1px solid var(--border-subtle)",
              borderRadius: "var(--radius-chip)", color: "var(--text-secondary)",
              cursor: probing ? "wait" : "pointer", fontFamily: "var(--font-display)",
            }}
          >
            {probing ? "Probing…" : "Detect Product"}
          </button>
        </div>
        {spotProduct && (
          <div style={{ fontSize: 12, fontFamily: "var(--font-mono)", color: "var(--green)" }}>
            {spotProduct.symbol} · ID {spotProduct.productId} · Contract size {spotProduct.contractSize}
          </div>
        )}
        {probeError && (
          <div style={{ fontSize: 11, color: "var(--red)", marginTop: 4 }}>{probeError}</div>
        )}
        {!spotProduct && !probeError && (
          <div style={{ fontSize: 11, color: "var(--text-muted)" }}>Click &quot;Detect Product&quot; to resolve the BTC/USDT spot product ID.</div>
        )}
      </div>

      {/* Order form */}
      <div style={{ ...CARD }}>
        {/* Buy / Sell */}
        <div style={{ display: "flex", gap: 8, marginBottom: 14 }}>
          <SideTab label="BUY" active={side === "buy"} isBuy={true} onClick={() => setSide("buy")} />
          <SideTab label="SELL" active={side === "sell"} isBuy={false} onClick={() => setSide("sell")} />
        </div>

        {/* Market / Limit */}
        <div style={{ display: "flex", gap: 6, marginBottom: 14 }}>
          <Tab label="Market" active={orderType === "market_order"} onClick={() => setOrderType("market_order")} />
          <Tab label="Limit" active={orderType === "limit_order"} onClick={() => setOrderType("limit_order")} />
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {/* USDT amount */}
          <div>
            <span style={LABEL}>Order Size (USDT)</span>
            <div style={{ position: "relative" }}>
              <input
                style={{ ...INPUT, paddingRight: 48 }}
                type="number"
                min="1"
                step="10"
                placeholder="100"
                value={usdtAmount}
                onChange={(e) => setUsdtAmount(e.target.value)}
              />
              <span style={{
                position: "absolute", right: 10, top: "50%", transform: "translateY(-50%)",
                fontSize: 11, color: "var(--text-muted)", fontFamily: "var(--font-mono)",
                pointerEvents: "none",
              }}>USDT</span>
            </div>
            {estimatedBTC && (
              <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4, fontFamily: "var(--font-mono)" }}>
                ≈ {estimatedBTC.toFixed(6)} BTC at current price
              </div>
            )}
            {/* Quick amounts */}
            <div style={{ display: "flex", gap: 6, marginTop: 8, flexWrap: "wrap" }}>
              {[100, 250, 500, 1000, 2500, 5000].map((amt) => (
                <button
                  key={amt}
                  type="button"
                  onClick={() => setUsdtAmount(String(amt))}
                  style={{
                    padding: "3px 10px", fontSize: 11, fontWeight: 600,
                    background: usdtAmount === String(amt) ? "var(--accent-dim)" : "var(--surface-3)",
                    border: `1px solid ${usdtAmount === String(amt) ? "var(--accent)" : "var(--border-subtle)"}`,
                    borderRadius: "var(--radius-chip)",
                    color: usdtAmount === String(amt) ? "var(--accent)" : "var(--text-muted)",
                    cursor: "pointer", fontFamily: "var(--font-display)",
                  }}
                >
                  ${amt}
                </button>
              ))}
            </div>
          </div>

          {/* Limit price (only for limit orders) */}
          {orderType === "limit_order" && (
            <div>
              <span style={LABEL}>Limit Price (USDT)</span>
              <div style={{ position: "relative" }}>
                <input
                  style={{ ...INPUT, paddingRight: 48 }}
                  type="number"
                  min="1"
                  step="100"
                  placeholder={btcPrice ? String(Math.round(btcPrice)) : "Enter price"}
                  value={limitPrice}
                  onChange={(e) => setLimitPrice(e.target.value)}
                />
                <span style={{
                  position: "absolute", right: 10, top: "50%", transform: "translateY(-50%)",
                  fontSize: 11, color: "var(--text-muted)", fontFamily: "var(--font-mono)",
                  pointerEvents: "none",
                }}>USD</span>
              </div>
              {btcPrice && (
                <div style={{ display: "flex", gap: 6, marginTop: 8, flexWrap: "wrap" }}>
                  {[-2, -1, -0.5, 0.5, 1, 2].map((pct) => (
                    <button
                      key={pct}
                      type="button"
                      onClick={() => setLimitPrice(String(Math.round(btcPrice * (1 + pct / 100))))}
                      style={{
                        padding: "3px 9px", fontSize: 11, fontWeight: 600,
                        background: "var(--surface-3)", border: "1px solid var(--border-subtle)",
                        borderRadius: "var(--radius-chip)", color: pct < 0 ? "var(--red)" : "var(--green)",
                        cursor: "pointer", fontFamily: "var(--font-display)",
                      }}
                    >
                      {pct > 0 ? "+" : ""}{pct}%
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Place order button */}
          <button
            type="button"
            onClick={placeOrder}
            disabled={placing}
            style={{
              width: "100%",
              padding: "11px 0",
              fontSize: 14,
              fontWeight: 700,
              fontFamily: "var(--font-display)",
              letterSpacing: "0.04em",
              background: placing
                ? "var(--surface-3)"
                : side === "buy"
                ? "linear-gradient(135deg, rgba(30,142,62,0.9), rgba(20,100,45,0.9))"
                : "linear-gradient(135deg, rgba(217,48,37,0.9), rgba(160,30,20,0.9))",
              color: placing ? "var(--text-muted)" : "#fff",
              border: "none",
              borderRadius: "var(--radius-sm)",
              cursor: placing ? "wait" : "pointer",
              transition: "all 0.15s",
              boxShadow: placing ? "none" : "0 2px 8px rgba(0,0,0,0.25)",
            }}
          >
            {placing
              ? "Placing Order…"
              : `${side === "buy" ? "BUY" : "SELL"} BTC · ${orderType === "market_order" ? "Market" : "Limit"} · $${usdtAmount || "0"}`}
          </button>
        </div>
      </div>

      {/* Last order result */}
      {lastOrder && (
        <div style={{
          ...CARD,
          border: `1px solid ${lastOrder.ok ? "rgba(30,142,62,0.3)" : "rgba(217,48,37,0.3)"}`,
          background: lastOrder.ok ? "var(--green-dim)" : "var(--red-dim)",
        }}>
          <div style={{ fontSize: 13, fontWeight: 700, color: lastOrder.ok ? "var(--green)" : "var(--red)", fontFamily: "var(--font-display)", marginBottom: 8 }}>
            {lastOrder.ok ? "Order Placed" : "Order Failed"}
          </div>
          {lastOrder.ok ? (
            <div style={{ fontSize: 12, fontFamily: "var(--font-mono)", color: "var(--text-secondary)", lineHeight: 1.7 }}>
              <div>Order ID: <span style={{ color: "var(--text-primary)" }}>{lastOrder.orderId}</span></div>
              <div>Symbol: <span style={{ color: "var(--text-primary)" }}>{lastOrder.symbol}</span></div>
              <div>State: <span style={{ color: "var(--text-primary)" }}>{lastOrder.state}</span></div>
              {lastOrder.fillPrice ? <div>Fill Price: <span style={{ color: "var(--green)" }}>${fmt(lastOrder.fillPrice, 1)}</span></div> : null}
              <div>Contracts: <span style={{ color: "var(--text-primary)" }}>{lastOrder.contracts}</span></div>
            </div>
          ) : (
            <div style={{ fontSize: 12, color: "var(--red)", fontFamily: "var(--font-mono)" }}>
              {lastOrder.error}
              {lastOrder.debug && <div style={{ marginTop: 4, fontSize: 10, color: "var(--text-muted)", wordBreak: "break-all" }}>{lastOrder.debug}</div>}
            </div>
          )}
        </div>
      )}

      {/* Order history */}
      {orders.length > 0 && (
        <div style={{ ...CARD }}>
          <div style={{ fontSize: 12, fontWeight: 700, color: "var(--text-secondary)", fontFamily: "var(--font-display)", marginBottom: 10 }}>
            Order History ({orders.length})
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {orders.map((o, i) => (
              <div key={i} style={{
                display: "flex", alignItems: "center", justifyContent: "space-between",
                padding: "7px 10px",
                background: "var(--surface-3)",
                borderRadius: "var(--radius-sm)",
                fontSize: 11,
                fontFamily: "var(--font-mono)",
              }}>
                <span style={{ color: (o.side ?? "buy") === "buy" ? "var(--green)" : "var(--red)", fontWeight: 700 }}>
                  {(o.side ?? "buy").toUpperCase()}
                </span>
                <span style={{ color: "var(--text-muted)" }}>{o.symbol}</span>
                <span style={{ color: "var(--text-secondary)" }}>{o.contracts} cts</span>
                <span style={{ color: o.fillPrice ? "var(--text-primary)" : "var(--text-muted)" }}>
                  {o.fillPrice ? `$${fmt(o.fillPrice, 1)}` : o.state ?? "—"}
                </span>
                <span style={{
                  padding: "2px 7px",
                  borderRadius: "var(--radius-chip)",
                  background: "var(--green-dim)",
                  color: "var(--green)",
                  fontSize: 10,
                  fontWeight: 600,
                }}>{o.orderId ? `#${o.orderId}` : "ok"}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
