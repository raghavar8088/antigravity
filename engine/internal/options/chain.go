package options

import (
	"encoding/json"
	"math"
	"net/http"
	"time"
)

// ── Data types ────────────────────────────────────────────────────────────────

// ChainLeg holds all displayable data for one side (call or put) of a strike
type ChainLeg struct {
	IV     float64 `json:"iv"` // Annualised IV as percent (e.g. 75.3)
	Delta  float64 `json:"delta"`
	Gamma  float64 `json:"gamma"`
	Theta  float64 `json:"theta"`
	Vega   float64 `json:"vega"`
	Mark   float64 `json:"mark"`
	Bid    float64 `json:"bid"`
	Ask    float64 `json:"ask"`
	OI     int     `json:"oi"`
	Volume int     `json:"volume"`
	IsITM  bool    `json:"isItm"`
}

// ChainRow is one strike level in the chain
type ChainRow struct {
	Strike      float64  `json:"strike"`
	IsATM       bool     `json:"isAtm"`
	MoneynessPC float64  `json:"moneynessPC"` // % from ATM, negative = below
	Call        ChainLeg `json:"call"`
	Put         ChainLeg `json:"put"`
}

// ExpiryMeta is one selectable expiry
type ExpiryMeta struct {
	Label string `json:"label"` // "26 Jan", "28 Mar"
	Value string `json:"value"` // RFC3339
	DTE   int    `json:"dte"`   // Days to expiry
}

// ChainResponse is the full API response
type ChainResponse struct {
	UnderlyingPrice float64      `json:"underlyingPrice"`
	BaseIV          float64      `json:"baseIv"` // ATM IV as percent
	Expiries        []ExpiryMeta `json:"expiries"`
	SelectedExpiry  string       `json:"selectedExpiry"`
	ExpiryLabel     string       `json:"expiryLabel"`
	DTE             int          `json:"dte"`
	Chain           []ChainRow   `json:"chain"`
}

// ── Expiry generation ─────────────────────────────────────────────────────────

// nextFriday returns the next Friday at 08:00 UTC on or after `from`.
func nextFriday(from time.Time) time.Time {
	t := from.UTC()
	// Zero out sub-day
	t = time.Date(t.Year(), t.Month(), t.Day(), 8, 0, 0, 0, time.UTC)
	for t.Weekday() != time.Friday {
		t = t.Add(24 * time.Hour)
	}
	// If we land exactly on today's Friday but past 08:00 UTC, skip to next week
	if t.Before(from.UTC()) {
		t = t.Add(7 * 24 * time.Hour)
	}
	return t
}

// generateExpiries returns 6 expiries: 4 weekly + end-of-month + end-of-quarter
func generateExpiries(now time.Time) []ExpiryMeta {
	var expiries []ExpiryMeta
	seen := map[string]bool{}

	// 4 consecutive weekly Fridays
	f := nextFriday(now)
	for i := 0; i < 4; i++ {
		key := f.Format("2006-01-02")
		if !seen[key] {
			expiries = append(expiries, ExpiryMeta{
				Label: f.Format("2 Jan 06"),
				Value: f.Format(time.RFC3339),
				DTE:   int(math.Ceil(f.Sub(now).Hours() / 24)),
			})
			seen[key] = true
		}
		f = f.Add(7 * 24 * time.Hour)
	}

	// Last Friday of the current month
	lastFriday := lastFridayOfMonth(now.Year(), now.Month())
	if !seen[lastFriday.Format("2006-01-02")] {
		expiries = append(expiries, ExpiryMeta{
			Label: lastFriday.Format("2 Jan 06") + " (EOM)",
			Value: lastFriday.Format(time.RFC3339),
			DTE:   int(math.Ceil(lastFriday.Sub(now).Hours() / 24)),
		})
		seen[lastFriday.Format("2006-01-02")] = true
	}

	// Last Friday of the current quarter
	qExpiry := lastFridayOfQuarter(now)
	if !seen[qExpiry.Format("2006-01-02")] {
		expiries = append(expiries, ExpiryMeta{
			Label: qExpiry.Format("2 Jan 06") + " (EoQ)",
			Value: qExpiry.Format(time.RFC3339),
			DTE:   int(math.Ceil(qExpiry.Sub(now).Hours() / 24)),
		})
	}

	return expiries
}

