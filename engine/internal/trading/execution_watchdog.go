package trading

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/observability"
)

const (
	defaultWatchdogInterval   = 2 * time.Minute
	defaultNoTradeAlertWindow = 45 * time.Minute
	defaultStaleFeedWindow    = 3 * time.Minute
)

var noTradeThresholds = []struct {
	window time.Duration
	label  string
}{
	{1 * time.Hour, "1h"},
	{6 * time.Hour, "6h"},
	{12 * time.Hour, "12h"},
	{24 * time.Hour, "24h"},
}

// WatchdogHealth is a point-in-time snapshot for health endpoints.
type WatchdogHealth struct {
	LastTickAt       time.Time `json:"last_tick_at,omitempty"`
	LastSignalAt     time.Time `json:"last_signal_at,omitempty"`
	LastFillAt       time.Time `json:"last_fill_at,omitempty"`
	KillSwitchActive bool      `json:"kill_switch_active"`
	KillSwitchReason string    `json:"kill_switch_reason,omitempty"`
	StaleMarketData  bool      `json:"stale_market_data"`
	NoTradeSinceFill time.Duration `json:"no_trade_since_fill,omitempty"`
	NoTradeAlertLevel string   `json:"no_trade_alert_level,omitempty"`
}

// ExecutionWatchdog monitors trading pipeline health and logs alerts when
// market data, signals, or fills go stale. It does not bypass risk controls.
type ExecutionWatchdog struct {
	client         marketdata.MarketDataClient
	killSwitch     *killswitch.Service
	interval       time.Duration
	noTradeAfter   time.Duration
	staleFeedAfter time.Duration

	lastTickAt   atomic.Int64
	lastSignalAt atomic.Int64
	lastFillAt   atomic.Int64
	lastAlertAt  atomic.Int64

	noTradeAlertFired atomic.Int64 // bitmask: bit i = threshold i fired this boot
}

// NewExecutionWatchdog creates a watchdog for the orchestrator hot path.
func NewExecutionWatchdog(client marketdata.MarketDataClient, ks *killswitch.Service) *ExecutionWatchdog {
	return &ExecutionWatchdog{
		client:         client,
		killSwitch:     ks,
		interval:       defaultWatchdogInterval,
		noTradeAfter:   defaultNoTradeAlertWindow,
		staleFeedAfter: defaultStaleFeedWindow,
	}
}

func (w *ExecutionWatchdog) RecordTick() {
	now := time.Now().UnixNano()
	w.lastTickAt.Store(now)
	observability.MockTradingLastTickUnix.Set(float64(now / int64(time.Second)))
}

func (w *ExecutionWatchdog) RecordSignal() {
	now := time.Now().UnixNano()
	w.lastSignalAt.Store(now)
	observability.MockTradingLastSignalUnix.Set(float64(now / int64(time.Second)))
	observability.MockTradingSignalsTotal.Inc()
	observability.StrategySignals.WithLabelValues("mock-desk", "BTC-USD", "approved").Inc()
}

func (w *ExecutionWatchdog) RecordFill() {
	now := time.Now().UnixNano()
	w.lastFillAt.Store(now)
	observability.MockTradingLastFillUnix.Set(float64(now / int64(time.Second)))
	observability.MockTradingFillsTotal.Inc()
	observability.OrdersSubmitted.WithLabelValues("paper", "BTC-USD", "unknown", "market").Inc()
	w.noTradeAlertFired.Store(0)
}

// Health returns the current watchdog snapshot.
func (w *ExecutionWatchdog) Health() WatchdogHealth {
	now := time.Now()
	h := WatchdogHealth{
		LastTickAt:   unixNanoToTime(w.lastTickAt.Load()),
		LastSignalAt: unixNanoToTime(w.lastSignalAt.Load()),
		LastFillAt:   unixNanoToTime(w.lastFillAt.Load()),
	}
	if w.killSwitch != nil {
		h.KillSwitchActive = w.killSwitch.IsActive()
		h.KillSwitchReason = w.killSwitch.Reason()
	}
	if !h.LastTickAt.IsZero() && now.Sub(h.LastTickAt) > w.staleFeedAfter {
		h.StaleMarketData = true
	}
	if !h.LastFillAt.IsZero() {
		h.NoTradeSinceFill = now.Sub(h.LastFillAt)
		h.NoTradeAlertLevel = w.noTradeLevel(h.NoTradeSinceFill)
	}
	return h
}

