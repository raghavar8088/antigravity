package phase25

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"antigravity-engine/internal/strategy"
	"antigravity-engine/internal/validation/phase23b"
)

// LiveForwardEngine runs a paper-trading simulation over the live-forward window
// using real Binance Futures data and real Strategy.OnCandle() execution.
// All signals flow through the full pipeline: feed → strategy → risk gate →
// allocation → sizing → fill simulation → position → exit → trade record.
type LiveForwardEngine struct {
	cfg     Phase25Config
	loader  *phase23b.BinanceFuturesLoader
}

func NewLiveForwardEngine(cfg Phase25Config) *LiveForwardEngine {
	return &LiveForwardEngine{
		cfg:    cfg,
		loader: phase23b.NewBinanceFuturesLoader(),
	}
}

// Run loads real data for the live-forward window and simulates paper trades.
func (e *LiveForwardEngine) Run(eligible []EligibilityRecord) ([]LiveTrade, int, error) {
	to := time.Now().UTC().Truncate(time.Minute)
	from := to.AddDate(0, 0, -e.cfg.LiveDays)

	candles, err := e.loader.LoadCandles(e.cfg.Symbol, from, to)
	if err != nil {
		return nil, 0, fmt.Errorf("live-forward data load: %w", err)
	}
	if len(candles) == 0 {
		return nil, 0, fmt.Errorf("no live-forward candles returned for %s [%s, %s]",
			e.cfg.Symbol, from.Format("2006-01-02"), to.Format("2006-01-02"))
	}

	// Build approved strategy inventory
	approvedNames := make(map[string]bool, len(eligible))
	for _, rec := range eligible {
		if rec.ApprovedForLive {
			approvedNames[rec.StrategyName] = true
		}
	}

	entries := buildFilteredInventory(approvedNames, e.cfg.MaxStrategies)

	replayCfg := phase23b.ReplayConfig{
		Symbol:         e.cfg.Symbol,
		From:           from,
		To:             to,
		InitialCapital: e.cfg.InitialCapital,
		PositionCapPct: phase23b.DefaultPositionCapPct,
		TakerFeeBps:    e.cfg.TakerFeeBps,
		MakerFeeBps:    phase23b.MakerFeeBps,
		SlippageBps:    e.cfg.SlippageBps,
		MaxHoldMins:    phase23b.DefaultMaxHoldMins,
		MinConfidence:  phase23b.DefaultMinConfidence,
	}

	engine := phase23b.NewRealReplayEngine(replayCfg)
	replays := engine.RunAll(entries, candles)

	trades := convertToLiveTrades(replays)
	return trades, len(candles), nil
}

// ComputeAllLiveMetrics computes LiveMetrics for each strategy from the live trade slice.
func ComputeAllLiveMetrics(trades []LiveTrade, tradingDays float64) map[string]LiveMetrics {
	byStrat := make(map[string][]LiveTrade)
	for _, t := range trades {
		byStrat[t.StrategyName] = append(byStrat[t.StrategyName], t)
	}

	result := make(map[string]LiveMetrics, len(byStrat))
	for name, stratTrades := range byStrat {
		result[name] = computeLiveMetrics(name, stratTrades, tradingDays)
	}
	return result
}

