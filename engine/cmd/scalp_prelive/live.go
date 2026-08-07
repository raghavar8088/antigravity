package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"antigravity-engine/internal/delta"
)

// Live perpetual trading from the scalp desk.
//
// This process was deliberately paper-only: "it holds no keys and has no order
// routing". That property is now conditional rather than absolute, at the
// owner's instruction, for a $100 account trading eight selected streams.
//
// The conditions are what keep it defensible:
//
//   - It does nothing at all unless SCALP_LIVE_ENABLED is true AND Delta
//     credentials are present. Absent either, this file is inert and the desk is
//     exactly as paper-only as it was before.
//   - It ships DISARMED even when enabled. Arming is a separate, authenticated,
//     typed-confirmation action.
//   - Only the eight allow-listed (strategy, symbol) streams can reach the venue.
//   - Sizing comes from risk on a $100 account, never from the paper desk's
//     $3,000 notional.
//
// The desk's paper record is completely unaffected: every stream keeps trading
// on paper regardless of what the live bridge does, so the leaderboard remains a
// clean measurement rather than a mixture of paper and live outcomes.

// liveDesk is the desk's optional live-trading arm. A nil *liveDesk is a fully
// working paper desk, which is why every method tolerates a nil receiver.
type liveDesk struct {
	bridge *delta.PerpBridge
	reg    *delta.PerpRegistry
}

// scalpLiveEnabled reports whether live trading is switched on at all.
// Defaults OFF: a desk that starts routing orders because an env var was
// forgotten is not a desk anyone should run.
func scalpLiveEnabled() bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("SCALP_LIVE_ENABLED")))
	return err == nil && v
}

// scalpLiveEquityUSD is the account this desk may risk.
func scalpLiveEquityUSD() float64 {
	if raw := strings.TrimSpace(os.Getenv("SCALP_LIVE_EQUITY_USD")); raw != "" {
		if f, err := strconv.ParseFloat(raw, 64); err == nil && f > 0 {
			return f
		}
	}
	return 100
}

