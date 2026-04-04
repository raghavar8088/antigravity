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
	IV     float64 `json:"iv"`     // Annualised IV as percent (e.g. 75.3)
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

// ── IV smile model ────────────────────────────────────────────────────────────

// smileIV returns the implied volatility for a given strike, applying a
// realistic volatility smile + negative skew (puts priced higher than calls).
//
//	baseIV  – ATM IV (annualised fraction, e.g. 0.75)
//	spot    – current BTC price
//	strike  – option strike
//	optType – Call or Put
func smileIV(baseIV, spot, strike float64, optType OptionType) float64 {
	m := math.Log(strike / spot) // log-moneyness: 0=ATM, >0=OTM call, <0=OTM put

	// Quadratic smile with negative skew
	const smile = 2.5  // curvature
	const skew = 0.25  // tilt: puts carry extra premium

	iv := baseIV * math.Exp(smile*m*m-skew*m)

	// Floor and cap to realistic BTC bounds
	if iv < 0.30 {
		iv = 0.30
	}
	if iv > 3.50 {
		iv = 3.50
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
	dist := math.Abs(math.Log(strike/spot)) // log-moneyness magnitude
	base := 8000.0 * math.Exp(-10.0*dist) * math.Sqrt(float64(dte+1))
	noise := 0.70 + float64(pseudoSeed(strike))/1000.0*0.60
	v := int(base * noise)
	if v < 0 {
		v = 0
	}
	return v
}

func simulateVolume(oi int, strike float64) int {
	noise := 0.05 + float64(pseudoSeed(strike+1))/1000.0*0.25
	v := int(float64(oi) * noise)
	if v < 0 {
		v = 0
	}
	return v
}

// ── Bid / Ask spread ──────────────────────────────────────────────────────────

func bidAsk(mark float64, logMoneyness float64) (bid, ask float64) {
	spread := 0.012 + 0.12*math.Abs(logMoneyness) // 1.2% ATM, widens OTM
	if spread > 0.15 {
		spread = 0.15
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
	return math.Round(v/500) * 500
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
	priceHist := append([]float64{}, e.priceHist...)
	e.mu.RUnlock()

	if spot <= 0 {
		spot = 67000 // fallback until first tick arrives
	}

	now := time.Now().UTC()
	expiries := generateExpiries(now)
	baseIV := EstimateIV(priceHist) // annualised fraction

	// Parse requested expiry (default to nearest)
	selectedValue := r.URL.Query().Get("expiry")
	var selectedExpiry ExpiryMeta
	if selectedValue == "" {
		selectedExpiry = expiries[0]
	} else {
		for _, ex := range expiries {
			if ex.Value == selectedValue {
				selectedExpiry = ex
				break
			}
		}
		if selectedExpiry.Value == "" {
			selectedExpiry = expiries[0]
		}
	}

	expiryTime, err := time.Parse(time.RFC3339, selectedExpiry.Value)
	if err != nil {
		http.Error(w, "bad expiry", http.StatusBadRequest)
		return
	}

	chain := BuildChain(spot, expiryTime, baseIV)

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
