from __future__ import annotations
import sys
from pathlib import Path

# Add current directory to path so we can import internal modules
sys.path.append(str(Path(__file__).resolve().parent))

from infrastructure.ai.strategy_service.management import (
    StrategyPortfolioAllocator, 
    StrategyScoringEngine, 
    RegimeStrategyRouter, 
    NewsEventProtectionLayer,
    TradeJournal,
    MonteCarloRiskTester
)
from infrastructure.ai.strategy_service.schemas import (
    Regime, 
    EventWindow, 
    JournalEntry, 
    StrategyHealthInfo,
    Side,
    Candle
)
from infrastructure.ai.strategy_service.framework import ApexScalpFramework, ApexScalpConfig, StrategyCycleRequest
from datetime import datetime, timezone, timedelta

def test_allocator():
    print("Testing StrategyPortfolioAllocator...")
    scoring = StrategyScoringEngine()
    # High win rate strategy
    scoring.health["WINNER"] = StrategyHealthInfo(strategy="WINNER", wins=8, total_trades=10, win_rate=0.8, enabled=True)
    # Low win rate strategy
    scoring.health["LOSER"] = StrategyHealthInfo(strategy="LOSER", wins=2, total_trades=10, win_rate=0.2, enabled=True)
    
    allocator = StrategyPortfolioAllocator(scoring)
    
    w_winner = allocator.get_weight("WINNER", Regime.TRENDING_BULL)
    w_loser = allocator.get_weight("LOSER", Regime.TRENDING_BULL)
    
    print(f"Winner Weight: {w_winner}, Loser Weight: {w_loser}")
    assert w_winner > w_loser, "Winner should have more weight than Loser"
    print("Allocator OK.")

def test_router():
    print("Testing RegimeStrategyRouter...")
    router = RegimeStrategyRouter()
    
    assert router.is_allowed(Regime.TRENDING_BULL, "BREAKOUT_VWAP"), "Breakout should be allowed in Trend"
    assert not router.is_allowed(Regime.RANGE, "BREAKOUT_VWAP"), "Breakout should not be allowed in Range"
    assert router.is_allowed(Regime.RANGE, "LIQUIDITY_SWEEP"), "Liquidity Sweep should be allowed in Range"
    print("Router OK.")

def test_event_layer():
    print("Testing NewsEventProtectionLayer...")
    layer = NewsEventProtectionLayer()
    now = datetime.now(timezone.utc)
    
    layer.add_event(EventWindow(
        name="FOMC",
        start_ts=now - timedelta(minutes=5),
        end_ts=now + timedelta(minutes=5),
        risk_mode="BLOCK"
    ))
    
    blocked, reason = layer.is_blocked(now)
    assert blocked, f"Should be blocked during FOMC. Reason: {reason}"
    
    blocked, reason = layer.is_blocked(now + timedelta(minutes=10))
    assert not blocked, "Should not be blocked after FOMC"
    print("EventLayer OK.")

def test_journal_and_mc():
    print("Testing Journal and Monte Carlo...")
    journal = TradeJournal()
    now = datetime.now(timezone.utc)
    
    for i in range(10):
        journal.record(JournalEntry(
            ts=now, strategy="TEST", side=Side.LONG, regime="RANGE",
            entry_reason="Test", exit_reason="Test", pnl=100 if i % 2 == 0 else -50,
            winner=i % 2 == 0
        ))
    
    summary = journal.summarize()
    assert summary["total"] == 10
    assert summary["win_rate"] == 0.5
    
    pnls = [e.pnl for e in journal.entries]
    tester = MonteCarloRiskTester()
    report = tester.run(pnls, 10000)
    
    print(f"MC Avg Return: {report.avg_return_pct}%")
    assert report.simulations == 500
    print("Journal and MC OK.")

def test_framework_integration():
    print("Testing Framework Institutional Integration...")
    config = ApexScalpConfig()
    framework = ApexScalpFramework(config)
    framework.event_layer = NewsEventProtectionLayer()
    framework.journal = TradeJournal()
    
    # Mock request
    now = datetime.now(timezone.utc)
    candles = [Candle(timestamp=now, open=50000, high=50100, low=49900, close=50050, volume=10)]
    
    request = StrategyCycleRequest(
        symbol="BTCUSDT",
        candles_1m=candles,
        balance=10000,
        trade_history=[]
    )
    
    # Test Event Block
    framework.event_layer.add_event(EventWindow(
        name="CRASH", start_ts=now - timedelta(minutes=1), end_ts=now + timedelta(minutes=1), risk_mode="BLOCK"
    ))
    
    response = framework.run_cycle(request)
    assert not response.risk_gate_passed, "Should be blocked by event layer"
    assert "Blocked by news event" in response.blocked_reason
    print("Framework Integration OK.")

if __name__ == "__main__":
    try:
        test_allocator()
        test_router()
        test_event_layer()
        test_journal_and_mc()
        test_framework_integration()
        print("\nALL INSTITUTIONAL UPGRADE VERIFICATIONS PASSED!")
    except Exception as e:
        print(f"\nVERIFICATION FAILED: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
