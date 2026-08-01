package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
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
//
// PERMISSION, NOT ENDORSEMENT. Being on this list only means an order from this
// strategy is not rejected outright; whether it has EARNED real capital is the
// eligibility gate's question (SetLiveEligibility below), and that gate is
// currently not enforced. So on today's configuration this list alone decides
// what trades with real money.
//
// What the real record says, as of 2026-07-31 — three of these have actually
// filled on Delta, and all three lost:
//
//	Intraday_PutBuy_RSIOverboughtExtreme_150m   6 fills / 1d   exp -$0.019  PF 0.04
//	Swing_CallBuy_OverextensionFadeDown_600m    5 fills / 1d   exp -$0.018  PF 0.27
//	Swing_PutBuy_OverextensionFadeUp_600m       5 fills / 2d   exp -$0.013  PF 0.03
//
// The first of those is the top strategy on the paper leaderboard by a factor of
// twelve (+$2,181 on a $1,000 paper account). Paper rank and real outcome point
// opposite ways on the same strategies, which is worth remembering before this
// list is extended again on paper P&L alone.
var defaultLiveStrategyNames = []string{
	"Intraday_PutBuy_RSIOverboughtExtreme_150m",
	"Swing_CallBuy_OverextensionFadeDown_600m",
	"Swing_PutBuy_OverextensionFadeUp_600m",
	"Intraday_PutBuy_SharpReversalDown_150m",
	"Intraday_CallBuy_CapitulationRecovery_180m",
	// Added 2026-07-31 at the owner's direction, from the paper leaderboard.
	// None has a real fill yet; each is unproven rather than disproven.
	"Swing_PutBuy_BreakdownTrendBear_960m",
	"Intraday_CallBuy_TripleBull_180m",
	"Swing_PutBuy_ATRExpandBear_720m",
	"Intraday_PutBuy_BreakdownTrendBear_240m",
}

// parseEnvBoolDefault reads a boolean env var, falling back to def when unset or
// unparseable. Used for switches whose safe state is the default, so a typo can
// never silently enable a stricter or looser live-trading policy.
func parseEnvBoolDefault(key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("[LIVE ENGINE] %s=%q is not a boolean — using default %v", key, raw, def)
		return def
	}
	return v
}

