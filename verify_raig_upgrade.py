from __future__ import annotations
import sys
from pathlib import Path

# Add current directory to path so we can import internal modules
sys.path.append(str(Path(__file__).resolve().parent))

from infrastructure.ai.strategy_service.management import ExitManager, StrategyScoringEngine
from infrastructure.ai.strategy_service.schemas import ExitManagementConfig, Side, Candle, StrategyCycleRequest, OpenPosition, ClosedTrade
from infrastructure.ai.strategy_service.framework import ApexScalpFramework, ApexScalpConfig

def test_exit_manager():
    print("Testing ExitManager...")
    config = ExitManagementConfig(enable_trailing_stop=True, trailing_atr_multiple=1.0)
    manager = ExitManager(config)
    
    # LONG: Price went up, SL should follow
    new_sl = manager.update_sl_if_needed(Side.LONG, entry=1000, current_sl=950, latest_price=1100, atr=50)
    assert new_sl == 1050, f"Expected 1050, got {new_sl}"
    
    # SHORT: Price went down, SL should follow
    new_sl = manager.update_sl_if_needed(Side.SHORT, entry=1000, current_sl=1050, latest_price=900, atr=50)
    assert new_sl == 950, f"Expected 950, got {new_sl}"
    print("ExitManager OK.")

def test_scoring_engine():
    print("Testing StrategyScoringEngine...")
    engine = StrategyScoringEngine(min_trades=3, min_win_rate=0.5)
    
    engine.record_trade("TEST_STRAT", 100) # Win 1
    engine.record_trade("TEST_STRAT", -50) # Loss 1
    engine.record_trade("TEST_STRAT", -20) # Loss 2
    
    # After 3 trades, win rate is 0.33, should be disabled
    assert not engine.is_enabled("TEST_STRAT"), "Strategy should be disabled"
    print("StrategyScoringEngine OK.")

def test_framework_cycle():
    print("Testing Framework Cycle Integration...")
    config = ApexScalpConfig()
    framework = ApexScalpFramework(config)
    
    # Mock request
    now = __import__("datetime").datetime.now(__import__("datetime").timezone.utc)
    candles = [Candle(timestamp=now, open=50000, high=50100, low=49900, close=50050, volume=10)]
    
    request = StrategyCycleRequest(
        symbol="BTCUSDT",
        candles_1m=candles,
        balance=10000,
        open_positions=[
            OpenPosition(id="1", strategy="STRAT1", side=Side.LONG, entry_price=50000, stop_loss=49500, take_profit=51000, qty=0.1, opened_at=now)
        ],
        trade_history=[
            ClosedTrade(strategy="STRAT1", side=Side.LONG, realized_pnl=100)
        ]
    )
    
    response = framework.run_cycle(request)
    
    assert response.dashboard is not None, "Dashboard should not be None"
    assert len(response.dashboard.leaderboard) > 0, "Leaderboard should have items"
    assert framework.last_status is not None, "Framework should store last status"
    print("Framework OK.")

if __name__ == "__main__":
    try:
        test_exit_manager()
        test_scoring_engine()
        test_framework_cycle()
        print("\nALL UPGRADE VERIFICATIONS PASSED!")
    except Exception as e:
        print(f"\nVERIFICATION FAILED: {e}")
        sys.exit(1)
