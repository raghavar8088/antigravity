from __future__ import annotations

import logging
import math
from datetime import datetime, timezone
from typing import Any

from .framework import ApexScalpFramework
from .schemas import (
    BacktestReport,
    BacktestTrade,
    ClosedTrade,
    StrategyCycleRequest,
)

logger = logging.getLogger("ApexScalp.Backtest")


class Backtester:
    """
    Backtesting engine for the ApexScalp framework.
    Simulates a sequence of strategy cycles and tracks portfolio performance.
    """

    def __init__(self, framework: ApexScalpFramework, starting_equity: float = 10000.0):
        self.framework = framework
        self.starting_equity = starting_equity
        self.equity = starting_equity
        self.equity_curve: list[float] = [starting_equity]
        self.trades: list[BacktestTrade] = []
        self.history: list[ClosedTrade] = []

    def run(self, snapshots: list[StrategyCycleRequest]) -> BacktestReport:
        """
        Runs the backtest over a series of requests representing points in time.
        Note: This is a simplified simulation where trades are assumed to execute 
        and we track performance metrics based on cycle response.
        """
        for i, request in enumerate(snapshots):
            # Update request with current account state
            request.balance = self.equity
            request.trade_history = self.history
            request.daily_pnl = self.equity - self.starting_equity # Simplified daily PnL
            
            # Run the framework cycle
            response = self.framework.run_cycle(request)
            
            # If a trade is executed, we simulate it
            # In a real backtester, we'd wait for a future snapshot to 'close' the trade.
            # Here, we simulate a mock outcome for the selected signal if it passed risk gates.
            if response.risk_gate_passed and response.execution_plan:
                # Mock a trade outcome (Simplified: 50/50 win/loss for testing if no actual exit logic provided)
                # In a production backtester, you would track the position across multiple snapshots.
                plan = response.execution_plan
                
                # For this simplified backtester, we'll just record the intent.
                # A full backtest would require tracking open positions.
                logger.info(f"Backtest step {i}: Trade signal {plan.strategy} {plan.side}")

            self.equity_curve.append(self.equity)

        return self._generate_report()

    def _generate_report(self) -> BacktestReport:
        ending_equity = self.equity
        total_return_pct = ((ending_equity / self.starting_equity) - 1) * 100
        
        wins = [t for t in self.trades if t.winner]
        losses = [t for t in self.trades if not t.winner]
        
        gross_profit = sum(t.pnl for t in wins)
        gross_loss = abs(sum(t.pnl for t in losses))
        win_rate = len(wins) / len(self.trades) if self.trades else 0.0
        profit_factor = gross_profit / gross_loss if gross_loss > 0 else math.inf
        max_dd = self._calculate_max_drawdown()

        return BacktestReport(
            starting_equity=self.starting_equity,
            ending_equity=ending_equity,
            total_return_pct=total_return_pct,
            total_trades=len(self.trades),
            win_rate=win_rate,
            profit_factor=profit_factor,
            max_drawdown_pct=max_dd,
            trades=self.trades,
        )

    def _calculate_max_drawdown(self) -> float:
        peak = self.equity_curve[0]
        max_dd = 0.0
        for val in self.equity_curve:
            peak = max(peak, val)
            dd = (peak - val) / peak if peak > 0 else 0.0
            max_dd = max(max_dd, dd)
        return max_dd * 100
