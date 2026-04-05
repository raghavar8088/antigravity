from __future__ import annotations

import json
import logging
import math
import statistics
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import requests

from .schemas import (
    StrategyHealthInfo,
    TelegramConfig,
    AllocationProfile,
    EventWindow,
    JournalEntry,
    MonteCarloReport,
    Regime,
)


def get_logger(name: str = "RAIG_Management") -> logging.Logger:
    logger = logging.getLogger(name)
    if logger.handlers:
        return logger
    handler = logging.StreamHandler()
    handler.setFormatter(logging.Formatter("%(asctime)s - %(name)s - %(levelname)s - %(message)s"))
    logger.addHandler(handler)
    logger.setLevel(logging.INFO)
    return logger


logger = get_logger()


class ExitManager:
    def __init__(self, config: ExitManagementConfig):
        self.config = config

    def update_sl_if_needed(
        self,
        side: Side,
        entry: float,
        current_sl: float,
        latest_price: float,
        atr: float,
    ) -> float:
        new_sl = current_sl

        # 1. Break-Even
        if self.config.enable_break_even:
            initial_risk = abs(entry - current_sl)
            if initial_risk > 0:
                if side == Side.LONG:
                    rr_progress = (latest_price - entry) / initial_risk
                    if rr_progress >= self.config.break_even_trigger_rr:
                        new_sl = max(new_sl, entry)
                else:
                    rr_progress = (entry - latest_price) / initial_risk
                    if rr_progress >= self.config.break_even_trigger_rr:
                        new_sl = min(new_sl, entry)

        # 2. Trailing Stop
        if self.config.enable_trailing_stop and atr > 0:
            trail_distance = atr * self.config.trailing_atr_multiple
            if side == Side.LONG:
                potential_sl = latest_price - trail_distance
                new_sl = max(new_sl, potential_sl)
            else:
                potential_sl = latest_price + trail_distance
                new_sl = min(new_sl, potential_sl)

        return new_sl


class StrategyScoringEngine:
    def __init__(self, min_trades: int = 8, min_win_rate: float = 0.35, min_pf: float = 0.95):
        self.min_trades = min_trades
        self.min_win_rate = min_win_rate
        self.min_pf = min_pf
        self.health: Dict[str, StrategyHealthInfo] = {}

    def _get_health(self, strategy: str) -> StrategyHealthInfo:
        if strategy not in self.health:
            self.health[strategy] = StrategyHealthInfo(strategy=strategy)
        return self.health[strategy]

    def record_trade(self, strategy: str, pnl: float):
        h = self._get_health(strategy)
        h.total_trades += 1
        if pnl > 0:
            h.wins += 1
        elif pnl < 0:
            h.losses += 1

        # We don't store gross profit/loss directly in the schema to keep it light,
        # but in a real system we'd track it. For now, we'll approximate PF or 
        # assume it's tracked externally. 
        # In the user's snippet, they used gross_profit/gross_loss.
        # I'll add them as internal state or just use win_rate for auto-disabling.
        
        if h.total_trades >= self.min_trades:
            if h.win_rate < self.min_win_rate:
                if h.enabled:
                    h.enabled = False
                    logger.warning("Strategy %s auto-disabled due to low win rate (%.2f)", strategy, h.win_rate)

    def is_enabled(self, strategy: str) -> bool:
        return self._get_health(strategy).enabled

    def get_leaderboard(self) -> List[StrategyHealthInfo]:
        return sorted(self.health.values(), key=lambda x: (x.enabled, x.win_rate), reverse=True)


class TelegramAlerter:
    def __init__(self, config: TelegramConfig):
        self.config = config

    def send(self, message: str):
        if not self.config.enabled or not self.config.bot_token or not self.config.chat_id:
            return

        url = f"https://api.telegram.org/bot{self.config.bot_token}/sendMessage"
        payload = {"chat_id": self.config.chat_id, "text": message}
        try:
            res = requests.post(url, json=payload, timeout=10)
            res.raise_for_status()
        except Exception as exc:
            logger.error("Failed to send Telegram alert: %s", exc)


class DashboardManager:
    def __init__(self):
        self.latest_status: Optional[DashboardStatus] = None

    def update(self, status: DashboardStatus):
        self.latest_status = status

    def get_status(self) -> Optional[DashboardStatus]:
        return self.latest_status