// scalpLiveSymbols is the symbol half of the allow-list. Empty means "any symbol
// the allow-listed strategies trade", which is broader than the owner selected,
// so it is set explicitly by default.
// scalpLiveSymbols is the distinct symbols in the live stream selection.
//
// Derived from the streams rather than configured separately, so leverage is
// set on exactly the products that can be traded and no others. Keeping a
// second list meant the two could disagree — and the way they disagreed was by
// permitting more than was chosen.
//
// SCALP_LIVE_SYMBOLS still overrides, for operating a subset without a redeploy.
func scalpLiveSymbols() []string {
	if raw := strings.TrimSpace(os.Getenv("SCALP_LIVE_SYMBOLS")); raw != "" {
		out := []string{}
		for _, p := range strings.Split(raw, ",") {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, strings.ToUpper(s))
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, st := range delta.ScalpLiveStreams() {
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if sym != "" && !seen[sym] {
			seen[sym] = true
			out = append(out, sym)
		}
	}
	return out
}

// newLiveDesk builds the live arm, or returns nil when live trading is off or
// unconfigured. A nil return is the normal, safe case.
func newLiveDesk(ctx context.Context) *liveDesk {
	if !scalpLiveEnabled() {
		log.Printf("[SCALP LIVE] disabled (SCALP_LIVE_ENABLED not true) — desk is paper-only")
		return nil
	}
	// Credentials are read by the client itself from the environment; they are
	// never logged, echoed or held here.
	if strings.TrimSpace(os.Getenv("DELTA_API_KEY")) == "" || strings.TrimSpace(os.Getenv("DELTA_API_SECRET")) == "" {
		// Enabled but unconfigured is a misconfiguration, not a reason to guess.
		log.Printf("[SCALP LIVE] ⚠️  SCALP_LIVE_ENABLED is set but Delta credentials are absent — staying paper-only")
		return nil
	}

	client, err := delta.NewClient()
	if err != nil {
		log.Printf("[SCALP LIVE] ⚠️  Delta client unavailable (%v) — staying paper-only", err)
		return nil
	}
	reg := delta.NewPerpRegistry()
	if err := reg.Refresh(ctx); err != nil {
		log.Printf("[SCALP LIVE] ⚠️  product registry refresh failed (%v) — staying paper-only rather than sizing on unknown contracts", err)
		return nil
	}

	equity := scalpLiveEquityUSD()
	b := delta.NewPerpBridge(client, reg, equity)
	// Exact streams, not strategies x symbols. The cross product enabled
	// pairings the operator never selected — three chosen rows became six live
	// streams — and the allow-list is the last thing between a paper signal and
	// real money.
	b.AllowList().SetPairs(delta.ScalpLiveStreams())

	log.Printf("[SCALP LIVE] configured: $%.2f account, %d strategies, %d products known — DISARMED until armed explicitly",
		equity, b.AllowList().Count(), reg.Count())

	// Persist the open book alongside the desk's own state, on the mounted
	// volume, so custody survives a restart instead of stranding funded
	// positions with no stop, target or time stop.
	// Set the account's per-product leverage BEFORE anything can trade.
	//
	// Delta ships these products at 100x, which puts the liquidation price 0.5%
	// from entry — inside every one of this desk's 0.35%-0.98% stops. Two
	// positions were force-closed at exactly 0.500% before their own stops were
	// reached. Without this the venue, not the strategy, decides every exit.
	lctx, lcancel := context.WithTimeout(ctx, 30*time.Second)
	if err := b.EnsureLeverage(lctx, scalpLiveSymbols()); err != nil {
		log.Printf("[SCALP LIVE] ⚠️  leverage not fully applied (%v) — affected symbols will be refused by the stop-reachability guard", err)
	}
	lcancel()

	b.SetStateDir(liveStateDir())
	rctx, rcancel := context.WithTimeout(ctx, 45*time.Second)
	if err := b.Restore(rctx); err != nil {
		log.Printf("[SCALP LIVE] custody restore incomplete: %v", err)
	}
	rcancel()

	d := &liveDesk{bridge: b, reg: reg}
	go d.refreshLoop(ctx)
	go b.Monitor(ctx, 15*time.Second)
	return d
}

// liveStateDir is where the live position book lives. It defaults to the desk's
// own state directory, which is a mounted volume in production — a path inside
// the container would be destroyed by the very restart this is meant to survive.
func liveStateDir() string {
	if d := strings.TrimSpace(os.Getenv("SCALP_LIVE_STATE_DIR")); d != "" {
		return d
	}
	return "/app/data/scalp_prelive"
}

// refreshLoop keeps the product registry fresh. A stale registry refuses to
// size, so letting it age would silently stop live trading.
func (d *liveDesk) refreshLoop(ctx context.Context) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := d.reg.Refresh(ctx); err != nil {
				log.Printf("[SCALP LIVE] product refresh failed: %v", err)
			}
		}
	}
}

// liveFillHook is the seam between the paper desk and real money.
//
// It exists as a variable so a test can prove that a given fill path reaches the
// live bridge at all. That is not a hypothetical concern: the ANTI_ mirrors were
// silently unable to trade because their fill path never called it, and no test
// could observe the omission without this.
var liveFillHook = func(d *liveDesk, strategy, symbol string, pos *position) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	d.bridge.OnPaperOpen(ctx, strategy, symbol, pos.Dir == "LONG", pos.Entry, pos.SL, pos.TP,
		profileTTL(pos.Profile))
}

// profileTTL is the holding-period cap this strategy's execution profile uses,
// as a duration. Bars are one minute on this desk.
//
// It is passed to the live bridge because the time stop is not a fallback here —
// over 500 measured paper trades it accounted for 456 of the exits (91.2%),
// against 30 stops and 14 targets. A live position without it reproduces under
// 9% of the desk's behaviour and holds the rest indefinitely.
func profileTTL(profile string) time.Duration {
	cfg, ok := profiles[profile]
	if !ok || cfg.TTLBars <= 0 {
		// An unknown profile must not mean "hold forever". Fall back to the
		// longest configured cap rather than to no cap at all.
		longest := 0
		for _, c := range profiles {
			if c.TTLBars > longest {
				longest = c.TTLBars
			}
		}
		return time.Duration(longest) * time.Minute
	}
	return time.Duration(cfg.TTLBars) * time.Minute
}

