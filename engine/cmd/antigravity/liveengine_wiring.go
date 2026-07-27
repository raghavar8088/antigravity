package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"antigravity-engine/internal/delta"
	killswitchpkg "antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/liveengine"
	"antigravity-engine/internal/options"
)

// defaultLiveStrategyNames is the owner-selected set of BUYING strategies allowed
// to place live orders. Overridable via LIVE_ENGINE_STRATEGIES (comma-separated)
// and adjustable at runtime via the Live Engine roster API.
var defaultLiveStrategyNames = []string{
	"Intraday_PutBuy_RSIOverboughtExtreme_150m",
	"Swing_CallBuy_OverextensionFadeDown_600m",
	"Swing_PutBuy_OverextensionFadeUp_600m",
	"Intraday_PutBuy_SharpReversalDown_150m",
	"Intraday_CallBuy_CapitulationRecovery_180m",
}

func defaultLiveStrategies() []string {
	if raw := strings.TrimSpace(os.Getenv("LIVE_ENGINE_STRATEGIES")); raw != "" {
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return defaultLiveStrategyNames
}

// wireLiveEngine constructs the Live Engine control plane, wires its hooks to the
// real Delta bridge + kill switch, registers the /api/live-engine HTTP surface,
// and starts the auto-disarm monitor. It ships DISARMED and never arms here —
// arming is a human action via the authenticated typed-confirmation endpoint.
func wireLiveEngine(
	ctx context.Context,
	bridge *delta.Bridge,
	ks *killswitchpkg.Service,
	buyEngine *options.Engine,
) *liveengine.Controller {
	// Buying-only, native long signals: the bridge BUYS the exact option the
	// buying engine opened (bounded risk = premium paid), never inverts, never
	// sells. Gated to the per-strategy allow-list below.
	bridge.SetBuyingMode(true)
	bridge.SetNativeBuyMode(true)
	bridge.SetLiveAllowList(defaultLiveStrategies())

	ctrl := liveengine.New(liveengine.Hooks{
		SetEffectorEnabled: func(enabled bool) { bridge.SetEnabled(enabled) },
		IsConfigured:       bridge.IsConfigured,
		KillSwitchActive:   ks.IsActive,
		KillSwitchReason:   ks.Reason,
		SetKillSwitch: func(ctx context.Context, active bool, actor, reason string) error {
			if active {
				return ks.Trigger(ctx, killswitchpkg.Activation{
					Trigger:    killswitchpkg.TriggerManualOperator,
					Reason:     reason,
					OperatorID: actor,
					Actions: []killswitchpkg.Action{
						killswitchpkg.ActionBlockNewOrders,
						killswitchpkg.ActionSendAlerts,
					},
				})
			}
			return ks.Release(ctx, killswitchpkg.TriggerManualOperator, actor, reason)
		},
		CloseAll: bridge.CloseAll,
		AccountEquityUSD: func(ctx context.Context) (float64, error) {
			if bridge.Client() == nil {
				return 0, nil
			}
			return bridge.Client().GetWallet(ctx)
		},
	})

	data := liveengine.DataProviders{
		Account:        liveEngineAccountProvider(bridge, ctrl),
		Positions:      liveEnginePositionsProvider(bridge),
		Orders:         liveEngineOrdersProvider(bridge),
		Roster:         liveEngineRosterProvider(buyEngine, bridge),
		Reconciliation: liveEngineReconciliationProvider(bridge),
		AllowList:      bridge.LiveAllowList,
		SetAllowList: func(names []string) error {
			bridge.SetLiveAllowList(names)
			ctrl.RecordRosterChange("operator", fmt.Sprintf("live allow-list set to %d strategies", len(names)))
			return nil
		},
	}

	// Track consecutive broker rejects for the auto-disarm trigger.
	bridge.SetSubmitResultHook(func(err error) {
		if err != nil {
			ctrl.RecordReject(err.Error())
		} else {
			ctrl.RecordFillOK()
		}
	})

	handler := liveengine.NewHandler(ctrl, data, liveEngineAuthorizer)
	http.Handle("/api/live-engine/", handler)

	go liveEngineAutoDisarmMonitor(ctx, ctrl, bridge, ks)

	log.Printf("[LIVE ENGINE] control plane wired — DISARMED, ceiling $%.0f, buying-only", liveengine.MaxTradableUSD)
	return ctrl
}

// liveEngineAuthorizer gates mutations (arm/disarm/close-all). It fails closed:
// with no token configured, no mutation is permitted. The Next.js proxy injects
// the server-side token; it never reaches the browser.
func liveEngineAuthorizer(r *http.Request) (string, bool) {
	token := strings.TrimSpace(os.Getenv("LIVE_ENGINE_API_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("INTERNAL_API_SECRET"))
	}
	if token == "" {
		return "", false // fail closed — no auth configured means no mutations
	}
	if r.Header.Get("X-API-Token") != token {
		return "", false
	}
	actor := strings.TrimSpace(r.Header.Get("X-Actor"))
	if actor == "" {
		actor = "operator"
	}
	return actor, true
}

func liveEngineAccountProvider(bridge *delta.Bridge, ctrl *liveengine.Controller) func(context.Context) (liveengine.AccountView, error) {
	return func(ctx context.Context) (liveengine.AccountView, error) {
		now := time.Now().UTC()
		client := bridge.Client()
		if client == nil {
			return liveengine.AccountView{Source: "delta: not configured", AsOf: now, Stale: true, CeilingUSD: liveengine.MaxTradableUSD}, nil
		}
		equity, err := client.GetWallet(ctx)
		if err != nil {
			return liveengine.AccountView{Source: "delta: " + err.Error(), AsOf: now, Stale: true, CeilingUSD: liveengine.MaxTradableUSD}, nil
		}
		positions, _ := client.GetPositions(ctx)
		marginUsed, openRisk := 0.0, 0.0
		for _, p := range positions {
			marginUsed += p.Margin
			// Long options: max loss is the premium paid (mark × size), not margin.
			openRisk += p.MarkPrice * p.Size * delta.DeltaContractSizeBTC
		}
		return liveengine.AccountView{
			EquityUSD:     equity,
			TradableUSD:   ctrl.TradableEquityUSD(equity),
			CeilingUSD:    liveengine.MaxTradableUSD,
			AvailableUSD:  equity - marginUsed,
			MarginUsedUSD: marginUsed,
			OpenRiskUSD:   openRisk,
			Source:        "delta:/v2/wallet+positions",
			AsOf:          now,
			Stale:         false,
		}, nil
	}
}

func liveEnginePositionsProvider(bridge *delta.Bridge) func(context.Context) ([]map[string]any, error) {
	return func(ctx context.Context) ([]map[string]any, error) {
		client := bridge.Client()
		out := make([]map[string]any, 0)
		if client == nil {
			return out, nil
		}
		positions, err := client.GetPositions(ctx)
		if err != nil {
			return out, err
		}
		// Best-effort strategy attribution from open bridge trades by symbol.
		strategyBySymbol := map[string]string{}
		for _, t := range bridge.OpenTrades() {
			if t.DeltaSymbol != "" {
				strategyBySymbol[t.DeltaSymbol] = t.StrategyName
			}
		}
		for _, p := range positions {
			out = append(out, map[string]any{
				"symbol":        p.Symbol,
				"side":          p.Side,
				"size":          p.Size,
				"entryPrice":    p.EntryPrice,
				"markPrice":     p.MarkPrice,
				"unrealizedPnl": p.UnrealisedPnl,
				"marginUsd":     p.Margin,
				// Long options have no liquidation price — 10x is inert on longs.
				"liquidationPrice": "N/A (long option)",
				"strategy":         strategyBySymbol[p.Symbol],
			})
		}
		return out, nil
	}
}

func liveEngineOrdersProvider(bridge *delta.Bridge) func(context.Context) ([]map[string]any, error) {
	return func(ctx context.Context) ([]map[string]any, error) {
		out := make([]map[string]any, 0)
		for _, t := range bridge.Trades() {
			row := map[string]any{
				"id":           t.ID,
				"strategy":     t.StrategyName,
				"optionType":   t.OptionType,
				"strike":       t.Strike,
				"symbol":       t.DeltaSymbol,
				"contracts":    t.Contracts,
				"side":         t.Side,
				"premiumUsd":   t.PremiumUSD,
				"fillPrice":    t.FillPrice,
				"status":       t.Status,
				"deltaOrderId": t.DeltaOrderID,
				"openedAt":     t.OpenedAt,
			}
			if t.FailureReason != "" {
				row["rejectReason"] = t.FailureReason
			}
			out = append(out, row)
		}
		return out, nil
	}
}

func liveEngineRosterProvider(buyEngine *options.Engine, bridge *delta.Bridge) func(context.Context) ([]liveengine.StrategyEligibility, error) {
	return func(ctx context.Context) ([]liveengine.StrategyEligibility, error) {
		out := make([]liveengine.StrategyEligibility, 0)
		if buyEngine == nil {
			return out, nil
		}
		allowed := map[string]bool{}
		for _, n := range bridge.LiveAllowList() {
			allowed[n] = true
		}
		for _, s := range buyEngine.StrategyStatuses() {
			// Real-fill counts are zero until the desk trades live capital; the
			// synthetic Black-Scholes record does not qualify for real money, so
			// every strategy is correctly not-live with an inspectable reason.
			e := liveengine.EvaluateEligibility(liveengine.StrategyInput{
				Strategy:        s.Name,
				OptionType:      string(s.OptionType),
				SyntheticTrades: s.TotalTrades,
				SyntheticPnL:    s.TotalPnL,
				RealFills:       0,
				RealDays:        0,
			})
			e.Allowed = allowed[s.Name]
			out = append(out, e)
		}
		return out, nil
	}
}

func liveEngineReconciliationProvider(bridge *delta.Bridge) func(context.Context) (liveengine.ReconciliationView, error) {
	return func(ctx context.Context) (liveengine.ReconciliationView, error) {
		now := time.Now().UTC()
		client := bridge.Client()
		engineOpen := len(bridge.OpenTrades())
		if client == nil {
			return liveengine.ReconciliationView{Matched: engineOpen == 0, EnginePositions: engineOpen, AsOf: now, Error: "delta client not configured"}, nil
		}
		positions, err := client.GetPositions(ctx)
		if err != nil {
			return liveengine.ReconciliationView{EnginePositions: engineOpen, AsOf: now, Error: err.Error()}, nil
		}
		deltaOpen := 0
		for _, p := range positions {
			if p.Size != 0 {
				deltaOpen++
			}
		}
		view := liveengine.ReconciliationView{
			EnginePositions: engineOpen,
			DeltaPositions:  deltaOpen,
			Matched:         engineOpen == deltaOpen,
			AsOf:            now,
		}
		if !view.Matched {
			view.Mismatches = []string{
				"engine shows " + itoa(engineOpen) + " open live trades; Delta reports " + itoa(deltaOpen) + " positions",
			}
		}
		return view, nil
	}
}

// liveEngineAutoDisarmMonitor is the one-way safety net: while armed, it
// auto-disarms on an active kill switch or a reconciliation mismatch between
// engine state and Delta truth. Re-arming is always a human action.
func liveEngineAutoDisarmMonitor(ctx context.Context, ctrl *liveengine.Controller, bridge *delta.Bridge, ks *killswitchpkg.Service) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !ctrl.IsArmed() {
				continue
			}
			if ks.IsActive() {
				ctrl.AutoDisarm("kill_switch_active", ks.Reason())
				continue
			}
			client := bridge.Client()
			if client == nil {
				continue
			}
			// Daily-loss breaker: disarm + flatten when down the daily limit.
			if equity, eqErr := client.GetWallet(ctx); eqErr == nil {
				if ctrl.CheckDailyLoss(equity, time.Now()) {
					if _, cErr := bridge.CloseAll(ctx); cErr != nil {
						log.Printf("[LIVE ENGINE] daily-loss close-all error: %v", cErr)
					}
					continue
				}
			}
			positions, err := client.GetPositions(ctx)
			if err != nil {
				// Broker/feed unreachable while armed — treat as feed loss.
				ctrl.OnPriceFeedLost("delta positions unreachable: " + err.Error())
				continue
			}
			deltaOpen := 0
			for _, p := range positions {
				if p.Size != 0 {
					deltaOpen++
				}
			}
			if engineOpen := len(bridge.OpenTrades()); engineOpen != deltaOpen {
				ctrl.OnReconciliationMismatch("engine=" + itoa(engineOpen) + " delta=" + itoa(deltaOpen))
			}
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