func lastFridayOfMonth(year int, month time.Month) time.Time {
	// Start from the last day of the month and walk back to Friday
	last := time.Date(year, month+1, 0, 8, 0, 0, 0, time.UTC)
	for last.Weekday() != time.Friday {
		last = last.Add(-24 * time.Hour)
	}
	return last
}

func lastFridayOfQuarter(now time.Time) time.Time {
	// Quarter ends: Mar, Jun, Sep, Dec
	qEnd := []time.Month{time.March, time.June, time.September, time.December}
	y := now.Year()
	for _, m := range qEnd {
		t := lastFridayOfMonth(y, m)
		if t.After(now.UTC()) {
			return t
		}
	}
	// Roll to next year Q1
	return lastFridayOfMonth(y+1, time.March)
}

func nextWeeklyExpiryForConfig(from time.Time, cfg ChainConfig) time.Time {
	t := from.UTC()
	t = time.Date(t.Year(), t.Month(), t.Day(), cfg.ExpiryHourUTC, 0, 0, 0, time.UTC)
	for t.Weekday() != cfg.WeeklyExpiryWeekday {
		t = t.Add(24 * time.Hour)
	}
	if t.Before(from.UTC()) {
		t = t.Add(7 * 24 * time.Hour)
	}
	return t
}

func lastWeekdayOfMonthForConfig(year int, month time.Month, cfg ChainConfig) time.Time {
	last := time.Date(year, month+1, 0, cfg.ExpiryHourUTC, 0, 0, 0, time.UTC)
	for last.Weekday() != cfg.WeeklyExpiryWeekday {
		last = last.Add(-24 * time.Hour)
	}
	return last
}

func lastWeekdayOfQuarterForConfig(now time.Time, cfg ChainConfig) time.Time {
	qEnd := []time.Month{time.March, time.June, time.September, time.December}
	y := now.Year()
	for _, m := range qEnd {
		t := lastWeekdayOfMonthForConfig(y, m, cfg)
		if t.After(now.UTC()) {
			return t
		}
	}
	return lastWeekdayOfMonthForConfig(y+1, time.March, cfg)
}

func generateExpiriesForConfig(now time.Time, cfg ChainConfig) []ExpiryMeta {
	weeklyCount := cfg.WeeklyCount
	if weeklyCount <= 0 {
		weeklyCount = 4
	}

	var expiries []ExpiryMeta
	seen := map[string]bool{}

	expiry := nextWeeklyExpiryForConfig(now, cfg)
	for i := 0; i < weeklyCount; i++ {
		key := expiry.Format("2006-01-02")
		if !seen[key] {
			expiries = append(expiries, ExpiryMeta{
				Label: expiry.Format("2 Jan 06"),
				Value: expiry.Format(time.RFC3339),
				DTE:   int(math.Max(1, math.Ceil(expiry.Sub(now).Hours()/24))),
			})
			seen[key] = true
		}
		expiry = expiry.Add(7 * 24 * time.Hour)
	}

	eom := lastWeekdayOfMonthForConfig(now.Year(), now.Month(), cfg)
	eomKey := eom.Format("2006-01-02")
	if !seen[eomKey] {
		expiries = append(expiries, ExpiryMeta{
			Label: eom.Format("2 Jan 06") + " (EOM)",
			Value: eom.Format(time.RFC3339),
			DTE:   int(math.Max(1, math.Ceil(eom.Sub(now).Hours()/24))),
		})
		seen[eomKey] = true
	}

	quarterly := lastWeekdayOfQuarterForConfig(now, cfg)
	quarterKey := quarterly.Format("2006-01-02")
	if !seen[quarterKey] {
		expiries = append(expiries, ExpiryMeta{
			Label: quarterly.Format("2 Jan 06") + " (EOQ)",
			Value: quarterly.Format(time.RFC3339),
			DTE:   int(math.Max(1, math.Ceil(quarterly.Sub(now).Hours()/24))),
		})
	}

	return expiries
}

