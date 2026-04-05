from __future__ import annotations

import logging
import uuid
from datetime import datetime, timezone
from typing import Any

from .framework import ApexScalpFramework
from .schemas import Side, StrategyCycleRequest, StrategySignal

logger = logging.getLogger("ApexScalp.Execution")


class ExchangeClient:
    def get_latest_price(self, symbol: str) -> float:
        raise NotImplementedError

    def get_ohlcv(self, symbol: str, timeframe: str, limit: int = 200) -> list[dict[str, float]]:
        raise NotImplementedError

    def place_market_order(self, symbol: str, side: str, qty: float) -> dict[str, Any]:
        raise NotImplementedError

    def get_position(self, symbol: str) -> dict[str, Any]:
        raise NotImplementedError


class CCXTExchangeClient(ExchangeClient):
    """
    Exchange client using the ccxt library.
    Requires: pip install ccxt
    """

    def __init__(self, exchange_id: str, api_key: str, api_secret: str, market_type: str = "swap"):
        try:
            import ccxt
        except ImportError as exc:
            raise ImportError("ccxt is required for CCXTExchangeClient. Install it with: pip install ccxt") from exc

        exchange_class = getattr(ccxt, exchange_id)
        self.exchange = exchange_class(
            {
                "apiKey": api_key,
                "secret": api_secret,
                "enableRateLimit": True,
                "options": {"defaultType": market_type},
            }
        )

    def get_latest_price(self, symbol: str) -> float:
        ticker = self.exchange.fetch_ticker(symbol)
        return float(ticker["last"])

    def get_ohlcv(self, symbol: str, timeframe: str, limit: int = 200) -> list[dict[str, float]]:
        rows = self.exchange.fetch_ohlcv(symbol, timeframe=timeframe, limit=limit)
        output: list[dict[str, float]] = []
        for row in rows:
            output.append(
                {
                    "timestamp": float(row[0]),
                    "open": float(row[1]),
                    "high": float(row[2]),
                    "low": float(row[3]),
                    "close": float(row[4]),
                    "volume": float(row[5]),
                }
            )
        return output

    def place_market_order(self, symbol: str, side: str, qty: float) -> dict[str, Any]:
        order_side = "buy" if side == "LONG" else "sell"
        return self.exchange.create_market_order(symbol, order_side, qty)

    def get_position(self, symbol: str) -> dict[str, Any]:
        try:
            positions = self.exchange.fetch_positions([symbol])
            return positions[0] if positions else {}
        except Exception as e:
            logger.error(f"Failed to fetch position: {e}")
            return {}


class LiveTradingBot:
    def __init__(self, framework: ApexScalpFramework, exchange: ExchangeClient, symbol: str):
        self.framework = framework
        self.exchange = exchange
        self.symbol = symbol

    def run_once(self, request_base: StrategyCycleRequest) -> dict[str, Any]:
        """
        Polls exchange data, merges it with the request, and runs a strategy cycle.
        """
        # In a real scenario, we would fetch OHLCV and populate request_base here.
        # This is a stub showing how to link them.
        response = self.framework.run_cycle(request_base)

        if response.risk_gate_passed and response.execution_plan:
            plan = response.execution_plan
            logger.info(f"Executing {plan.order_type} {plan.side} on {plan.symbol} for {plan.quantity} units.")
            
            # Real execution
            try:
                order_result = self.exchange.place_market_order(
                    symbol=self.symbol,
                    side=plan.side,
                    qty=plan.quantity,
                )
                return {"order": order_result, "cycle": response.model_dump()}
            except Exception as e:
                logger.error(f"Execution failed: {e}")
                return {"error": str(e), "cycle": response.model_dump()}

        return {"cycle": response.model_dump(), "note": response.blocked_reason or "No trade executed."}
