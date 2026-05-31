package funding

import (
	"context"
	"sync"
	"time"

	"antigravity-engine/internal/alpha"
)

type Regime string

const (
	RegimePositiveExtreme Regime = "POSITIVE_EXTREME"
	RegimeNegativeExtreme Regime = "NEGATIVE_EXTREME"
	RegimeNeutral         Regime = "NEUTRAL"
)

type Metrics struct {
	ZScore     float64
	Percentile float64
	Momentum   float64
	Regime     Regime
}

type Engine struct {
	mu        sync.RWMutex
	cache     *Cache
	collector *Collector
}

func NewEngine(cache *Cache, collector *Collector) *Engine {
	if collector == nil {
		collector = NewCollector()
	}
	return &Engine{cache: cache, collector: collector}
}

func (e *Engine) CollectOnce(ctx context.Context, exchanges []string, symbols []string) []error {
	errs := make([]error, 0)
	for _, exchange := range exchanges {
		for _, symbol := range symbols {
			snap, err := e.collector.Fetch(ctx, exchange, symbol)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if e.cache != nil {
				if err := e.cache.Add(snap); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	return errs
}

func (e *Engine) Metrics(exchange, symbol string, window time.Duration) Metrics {
	if e.cache == nil {
		return Metrics{Percentile: 50, Regime: RegimeNeutral}
	}
	rows := e.cache.History(exchange, symbol, window)
	if len(rows) == 0 {
		return Metrics{Percentile: 50, Regime: RegimeNeutral}
	}
	rates := make([]float64, 0, len(rows))
	for _, row := range rows {
		rates = append(rates, row.FundingRate)
	}
	current := rates[len(rates)-1]
	momentum := 0.0
	if len(rates) >= 2 {
		momentum = current - rates[len(rates)-2]
	}
	percentile := alpha.PercentileRank(current, rates)
	regime := RegimeNeutral
	if current >= 0.001 || percentile >= 90 {
		regime = RegimePositiveExtreme
	} else if current <= -0.0005 || percentile <= 10 {
		regime = RegimeNegativeExtreme
	}
	return Metrics{
		ZScore:     alpha.ZScore(current, rates),
		Percentile: percentile,
		Momentum:   momentum,
		Regime:     regime,
	}
}