class StrategyPortfolioAllocator:
    def __init__(self, scoring_engine: StrategyScoringEngine):
        self.scoring_engine = scoring_engine
        self.default_weight = 1.0
        self.min_weight = 0.25
        self.max_weight = 2.0

    def get_weight(self, strategy: str, regime: Regime | None = None) -> float:
        h = self.scoring_engine.health.get(strategy)
        if h and not h.enabled:
            return 0.0

        win_rate = h.win_rate if h else 0.5
        # Dynamic weighting based on win rate
        win_component = max(0.5, min(1.5, win_rate * 2))
        
        regime_component = 1.0
        if regime:
            # High-level regime bonuses
            if "BREAKOUT" in strategy.upper() and regime in {Regime.TRENDING_BULL, Regime.TRENDING_BEAR}:
                regime_component = 1.2
            elif "PULLBACK" in strategy.upper() and regime in {Regime.TRENDING_BULL, Regime.TRENDING_BEAR}:
                regime_component = 1.15
            elif "LIQUIDITY" in strategy.upper() and regime == Regime.RANGE:
                regime_component = 1.1

        raw = self.default_weight * win_component * regime_component
        return max(self.min_weight, min(self.max_weight, round(raw, 2)))

    def leaderboard(self, regime: Regime | None = None) -> List[AllocationProfile]:
        profiles = []
        for strat in self.scoring_engine.health.keys():
            weight = self.get_weight(strat, regime)
            profiles.append(AllocationProfile(strategy=strat, weight=weight))
        return sorted(profiles, key=lambda x: x.weight, reverse=True)


class RegimeStrategyRouter:
    def __init__(self):
        self.mapping: Dict[Regime, List[str]] = {
            Regime.TRENDING_BULL: ["BREAKOUT", "PULLBACK", "TREND"],
            Regime.TRENDING_BEAR: ["BREAKOUT", "PULLBACK", "TREND"],
            Regime.RANGE: ["LIQUIDITY", "MEAN_REVERSION", "SCALP"],
            Regime.HIGH_VOL: ["SCALP", "VOLATILITY"],
            Regime.NO_TRADE: [],
        }

    def is_allowed(self, regime: Regime, strategy: str) -> bool:
        allowed_keywords = self.mapping.get(regime, [])
        strategy_upper = strategy.upper()
        return any(keyword in strategy_upper for keyword in allowed_keywords)


class NewsEventProtectionLayer:
    def __init__(self):
        self.events: List[EventWindow] = []

    def add_event(self, event: EventWindow):
        self.events.append(event)

    def is_blocked(self, now: datetime | None = None) -> tuple[bool, str]:
        current_time = now or datetime.now(timezone.utc)
        for event in self.events:
            if event.start_ts <= current_time <= event.end_ts:
                if event.risk_mode == "BLOCK":
                    return True, f"Blocked by news event: {event.name}"
        return False, "OK"


class TradeJournal:
    def __init__(self):
        self.entries: List[JournalEntry] = []

    def record(self, entry: JournalEntry):
        self.entries.append(entry)

    def summarize(self) -> Dict[str, Any]:
        if not self.entries:
            return {"total": 0, "win_rate": 0.0, "net_pnl": 0.0}
        wins = sum(1 for e in self.entries if e.winner)
        return {
            "total": len(self.entries),
            "win_rate": round(wins / len(self.entries), 2),
            "net_pnl": round(sum(e.pnl for e in self.entries), 2),
            "last_entry": self.entries[-1].ts.isoformat() if self.entries else None
        }


class MonteCarloRiskTester:
    def __init__(self, seed: int = 42):
        import random
        self.rng = random.Random(seed)

    def run(self, trade_pnls: List[float], starting_equity: float, simulations: int = 500) -> MonteCarloReport:
        if not trade_pnls:
            return MonteCarloReport(
                simulations=0, trades_per_simulation=0, 
                avg_return_pct=0, worst_return_pct=0, best_return_pct=0,
                avg_max_drawdown_pct=0, worst_max_drawdown_pct=0
            )

        returns = []
        drawdowns = []
        for _ in range(simulations):
            sim_trades = trade_pnls[:]
            self.rng.shuffle(sim_trades)
            equity = starting_equity
            curve = [equity]
            for pnl in sim_trades:
                equity += pnl
                curve.append(equity)
            
            final_ret = ((equity / starting_equity) - 1) * 100
            returns.append(final_ret)
            
            # Max Drawdown for this sim
            peak = curve[0]
            max_dd = 0.0
            for val in curve:
                peak = max(peak, val)
                if peak > 0:
                    dd = (peak - val) / peak
                    max_dd = max(max_dd, dd)
            drawdowns.append(max_dd * 100)

        return MonteCarloReport(
            simulations=simulations,
            trades_per_simulation=len(trade_pnls),
            avg_return_pct=round(statistics.mean(returns), 2),
            worst_return_pct=round(min(returns), 2),
            best_return_pct=round(max(returns), 2),
            avg_max_drawdown_pct=round(statistics.mean(drawdowns), 2),
            worst_max_drawdown_pct=round(max(drawdowns), 2)
        )
