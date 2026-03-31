# Python Strategy Service

This service now contains both:

- a multi-strategy BTC scalping evaluator
- an `ApexScalp` framework layer with YAML config loading, risk gating, performance metrics, and execution planning

## Implemented strategies

- `mechanical_1m_btc_vwap_ema_rsi`
- `four_hour_range_reentry`
- `three_ema_williams_fractal`
- `smc_fvg_liquidity_grab`
- `ny_open_session_range_breakout`
- `sr_ema_crossover_sniper`
- `stochastic_momentum_squeeze`
- `bollinger_band_bounce`
- `keltner_channel_breakout`

## Framework modules

- `ApexScalpConfig`: YAML-backed runtime config loader
- `RiskManager`: 1% risk sizing, three-strikes block, daily loss cap, daily trade cap
- `PaperExecutionEngine`: maker/taker fee aware order-plan builder
- `PerformanceTracker`: win rate, Sharpe-style metric, fee/slippage drag
- `StrategyManager`: full market scan plus signal selection and managed run-cycle flow

## Run

From the repo root:

```bash
pip install -r requirements.txt
python main.py
```

The service will start on `http://localhost:8000`.

Default runtime settings live in `infrastructure/ai/config.yaml`.

## Endpoints

- `GET /health`
- `GET /strategies`
- `POST /strategies/evaluate`
- `GET /framework/config`
- `POST /framework/run-cycle`

## Example strategy-evaluation request

```json
{
  "symbol": "BTCUSDT",
  "candles_1m": [
    {
      "timestamp": "2026-03-31T09:30:00Z",
      "open": 84000,
      "high": 84020,
      "low": 83990,
      "close": 84010,
      "volume": 12.5
    }
  ],
  "candles_5m": [],
  "candles_30m": [],
  "candles_1h": [],
  "candles_4h": [],
  "support_resistance": {
    "support": 83850,
    "resistance": 84220
  },
  "config": {
    "mechanical_session_timezone": "Asia/Kolkata",
    "mechanical_session_start": "05:30",
    "ny_timezone": "America/New_York"
  }
}
```

## Example managed cycle request

```json
{
  "symbol": "BTCUSDT",
  "balance": 10000,
  "daily_pnl": -120,
  "daily_trades": 6,
  "consecutive_losses": 1,
  "candles_1m": [],
  "candles_5m": [],
  "candles_30m": [],
  "candles_1h": [],
  "candles_4h": [],
  "strategy_overrides": {
    "enable_stochastic_squeeze": true
  },
  "trade_history": [
    {
      "strategy": "mechanical_1m_btc_vwap_ema_rsi",
      "side": "LONG",
      "realized_pnl": 42.5,
      "holding_minutes": 7,
      "fees_paid": 3.2,
      "slippage_paid": 1.8
    }
  ]
}
```

## Response shape

Raw evaluation returns per-strategy signals with:

- `state`: `LONG`, `SHORT`, `WAIT`, or `NO_TRADE`
- `entry_price`
- `stop_loss`
- `take_profit`
- optional partial take-profit fields
- `reason`
- `metadata`

Managed cycle responses also include:

- `selected_signal`
- `risk_gate_passed`
- `blocked_reason`
- `execution_plan`
- `performance`
- `loaded_config`