func computeLiveMetrics(name string, trades []LiveTrade, tradingDays float64) LiveMetrics {
	m := LiveMetrics{StrategyName: name}
	if len(trades) == 0 {
		return m
	}

	m.TotalTrades = len(trades)
	m.AlphaFamily = trades[0].AlphaFamily

	wins, losses := 0, 0
	sumWin, sumLoss := 0.0, 0.0
	pnls := make([]float64, 0, len(trades))

	maxConWin, maxConLoss := 0, 0
	curWin, curLoss := 0, 0

	var totalHoldMins float64
	for _, t := range trades {
		m.NetPnLUSD += t.NetPnLUSD
		m.TotalFeeUSD += t.FeeUSD
		m.TotalSlipUSD += t.SlippageUSD
		m.TotalFundingUSD += t.FundingUSD
		m.TotalLatencyMs += t.LatencySignalSubmitMs + t.LatencySubmitAckMs + t.LatencyAckFillMs

		pnls = append(pnls, t.NetPnLUSD)
		holdMins := t.ExitTime.Sub(t.EntryTime).Minutes()
		totalHoldMins += holdMins

		if t.IsWin {
			wins++
			sumWin += t.NetPnLUSD
			curWin++
			curLoss = 0
			if curWin > maxConWin {
				maxConWin = curWin
			}
		} else {
			losses++
			sumLoss += -t.NetPnLUSD
			curLoss++
			curWin = 0
			if curLoss > maxConLoss {
				maxConLoss = curLoss
			}
		}
	}

	m.WinRate = float64(wins) / float64(len(trades))
	m.MaxConWins = maxConWin
	m.MaxConLosses = maxConLoss
	m.AvgHoldMins = totalHoldMins / float64(len(trades))
	m.GrossProfit = sumWin
	m.GrossLoss = sumLoss

	if sumLoss > 0 {
		m.ProfitFactor = sumWin / sumLoss
	}
	if wins > 0 {
		m.AvgWin = sumWin / float64(wins)
	}
	if losses > 0 {
		m.AvgLoss = sumLoss / float64(losses)
	}
	m.Expectancy = m.NetPnLUSD / float64(len(trades))

	// Risk metrics
	m.Sharpe = sharpe25(pnls)
	m.Sortino = sortino25(pnls)
	m.MaxDrawdown = maxDD25(pnls)
	m.RiskOfRuin = riskOfRuin25(m.WinRate, m.AvgWin, m.AvgLoss)

	// CAGR
	if tradingDays > 0 && m.NetPnLUSD != 0 {
		years := tradingDays / 365.0
		if years > 0 {
			m.CAGR = (math.Pow(1+m.NetPnLUSD/1_000_000, 1/years) - 1) * 100
		}
	}
	return m
}

// ── Conversion from Phase 23B replay to LiveTrade ────────────────────────────

func convertToLiveTrades(replays []phase23b.StrategyReplayResult) []LiveTrade {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var out []LiveTrade

	for _, r := range replays {
		family := inferAlphaFamily(r.AlphaSource)
		for _, ct := range r.Trades {
			// Simulate realistic latency (microsecond precision, μs → ms)
			signalSubmitMs := 0.5 + rng.Float64()*2.0    // 0.5–2.5ms
			submitAckMs := 1.0 + rng.Float64()*4.0        // 1–5ms
			ackFillMs := 0.2 + rng.Float64()*0.8          // 0.2–1ms (exchange side)
			fillPosMs := 0.1 + rng.Float64()*0.3          // 0.1–0.4ms (internal)
			posExitMs := 0.3 + rng.Float64()*1.5          // 0.3–1.8ms

			lt := LiveTrade{
				TradeID:      fmt.Sprintf("%s-%d", r.StrategyName, ct.EntryTime.UnixMilli()),
				StrategyName: ct.StrategyName,
				AlphaFamily:  family,
				EntryTime:    ct.EntryTime,
				ExitTime:     ct.ExitTime,
				EntryPrice:   ct.EntryPrice,
				ExitPrice:    ct.ExitPrice,
				SignalPrice:  ct.EntryPrice * (1 - e(rng)*0.0001),
				SubmitPrice:  ct.EntryPrice * (1 - e(rng)*0.0002),
				FillPrice:    ct.ExitPrice, // best approximation from replay
				Size:         ct.SizeUSD,
				Direction:    ct.Direction,
				GrossPnLUSD:  ct.GrossPnLUSD,
				FeeUSD:       ct.EntryFeeUSD + ct.ExitFeeUSD,
				SlippageUSD:  ct.SlippageUSD,
				FundingUSD:   ct.FundingUSD,
				NetPnLUSD:    ct.NetPnLUSD,
				LatencySignalSubmitMs: signalSubmitMs,
				LatencySubmitAckMs:    submitAckMs,
				LatencyAckFillMs:      ackFillMs,
				LatencyFillPositionMs: fillPosMs,
				LatencyPositionExitMs: posExitMs,
				IsWin:      ct.NetPnLUSD > 0,
				ExitReason: string(ct.ExitReason),
			}
			out = append(out, lt)
		}
	}
	return out
}

