package main

import (
	"fmt"
	"time"

	"antigravity-engine/internal/backtest"
	btExecution "antigravity-engine/internal/backtest/execution"
	v3 "antigravity-engine/internal/backtest/v3"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// The existing harness ("engine" mode): backtest.Runner.Run, line for line —
// shared ContextBuilder, DatasetToTicks, ScalerV3Adapter, v3 engine with
// DefaultRunConfig — except that the trades are kept instead of only the
// aggregate StrategyResult.

type engineShared struct {
	cfg   backtest.RunConfig
	cb    *backtest.ContextBuilder
	ticks []marketdata.Tick
	fr    []btExecution.FundingRate
}

func newEngineShared(mtf marketdata.MTFDataset) *engineShared {
	fr := make([]btExecution.FundingRate, len(mtf.FundingRates))
	for i, f := range mtf.FundingRates {
		fr[i] = btExecution.FundingRate{Timestamp: f.Time, Rate: f.Rate}
	}
	return &engineShared{cfg: backtest.DefaultRunConfig("BTCUSDT"), cb: backtest.NewContextBuilder(mtf),
		ticks: backtest.DatasetToTicks(mtf), fr: fr}
}

func runEngine(e scalers.RegistryEntry, sh *engineShared) ([]tradeRec, runStats, error) {
	st := runStats{Name: e.Name}
	regime := sh.cfg.Regime
	if len(e.Regimes) > 0 {
		regime = e.Regimes[0]
	}
	adapter := backtest.NewScalerV3Adapter(e.Strategy, sh.cb, regime, sh.cfg.Symbol, sh.cfg.SizeBTC)
	res, err := v3.NewEngine(sh.cfg.V3Config, sh.fr, nil).Run(v3.V3Input{
		Config: sh.cfg.V3Config, Ticks: sh.ticks, Strategy: adapter})
	if err != nil {
		return nil, st, err
	}
	out := make([]tradeRec, 0, len(res.Trades))
	for _, t := range res.Trades {
		r := tradeRec{Strategy: e.Name, Long: t.Side == strategy.ActionBuy,
			Entry: t.OpenedAt.Unix(), Exit: t.ClosedAt.Unix(), EntryPx: t.EntryPrice, ExitPx: t.ExitPrice,
			Reason: t.ExitReason, SLPct: t.StopLossPct, TPPct: t.TakeProfitPct,
			NetUSD: t.NetPnL, Size: t.Size}
		if t.EntryPrice > 0 && t.Size > 0 {
			r.Gross = t.GrossPnL / (t.EntryPrice * t.Size)
			r.Funding = t.FundingCostUSD / (t.EntryPrice * t.Size)
		}
		out = append(out, r)
	}
	st.Entries = len(out)
	return out, st, nil
}

// ── probe: what does a strategy actually see at the moment it trades? ────────

type probeStrategy struct {
	tickMs *int64
	rows   []probeRow
}

type probeRow struct {
	tick, now    time.Time
	price        float64
	last15mClose time.Time
	last1hClose  time.Time
	last4hClose  time.Time
	last1mClose  time.Time
}

func (p *probeStrategy) Name() string { return "probe" }
func (p *probeStrategy) ValidRegimes() []scalers.Regime {
	return []scalers.Regime{scalers.RegimeTrending}
}
func lastClose(c []scalers.Candle, d time.Duration) time.Time {
	if len(c) == 0 {
		return time.Time{}
	}
	return c[len(c)-1].OpenTime.Add(d)
}
func (p *probeStrategy) Evaluate(ctx scalers.MarketContext) scalers.Signal {
	p.rows = append(p.rows, probeRow{tick: time.UnixMilli(*p.tickMs).UTC(), now: ctx.Now, price: ctx.Price,
		last15mClose: lastClose(ctx.Candles15m, 15*time.Minute), last1hClose: lastClose(ctx.Candles1h, time.Hour),
		last4hClose: lastClose(ctx.Candles4h, 4*time.Hour), last1mClose: lastClose(ctx.Candles1m, time.Minute)})
	return scalers.Signal{Direction: scalers.DirectionNone}
}

// tickSpy records the time of the tick the adapter is evaluating on.
type tickSpy struct {
	inner  strategy.Strategy
	tickMs *int64
}

func (s *tickSpy) Name() string                               { return s.inner.Name() }
func (s *tickSpy) OnTick(t marketdata.Tick) []strategy.Signal { return s.inner.OnTick(t) }
func (s *tickSpy) OnCandle(t marketdata.Tick) []strategy.Signal {
	*s.tickMs = t.TimeMs
	return s.inner.OnCandle(t)
}

func runProbe(sh *engineShared, ds *dataset, decs []decision) {
	var ms int64
	p := &probeStrategy{tickMs: &ms}
	adapter := backtest.NewScalerV3Adapter(p, sh.cb, scalers.RegimeTrending, sh.cfg.Symbol, sh.cfg.SizeBTC)
	_, _ = v3.NewEngine(sh.cfg.V3Config, sh.fr, nil).Run(v3.V3Input{Config: sh.cfg.V3Config,
		Ticks: sh.ticks, Strategy: &tickSpy{inner: adapter, tickMs: &ms}})
	fmt.Println("ENGINE harness — what the strategy sees when it is evaluated (the entry fills at this tick):")
	report := func(rows []probeRow) {
		stale, future1h, future4h, future1m := 0, 0, 0, 0
		var lagSum time.Duration
		for _, r := range rows {
			lag := r.tick.Sub(r.last15mClose)
			lagSum += lag
			if lag > 15*time.Minute {
				stale++
			}
			if r.last1hClose.After(r.tick) {
				future1h++
			}
			if r.last4hClose.After(r.tick) {
				future4h++
			}
			if r.last1mClose.After(r.tick) {
				future1m++
			}
		}
		n := len(rows)
		fmt.Printf("  evaluations %d | mean age of newest 15m bar at the entry tick %v | stale >15m: %.1f%%\n",
			n, (lagSum / time.Duration(n)).Round(time.Minute), 100*float64(stale)/float64(n))
		fmt.Printf("  newest bar closes AFTER the entry tick (look-ahead): 1m %.1f%%  1h %.1f%%  4h %.1f%%\n",
			100*float64(future1m)/float64(n), 100*float64(future1h)/float64(n), 100*float64(future4h)/float64(n))
	}
	for _, r := range p.rows[:4] {
		fmt.Printf("  tick %s | ctx.Now %s | newest 15m closes %s | newest 1h closes %s | newest 4h closes %s\n",
			r.tick.Format("01-02 15:04"), r.now.Format("01-02 15:04"), r.last15mClose.Format("01-02 15:04"),
			r.last1hClose.Format("01-02 15:04"), r.last4hClose.Format("01-02 15:04"))
	}
	report(p.rows)

	fmt.Println("FIXED clock — the same probe:")
	p2 := &probeStrategy{tickMs: &ms}
	for i := range decs {
		ms = decs[i].T * 1000
		p2.Evaluate(ds.contextAt(&decs[i], scalers.RegimeTrending))
	}
	for _, r := range p2.rows[:4] {
		fmt.Printf("  entry %s | ctx.Now %s | newest 15m closes %s | newest 1h closes %s | newest 4h closes %s\n",
			r.tick.Format("01-02 15:04"), r.now.Format("01-02 15:04"), r.last15mClose.Format("01-02 15:04"),
			r.last1hClose.Format("01-02 15:04"), r.last4hClose.Format("01-02 15:04"))
	}
	report(p2.rows)
}