func (w *ExecutionWatchdog) noTradeLevel(d time.Duration) string {
	level := ""
	for _, th := range noTradeThresholds {
		if d >= th.window {
			level = th.label
		}
	}
	return level
}

// Run polls health indicators until ctx is cancelled.
func (w *ExecutionWatchdog) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	log.Printf("[WATCHDOG] execution watchdog started (interval=%s noTradeAlert=%s)", w.interval, w.noTradeAfter)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.check(ctx)
		}
	}
}

func (w *ExecutionWatchdog) check(ctx context.Context) {
	now := time.Now()
	_ = ctx

	ksActive := false
	if w.killSwitch != nil {
		ksActive = w.killSwitch.IsActive()
		observability.KillSwitchActive.Set(boolToFloat(ksActive))
		if ksActive {
			log.Printf("[WATCHDOG] ALERT kill_switch_active reason=%q — new orders blocked", w.killSwitch.Reason())
			observability.MockTradingWatchdogAlertsTotal.WithLabelValues("kill_switch_active").Inc()
			w.maybeAlert(now)
			return
		}
	}
	observability.KillSwitchActive.Set(0)

	if last := unixNanoToTime(w.lastTickAt.Load()); !last.IsZero() && now.Sub(last) > w.staleFeedAfter {
		log.Printf("[WATCHDOG] ALERT stale_market_data last_tick=%s ago=%s", last.UTC().Format(time.RFC3339), now.Sub(last).Round(time.Second))
		observability.MockTradingWatchdogAlertsTotal.WithLabelValues("stale_market_data").Inc()
		w.maybeAlert(now)
	}

	lastFill := unixNanoToTime(w.lastFillAt.Load())
	if lastFill.IsZero() {
		return
	}
	sinceFill := now.Sub(lastFill)
	w.checkNoTradeThresholds(now, sinceFill, lastFill)
}

func (w *ExecutionWatchdog) checkNoTradeThresholds(now time.Time, sinceFill time.Duration, lastFill time.Time) {
	lastSig := unixNanoToTime(w.lastSignalAt.Load())
	fired := w.noTradeAlertFired.Load()

	for i, th := range noTradeThresholds {
		if sinceFill < th.window {
			continue
		}
		bit := int64(1 << i)
		if fired&bit != 0 {
			continue
		}
		log.Printf("[WATCHDOG] ALERT no_fills_%s last_fill=%s last_signal=%s — verify gates and reconciliation",
			th.label,
			lastFill.UTC().Format(time.RFC3339),
			formatOptionalTime(lastSig),
		)
		observability.MockTradingNoTradeAlertTotal.WithLabelValues(th.label).Inc()
		observability.MockTradingWatchdogAlertsTotal.WithLabelValues("no_fills_" + th.label).Inc()
		fired |= bit
		w.noTradeAlertFired.Store(fired)
		w.maybeAlert(now)
	}

	if sinceFill > w.noTradeAfter {
		log.Printf("[WATCHDOG] ALERT no_fills last_fill=%s ago=%s last_signal=%s",
			lastFill.UTC().Format(time.RFC3339),
			sinceFill.Round(time.Second),
			formatOptionalTime(lastSig),
		)
	}
}

func (w *ExecutionWatchdog) maybeAlert(now time.Time) {
	last := unixNanoToTime(w.lastAlertAt.Load())
	if !last.IsZero() && now.Sub(last) < w.interval {
		return
	}
	w.lastAlertAt.Store(now.UnixNano())
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func unixNanoToTime(ns int64) time.Time {
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.UTC().Format(time.RFC3339)
}