// ── IV smile model ────────────────────────────────────────────────────────────

// smileIV returns the implied volatility for a given strike, applying a
// realistic volatility smile + negative skew (puts priced higher than calls).
//
//	baseIV  – ATM IV (annualised fraction, e.g. 0.75)
//	spot    – current BTC price
//	strike  – option strike
//	optType – Call or Put
func smileIV(baseIV, spot, strike float64, optType OptionType) float64 {
	return smileIVForProfile(baseIV, spot, strike, optType, defaultOptionsMarketProfile)
}

func smileIVForProfile(baseIV, spot, strike float64, optType OptionType, profile MarketProfile) float64 {
	if spot <= 0 || strike <= 0 {
		return profile.DefaultIV
	}

	cfg := profile.ChainConfig
	m := math.Log(strike / spot)
	iv := baseIV * math.Exp(cfg.SmileFactor*m*m-cfg.SkewFactor*m)

	if iv < profile.MinIV {
		iv = profile.MinIV
	}
	if iv > profile.MaxIV*1.15 {
		iv = profile.MaxIV * 1.15
	}
	return iv
}

// ── OI / Volume simulation ────────────────────────────────────────────────────

// pseudoSeed gives a deterministic "random" int in [0,1000) from a float key
func pseudoSeed(key float64) int {
	bits := math.Float64bits(key)
	return int((bits ^ (bits >> 32)) % 1000)
}

func simulateOI(strike, spot float64, dte int) int {
	return simulateOIForConfig(strike, spot, dte, defaultOptionsMarketProfile.ChainConfig)
}

func simulateOIForConfig(strike, spot float64, dte int, cfg ChainConfig) int {
	if strike <= 0 || spot <= 0 {
		return 0
	}
	dist := math.Abs(math.Log(strike / spot))
	base := cfg.OIBase * math.Exp(-cfg.OIDecay*dist) * math.Sqrt(float64(dte+1))
	noise := 0.70 + float64(pseudoSeed(strike))/1000.0*0.60
	v := int(base * noise)
	if v < 0 {
		v = 0
	}
	return v
}

func simulateVolume(oi int, strike float64) int {
	return simulateVolumeForConfig(oi, strike, defaultOptionsMarketProfile.ChainConfig)
}

func simulateVolumeForConfig(oi int, strike float64, cfg ChainConfig) int {
	noise := cfg.VolumeNoiseFloor + float64(pseudoSeed(strike+1))/1000.0*cfg.VolumeNoiseRange
	v := int(float64(oi) * noise)
	if v < 0 {
		v = 0
	}
	return v
}

// ── Bid / Ask spread ──────────────────────────────────────────────────────────

func bidAsk(mark float64, logMoneyness float64) (bid, ask float64) {
	return bidAskForConfig(mark, logMoneyness, defaultOptionsMarketProfile.ChainConfig)
}

func bidAskForConfig(mark float64, logMoneyness float64, cfg ChainConfig) (bid, ask float64) {
	spread := cfg.SpreadBase + cfg.SpreadSlope*math.Abs(logMoneyness)
	if spread > cfg.SpreadCap {
		spread = cfg.SpreadCap
	}
	bid = mark * (1 - spread)
	ask = mark * (1 + spread)
	if bid < 0.01 {
		bid = 0.01
	}
	if ask < bid+0.01 {
		ask = bid + 0.01
	}
	return
}

// ── Chain builder ─────────────────────────────────────────────────────────────

func round500(v float64) float64 {
	return roundToIncrement(v, defaultOptionsMarketProfile.ChainConfig.StrikeIncrement)
}

func roundToIncrement(v float64, increment float64) float64 {
	if increment <= 0 {
		return v
	}
	return math.Round(v/increment) * increment
}