// onPaperFill mirrors a paper entry into a real order. Safe on a nil receiver.
func (d *liveDesk) onPaperFill(strategy, symbol string, pos *position) {
	if pos == nil {
		return
	}
	if d == nil {
		// Paper-only, but still announce the stream so tests (and any future
		// observer) can see which fills WOULD have been offered to the bridge.
		if observeFill != nil {
			observeFill(strategy, symbol, pos)
		}
		return
	}
	if observeFill != nil {
		observeFill(strategy, symbol, pos)
	}
	liveFillHook(d, strategy, symbol, pos)
}

// observeFill, when set, receives every fill offered to the live path. Test-only.
var observeFill func(strategy, symbol string, pos *position)

// reportUnknown logs allow-listed names the desk does not actually run.
func (d *liveDesk) reportUnknown(names []string) {
	if d == nil {
		return
	}
	known := make(map[string]bool, len(names))
	for _, n := range names {
		known[n] = true
	}
	d.bridge.AllowList().ReportUnknown(known)
}

// registerHTTP mounts the live control plane. Every mutation is token-gated and
// requires a typed confirmation, matching the Live Engine's posture.
func (d *liveDesk) registerHTTP(
	gated func(http.HandlerFunc) http.HandlerFunc,
	postOnly func(http.HandlerFunc) http.HandlerFunc,
	writeJSON func(http.ResponseWriter, interface{}),
) {
	http.HandleFunc("/scalp/live/stats", gated(func(w http.ResponseWriter, r *http.Request) {
		if d == nil {
			writeJSON(w, map[string]any{"enabled": false, "reason": "live trading not configured"})
			return
		}
		s := d.bridge.Stats()
		writeJSON(w, map[string]any{"enabled": true, "stats": s})
	}))

	http.HandleFunc("/scalp/live/arm", gated(postOnly(func(w http.ResponseWriter, r *http.Request) {
		if d == nil {
			http.Error(w, "live trading not configured", http.StatusPreconditionFailed)
			return
		}
		// Typed confirmation. Arming spends real money, so it must not be
		// reachable by a stray POST or a curious click.
		var body struct {
			Confirm string `json:"confirm"`
			Actor   string `json:"actor"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Confirm != "ARM LIVE TRADING" {
			http.Error(w, `confirmation required: {"confirm":"ARM LIVE TRADING"}`, http.StatusBadRequest)
			return
		}
		actor := body.Actor
		if actor == "" {
			actor = "operator"
		}
		if err := d.bridge.Arm(actor, "manual arm"); err != nil {
			http.Error(w, err.Error(), http.StatusPreconditionFailed)
			return
		}
		writeJSON(w, map[string]any{"armed": true})
	})))

	http.HandleFunc("/scalp/live/disarm", gated(postOnly(func(w http.ResponseWriter, r *http.Request) {
		if d == nil {
			http.Error(w, "live trading not configured", http.StatusPreconditionFailed)
			return
		}
		d.bridge.Disarm("operator", "manual disarm")
		writeJSON(w, map[string]any{"armed": false})
	})))

	// Flattening must always be reachable, even when nothing else is.
	http.HandleFunc("/scalp/live/close-all", gated(postOnly(func(w http.ResponseWriter, r *http.Request) {
		if d == nil {
			http.Error(w, "live trading not configured", http.StatusPreconditionFailed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		n, err := d.bridge.CloseAll(ctx)
		resp := map[string]any{"closed": n}
		if err != nil {
			resp["error"] = err.Error()
		}
		writeJSON(w, resp)
	})))

	http.HandleFunc("/scalp/live/trades", gated(func(w http.ResponseWriter, r *http.Request) {
		if d == nil {
			writeJSON(w, []any{})
			return
		}
		writeJSON(w, d.bridge.History())
	}))

	http.HandleFunc("/scalp/live/reconcile", gated(func(w http.ResponseWriter, r *http.Request) {
		if d == nil {
			writeJSON(w, map[string]any{"enabled": false})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		// matched comes from NET SIZE agreement, not from equal counts. Delta
		// nets by symbol, so two bridge positions on one symbol are legitimately
		// one venue row — comparing counts reported a mismatch during normal
		// operation and trained the operator to ignore the alarm.
		mine, venue, matched, err := d.bridge.Reconcile(ctx)
		resp := map[string]any{"bridgePositions": mine, "deltaPositions": venue, "matched": matched}
		if err != nil {
			resp["error"] = err.Error()
		}
		writeJSON(w, resp)
	}))
}