func e(rng *rand.Rand) float64 { return rng.Float64() }

func inferAlphaFamily(alphaSource string) AlphaFamily {
	src := strings.ToLower(alphaSource)
	switch {
	case strings.Contains(src, "fund"):
		return FamilyFunding
	case strings.Contains(src, "liquidat"):
		return FamilyLiquidation
	case strings.Contains(src, "sweep") || strings.Contains(src, "liquidity"):
		return FamilyLiquiditySweep
	case strings.Contains(src, "fvg") || strings.Contains(src, "fair value"):
		return FamilyFVG
	case strings.Contains(src, "order block") || strings.Contains(src, "ob"):
		return FamilyOrderBlock
	case strings.Contains(src, "mss"):
		return FamilyMSS
	case strings.Contains(src, "market struct"):
		return FamilyMarketStructure
	case strings.Contains(src, "mean") || strings.Contains(src, "reversion"):
		return FamilyMeanReversion
	case strings.Contains(src, "momentum"):
		return FamilyMomentum
	case strings.Contains(src, "trend") || strings.Contains(src, "ema"):
		return FamilyTrend
	case strings.Contains(src, "session"):
		return FamilySessionExpansion
	default:
		return FamilyMomentum
	}
}

// buildFilteredInventory returns production strategies filtered to the approved set.
func buildFilteredInventory(approved map[string]bool, maxStrats int) []strategy.RegistryEntry {
	curated := strategy.BuildCuratedScalpers()
	all := strategy.BuildAllScalpers()

	seen := make(map[string]bool)
	var merged []strategy.RegistryEntry
	for _, e := range curated {
		if !seen[e.Strategy.Name()] {
			seen[e.Strategy.Name()] = true
			merged = append(merged, e)
		}
	}
	for _, e := range all {
		if !seen[e.Strategy.Name()] {
			seen[e.Strategy.Name()] = true
			merged = append(merged, e)
		}
	}

	filtered := make([]strategy.RegistryEntry, 0, len(approved))
	for _, e := range merged {
		if len(approved) == 0 || approved[e.Strategy.Name()] {
			filtered = append(filtered, e)
		}
	}

	if maxStrats > 0 && len(filtered) > maxStrats {
		filtered = filtered[:maxStrats]
	}
	return filtered
}

// ── Math helpers ──────────────────────────────────────────────────────────────

func sharpe25(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}
	mean := mean25(pnls)
	std := std25(pnls)
	if std == 0 {
		return 0
	}
	return mean / std * math.Sqrt(252)
}

func sortino25(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}
	mean := mean25(pnls)
	downVar := 0.0
	n := 0
	for _, p := range pnls {
		if p < 0 {
			downVar += p * p
			n++
		}
	}
	if n == 0 {
		return 4.0 // no losing trades → high ratio
	}
	downStd := math.Sqrt(downVar / float64(n))
	if downStd == 0 {
		return 0
	}
	return mean / downStd * math.Sqrt(252)
}

func maxDD25(pnls []float64) float64 {
	peak, dd := 0.0, 0.0
	equity := 0.0
	for _, p := range pnls {
		equity += p
		if equity > peak {
			peak = equity
		}
		drawdown := 0.0
		if peak > 0 {
			drawdown = (peak - equity) / peak * 100
		}
		if drawdown > dd {
			dd = drawdown
		}
	}
	return dd
}

func riskOfRuin25(winRate, avgWin, avgLoss float64) float64 {
	if avgWin <= 0 || avgLoss <= 0 || winRate <= 0 || winRate >= 1 {
		return 1.0
	}
	r := (avgLoss * (1 - winRate)) / (avgWin * winRate)
	if r >= 1 {
		return 1.0
	}
	return math.Pow(r, 100)
}

func mean25(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func std25(xs []float64) float64 {
	m := mean25(xs)
	v := 0.0
	for _, x := range xs {
		d := x - m
		v += d * d
	}
	return math.Sqrt(v / float64(len(xs)))
}

// percentile25 returns the p-th percentile (0–100) of a float64 slice.
func percentile25(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := make([]float64, len(xs))
	copy(sorted, xs)
	sort.Float64s(sorted)
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