// BuildChain computes the full option chain for a given spot price, expiry, and base IV.
func BuildChain(spot float64, expiry time.Time, baseIV float64) []ChainRow {
	atmStrike := round500(spot)
	dte := int(math.Max(1, math.Ceil(time.Until(expiry).Hours()/24)))

	const numStrikes = 20 // 20 above and 20 below ATM = 41 total
	const increment = 500.0

	var rows []ChainRow
	for i := -numStrikes; i <= numStrikes; i++ {
		strike := atmStrike + float64(i)*increment
		if strike <= 0 {
			continue
		}

		moneynessPC := (strike - spot) / spot * 100
		isATM := i == 0

		callIV := smileIV(baseIV, spot, strike, Call)
		putIV := smileIV(baseIV, spot, strike, Put)

		callRes := PriceOption(spot, strike, expiry, callIV, Call)
		putRes := PriceOption(spot, strike, expiry, putIV, Put)

		lm := math.Log(strike / spot)
		cBid, cAsk := bidAsk(callRes.Premium, lm)
		pBid, pAsk := bidAsk(putRes.Premium, lm)

		callOI := simulateOI(strike, spot, dte)
		putOI := simulateOI(strike+1, spot, dte) // offset seed so call≠put OI

		rows = append(rows, ChainRow{
			Strike:      strike,
			IsATM:       isATM,
			MoneynessPC: moneynessPC,
			Call: ChainLeg{
				IV:     callIV * 100,
				Delta:  callRes.Delta,
				Gamma:  callRes.Gamma,
				Theta:  callRes.Theta,
				Vega:   callRes.Vega,
				Mark:   callRes.Premium,
				Bid:    cBid,
				Ask:    cAsk,
				OI:     callOI,
				Volume: simulateVolume(callOI, strike),
				IsITM:  strike < spot,
			},
			Put: ChainLeg{
				IV:     putIV * 100,
				Delta:  putRes.Delta,
				Gamma:  putRes.Gamma,
				Theta:  putRes.Theta,
				Vega:   putRes.Vega,
				Mark:   putRes.Premium,
				Bid:    pBid,
				Ask:    pAsk,
				OI:     putOI,
				Volume: simulateVolume(putOI, strike+0.5),
				IsITM:  strike > spot,
			},
		})
	}
	return rows
}

// ── HTTP handler ──────────────────────────────────────────────────────────────

// HandleOptionChain serves GET /api/option-chain?expiry=<RFC3339>
func (e *Engine) HandleOptionChain(w http.ResponseWriter, r *http.Request) {
	setCORSOptions(w)
	if r.Method == http.MethodOptions {
		return
	}

	e.mu.RLock()
	spot := e.lastPrice
	minuteBarsCopy := append([]float64{}, e.minuteBars...)
	e.mu.RUnlock()

	profile := e.resolvedProfile()
	if spot <= 0 {
		spot = profile.ChainConfig.FallbackSpot
	}

	now := time.Now().UTC()
	expiries := generateExpiriesForConfig(now, profile.ChainConfig)
	baseIV := estimateIVWithBounds(minuteBarsCopy, profile.DefaultIV, profile.MinIV, profile.MaxIV)

	// Parse requested expiry (default to nearest)
	selectedValue := r.URL.Query().Get("expiry")
	selectedExpiry := expiries[0]
	if selectedValue != "" {
		for _, ex := range expiries {
			if ex.Value == selectedValue {
				selectedExpiry = ex
				break
			}
		}
	}

	expiryTime, err := time.Parse(time.RFC3339, selectedExpiry.Value)
	if err != nil {
		http.Error(w, "bad expiry", http.StatusBadRequest)
		return
	}

	chain := buildChainForProfile(spot, expiryTime, baseIV, profile)

	resp := ChainResponse{
		UnderlyingPrice: spot,
		BaseIV:          baseIV * 100,
		Expiries:        expiries,
		SelectedExpiry:  selectedExpiry.Value,
		ExpiryLabel:     selectedExpiry.Label,
		DTE:             selectedExpiry.DTE,
		Chain:           chain,
	}
	json.NewEncoder(w).Encode(resp)
}