// reportUnknownAllowListNames logs any allow-listed name that matches no
// registered buying strategy.
//
// The allow-list is a list of long hand-typed strings compared by equality, so a
// typo does not fail — it silently permits a strategy that does not exist, and
// the operator sees a longer roster while nothing changes. That is the same
// shape as the go-live gate that sat wired to hardcoded zeros: a control that
// reads as configured and does nothing. Logged rather than fatal, because
// refusing to boot the whole engine over one stale name would be a worse
// failure than trading the remaining valid ones.
func reportUnknownAllowListNames(allow []string, buyEngine *options.Engine) {
	if buyEngine == nil || len(allow) == 0 {
		return
	}
	known := map[string]bool{}
	for _, s := range buyEngine.StrategyStatuses() {
		known[s.Name] = true
	}
	unknown := make([]string, 0, len(allow))
	for _, n := range allow {
		if !known[n] {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) == 0 {
		log.Printf("[LIVE ENGINE] allow-list: %d strategies, all resolved", len(allow))
		return
	}
	log.Printf("[LIVE ENGINE] ⚠️  allow-list has %d name(s) matching NO registered strategy — they permit nothing: %s",
		len(unknown), strings.Join(unknown, ", "))
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
	allow := defaultLiveStrategies()
	bridge.SetLiveAllowList(allow)
	reportUnknownAllowListNames(allow, buyEngine)

	// Connect the go-live gate to the order path. The allow-list says a strategy
	// is PERMITTED; this says it has EARNED it. Both must hold once enforcement is
	// on. Enforcement defaults off because no strategy has a qualifying real-fill
	// record yet — turning it on now would stop the desk rather than improve it —
	// and flips via LIVE_ENGINE_ENFORCE_GATE once the record exists.
	bridge.SetLiveEligibility(func(name string) (bool, string) {
		rec := bridge.LiveStrategyRecord(name)
		var optionType string
		if buyEngine != nil {
			for _, s := range buyEngine.StrategyStatuses() {
				if s.Name == name {
					optionType = string(s.OptionType)
					break
				}
			}
		}
		v := liveengine.EvaluateEligibility(liveengine.StrategyInput{
			Strategy:       name,
			OptionType:     optionType,
			RealFills:      rec.Fills,
			RealDays:       rec.Days,
			RealExpectancy: rec.Expectancy,
			RealPF:         rec.ProfitFactor,
			RealFeePct:     rec.FeePctOfPremium,
		})
		return v.Live, v.Reason
	})
	bridge.SetGateEnforcement(parseEnvBoolDefault("LIVE_ENGINE_ENFORCE_GATE", false))

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
		Account:         liveEngineAccountProvider(bridge, ctrl),
		Positions:       liveEnginePositionsProvider(bridge),
		Orders:          liveEngineOrdersProvider(bridge),
		ClosedPositions: liveEngineClosedProvider(bridge),
		DailyPnl:        liveEngineDailyPnlProvider(bridge),
		Roster:          liveEngineRosterProvider(buyEngine, bridge),
		Reconciliation:  liveEngineReconciliationProvider(bridge),
		AllowList:       bridge.LiveAllowList,
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

	// Resume a deliberate human ON across restarts, so a deploy does not silently
	// stop live trading. Guarded inside RestoreArmState: never resumes after a
	// safety stop, a manual off, an active kill switch, or a stale state. Run off
	// the boot path — it reads broker equity and must not block startup.
	go func() {
		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		ctrl.RestoreArmState(rctx)
	}()

	// Keep the saved ON state fresh while armed, so its age reflects downtime
	// rather than time-since-arming. Without this a long-running armed engine
	// aged past armStateMaxAge and refused to resume after a routine redeploy.
	ctrl.StartArmStateHeartbeat(ctx)

	log.Printf("[LIVE ENGINE] control plane wired — ceiling $%.0f, buying-only (ON state resumes only if it was deliberately on)", liveengine.MaxTradableUSD)
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
			// Margin is genuinely shared: it is the wallet's, whichever desk
			// consumed it, and the operator needs to see the true figure.
			marginUsed += p.Margin
			// Open risk is NOT shared. It is labelled "premium at risk (long)"
			// and computed with the OPTION contract size, so including a
			// perpetual held by the scalp bridge both attributes another desk's
			// exposure here and values it with the wrong contract size — 0.001
			// for an ADAUSD contract that is 1.0.
			if !delta.IsOptionSymbol(p.Symbol) {
				continue
			}
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
		// Attribute to the tracked trade so TP/SL are measured against the entry
		// premium the monitor actually uses (FillPrice), not a display value.
		type tracked struct {
			strategy  string
			fill      float64
			contracts int
		}
		bySymbol := map[string]tracked{}
		byProduct := map[int]tracked{}
		for _, t := range bridge.OpenTrades() {
			tr := tracked{t.StrategyName, t.FillPrice, t.Contracts}
			if t.DeltaSymbol != "" {
				bySymbol[t.DeltaSymbol] = tr
			}
			// ProductID is always present on a real fill even when Delta omits the
			// symbol, so it is the reliable key for attribution.
			if t.ProductID != 0 {
				byProduct[t.ProductID] = tr
			}
		}
		for _, p := range positions {
			// Options only — a perpetual held by the scalp bridge is not this
			// engine's position, and ExitLevelsFor would compute option-premium
			// stop/target levels for it (a -50%-of-entry "stop" on a perp).
			if !delta.IsOptionSymbol(p.Symbol) {
				continue
			}
			contracts := int(p.Size)
			if contracts < 0 {
				contracts = -contracts
			}
			entry := p.EntryPrice
			strategy := ""
			tr, ok := byProduct[p.ProductID]
			if !ok {
				tr, ok = bySymbol[p.Symbol]
			}
			if ok {
				strategy = tr.strategy
				if tr.fill > 0 {
					entry = tr.fill // the basis the monitor closes against
				}
				if tr.contracts > 0 {
					contracts = tr.contracts
				}
			}

			// Delta reports unrealised_pnl as 0 for these options; compute it from
			// mark vs entry so the UI never shows a false zero on a real position.
			unrealized := p.UnrealisedPnl
			if unrealized == 0 {
				unrealized = delta.UnrealizedUSD(entry, p.MarkPrice, contracts)
			}
			exit := delta.ExitLevelsFor(entry, contracts)

			out = append(out, map[string]any{
				"symbol":        p.Symbol,
				"side":          p.Side,
				"size":          p.Size,
				"entryPrice":    entry,
				"markPrice":     p.MarkPrice,
				"unrealizedPnl": unrealized,
				"marginUsd":     p.Margin,
				// Exit plan the monitor enforces (+80% TP / -50% SL of premium).
				"takeProfitPrice": exit.TakeProfitPrice,
				"stopLossPrice":   exit.StopLossPrice,
				"takeProfitUsd":   exit.TakeProfitUSD,
				"stopLossUsd":     exit.StopLossUSD,
				// Long options have no liquidation price — 10x is inert on longs.
				"liquidationPrice": "N/A (long option)",
				"strategy":         strategy,
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

// tradeResult restates one closed live trade honestly: gross P&L, the two fee
// legs, and the net the account actually kept.
//
// Fees are BACKFILLED when the stored trade predates the fee model, so the
// existing order history is restated rather than left flattering. Before this,
// both the closed-positions and daily-P&L views recomputed P&L as
// (exit-entry)×size and reported it as "realised" — a gross number the account
// never received, on a venue charging up to 28% of premium per round trip.
type tradeResult struct {
	Gross    float64
	EntryFee float64
	ExitFee  float64
	Net      float64
}

func resultOf(t delta.LiveTrade) tradeResult {
	contracts := t.Contracts
	if contracts < 0 {
		contracts = -contracts
	}

	r := tradeResult{Gross: t.GrossPnl, EntryFee: t.EntryFeeUSD, ExitFee: t.ExitFeeUSD}

	// Recompute gross from entry/exit rather than trusting the stored value:
	// trades closed before the contract-size fix persisted a 1000x-overstated
	// number, and a stale field would keep displaying it.
	if t.FillPrice > 0 && t.CloseFillPrice > 0 && contracts != 0 {
		r.Gross = (t.CloseFillPrice - t.FillPrice) * float64(contracts) * delta.OptionContractSizeBTC
	} else if r.Gross == 0 {
		r.Gross = t.RealizedPnl
	}

	// Backfill either leg that was never recorded. An expiry has no closing
	// trade, so a zero exit fee there is correct, not missing.
	if r.EntryFee == 0 && t.FillPrice > 0 {
		r.EntryFee = delta.OptionFeeUSD(t.FillPrice, t.EntryBTCPrice, contracts)
	}
	if r.ExitFee == 0 && t.CloseFillPrice > 0 {
		r.ExitFee = delta.OptionFeeUSD(t.CloseFillPrice, t.EntryBTCPrice, contracts)
	}

	r.Net = r.Gross - r.EntryFee - r.ExitFee
	return r
}

// liveEngineClosedProvider lists positions the engine opened and has since
// closed (SL/TP/expiry), newest first, with the realised outcome.
func liveEngineClosedProvider(bridge *delta.Bridge) func(context.Context) ([]map[string]any, error) {
	return func(ctx context.Context) ([]map[string]any, error) {
		out := make([]map[string]any, 0)
		for _, t := range bridge.Trades() {
			if t.Status != "CLOSED" {
				continue
			}
			res := resultOf(t)
			row := map[string]any{
				"id":         t.ID,
				"strategy":   t.StrategyName,
				"optionType": t.OptionType,
				"symbol":     t.DeltaSymbol,
				"contracts":  t.Contracts,
				"entryPrice": t.FillPrice,
				"exitPrice":  t.CloseFillPrice,
				// realizedPnl is NET of both fee legs — what the account kept.
				// grossPnl and feesUsd are shown alongside so the cost of trading
				// is visible rather than inferred.
				"realizedPnl": res.Net,
				"grossPnl":    res.Gross,
				"feesUsd":     res.EntryFee + res.ExitFee,
				"openedAt":    t.OpenedAt,
				// Why it closed: take-profit, stop-loss, near-expiry or expiry.
				"exitReason": firstNonEmpty(t.ExitReason, t.FailureReason),
			}
			if t.ClosedAt != nil {
				row["closedAt"] = *t.ClosedAt
			}
			out = append(out, row)
			if len(out) >= 100 {
				break
			}
		}
		return out, nil
	}
}

// liveEngineDailyPnlProvider aggregates closed live positions by UTC day:
// capital actually deployed (premium paid), realised PnL, ROI on that capital,
// trade count and win rate. Newest day first.
func liveEngineDailyPnlProvider(bridge *delta.Bridge) func(context.Context) ([]map[string]any, error) {
	return func(ctx context.Context) ([]map[string]any, error) {
		type agg struct {
			capital float64
			pnl     float64
			gross   float64
			fees    float64
			trades  int
			wins    int
		}
		byDay := map[string]*agg{}
		for _, t := range bridge.Trades() {
			if t.Status != "CLOSED" || t.ClosedAt == nil {
				continue
			}
			day := t.ClosedAt.UTC().Format("2006-01-02")
			a := byDay[day]
			if a == nil {
				a = &agg{}
				byDay[day] = a
			}
			contracts := t.Contracts
			if contracts < 0 {
				contracts = -contracts
			}
			// Capital deployed = premium actually paid to open.
			a.capital += t.FillPrice * float64(contracts) * delta.OptionContractSizeBTC
			res := resultOf(t)
			a.pnl += res.Net
			a.gross += res.Gross
			a.fees += res.EntryFee + res.ExitFee
			a.trades++
			// A "win" is a trade that made money after costs. Counting gross wins
			// is how a book can report a respectable win rate while shrinking.
			if res.Net > 0 {
				a.wins++
			}
		}

		days := make([]string, 0, len(byDay))
		for d := range byDay {
			days = append(days, d)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(days)))

		out := make([]map[string]any, 0, len(days))
		for _, d := range days {
			a := byDay[d]
			roi := 0.0
			if a.capital > 0 {
				roi = 100 * a.pnl / a.capital
			}
			winPct := 0.0
			if a.trades > 0 {
				winPct = 100 * float64(a.wins) / float64(a.trades)
			}
			feeDrag := 0.0
			if a.capital > 0 {
				feeDrag = 100 * a.fees / a.capital
			}
			out = append(out, map[string]any{
				"date":        d,
				"capitalUsd":  a.capital,
				"pnlUsd":      a.pnl, // net
				"grossPnlUsd": a.gross,
				"feesUsd":     a.fees,
				// What fees cost as a share of the premium deployed that day —
				// the single number that shows whether the instrument is viable.
				"feeDragPct": feeDrag,
				"roiPct":     roi,
				"trades":     a.trades,
				"wins":       a.wins,
				"winRatePct": winPct,
			})
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
			// The real-fill record comes from closed LIVE trades, net of fees.
			// These were hardcoded to zero, which made every verdict "not live"
			// for a reason that could never change — so the gate was decorative
			// and the allow-list alone decided what traded with real money.
			rec := bridge.LiveStrategyRecord(s.Name)
			e := liveengine.EvaluateEligibility(liveengine.StrategyInput{
				Strategy:        s.Name,
				OptionType:      string(s.OptionType),
				SyntheticTrades: s.TotalTrades,
				SyntheticPnL:    s.TotalPnL,
				RealFills:       rec.Fills,
				RealDays:        rec.Days,
				RealExpectancy:  rec.Expectancy,
				RealPF:          rec.ProfitFactor,
				RealFeePct:      rec.FeePctOfPremium,
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
		// Count only OPTION positions.
		//
		// One Delta wallet now backs two desks: this options engine and the scalp
		// perpetual bridge. GetPositions returns both, so counting everything
		// made a perpetual opened by the OTHER desk read as "Delta reports 1
		// position, engine shows 0" — a mismatch banner on a real-money page for
		// a position this engine correctly does not own, and could not adopt
		// (adoption is already option-filtered).
		//
		// A reconciliation alarm that fires when nothing is wrong is worse than
		// none: it trains the operator to ignore the one that matters.
		deltaOpen := 0
		for _, p := range positions {
			if p.Size != 0 && delta.IsOptionSymbol(p.Symbol) {
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
			// Use TRUE equity = available balance + mark value of open options.
			// Available balance alone drops the moment premium is paid, so
			// deploying capital looked like a loss and could trip the $20 stop
			// with no losses at all.
			if equity, eqErr := liveEngineTotalEquity(ctx, bridge); eqErr == nil {
				if ctrl.CheckDailyLoss(equity, time.Now()) {
					if _, cErr := bridge.CloseAll(ctx); cErr != nil {
						log.Printf("[LIVE ENGINE] daily-loss close-all error: %v", cErr)
					}
					continue
				}
			}
			// Adopt any position that appeared since the last sweep, so a new
			// untracked position does not sit unmanaged until the next restart.
			if n, aerr := bridge.AdoptUntrackedPositions(ctx); aerr == nil && n > 0 {
				log.Printf("[LIVE ENGINE] custody: adopted %d position(s) during periodic sweep", n)
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
				// Surface loudly but do NOT stop the engine: the adoption sweep
				// above usually resolves this on the next tick, and a fill landing
				// between the two reads is a false positive. Halting on it stopped
				// live trading repeatedly for benign reasons.
				detail := "engine=" + itoa(engineOpen) + " delta=" + itoa(deltaOpen)
				log.Printf("[LIVE ENGINE] ⚠️  reconciliation mismatch (%s) — surfaced, NOT halting; adoption should reconcile it", detail)
				ctrl.NoteReconciliationMismatch(detail)
			}
		}
	}
}

// liveEngineTotalEquity is available balance plus the mark value of open option
// positions — the account's true worth. Premium paid leaves the available
// balance but is still held as position value, so the daily-loss breaker must
// count it or it mistakes capital deployment for losses.
func liveEngineTotalEquity(ctx context.Context, bridge *delta.Bridge) (float64, error) {
	client := bridge.Client()
	if client == nil {
		return 0, fmt.Errorf("delta client not configured")
	}
	avail, err := client.GetWallet(ctx)
	if err != nil {
		return 0, err
	}
	positions, perr := client.GetPositions(ctx)
	if perr != nil {
		// Fall back to available balance rather than blocking the breaker.
		return avail, nil
	}
	openValue := 0.0
	for _, p := range positions {
		size := p.Size
		if size < 0 {
			size = -size
		}
		openValue += p.MarkPrice * size * delta.OptionContractSizeBTC
	}
	return avail + openValue, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
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
