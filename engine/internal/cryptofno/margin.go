// Package cryptofno is the crypto F&O paper-trading desk: named accounts with
// their own capital, multi-leg basket execution against the live Delta option
// chain, and portfolio margin that gives credit for hedges.
//
// # Why margin is computed here rather than asked for
//
// Delta Exchange has no basket/portfolio margin endpoint — POST /v2/orders/margins
// returns 404, and the per-product fields (initial_margin, scaling factors) price
// ONE contract in isolation. Margining each leg independently is what makes a
// hedged book look as risky as a naked one: a short strangle and an iron condor
// containing that same strangle would reserve the same capital, even though the
// condor's loss is capped by its wings and the strangle's is not.
//
// So this engine does what an exchange risk system does: revalue the WHOLE basket
// across a grid of adverse underlying moves and reserve the worst outcome. That
// single change is what produces the behaviour asked for — sell a call and a put
// and the requirement is large; buy wings against them and it collapses toward
// the defined max loss.
package cryptofno

import (
	"math"
	"sort"
	"time"
)

// Side of a leg.
type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

// OptionType of a leg. Futures/perps use TypeFuture.
type OptionType string

const (
	TypeCall   OptionType = "CALL"
	TypePut    OptionType = "PUT"
	TypeFuture OptionType = "FUTURE"
)

// Leg is one contract in a basket.
type Leg struct {
	Symbol    string     `json:"symbol"`
	ProductID int        `json:"productId"`
	Type      OptionType `json:"type"`
	Side      Side       `json:"side"`
	Strike    float64    `json:"strike"`
	Expiry    time.Time  `json:"expiry"`
	Lots      int        `json:"lots"`
	// PremiumPerBTC is the quoted premium in USD per unit of underlying, exactly
	// as Delta quotes it. Multiply by ContractValue for the USD cost of one lot.
	PremiumPerBTC float64 `json:"premiumPerBtc"`
	// IV is the implied volatility used to revalue this leg under stress. Taken
	// from the chain; a leg with no IV is revalued on intrinsic value alone,
	// which is conservative for a short and understates a long.
	IV float64 `json:"iv"`
	// ContractValue is the underlying per contract (0.001 BTC on Delta options).
	ContractValue float64 `json:"contractValue"`
}

// signedLots is +ve for long exposure, -ve for short.
func (l Leg) signedLots() float64 {
	n := float64(l.Lots)
	if l.Side == SideSell {
		return -n
	}
	return n
}

// PremiumUSD is what one lot costs (long) or credits (short), in USD.
func (l Leg) PremiumUSD() float64 {
	return l.PremiumPerBTC * l.ContractValue * float64(l.Lots)
}

// MarginParams mirrors the fields Delta publishes per product. Defaults match
// what the live BTC option chain returns.
type MarginParams struct {
	// InitialMarginPct is Delta's `initial_margin`, a percentage of notional.
	InitialMarginPct float64
	// ScenarioRangePct is how far the underlying is stressed in each direction.
	// Exchange SPAN-style systems scan a band around spot; anything narrower
	// than the market's real daily range under-reserves.
	ScenarioRangePct float64
	// ScenarioSteps is the number of points scanned across the range. More steps
	// find sharper worst cases around strikes, where option payoffs kink.
	ScenarioSteps int
	// VolShockPct widens IV in the stressed revaluation. A short option loses on
	// BOTH an adverse move and rising vol, and ignoring the second is how a
	// short-vol book looks safe right up until it is not.
	VolShockPct float64
	// MinShortMarginPct floors the requirement on a short leg as a share of
	// notional, so a far-OTM short that the scenario grid barely touches still
	// reserves something. Naked short options have unbounded loss; zero margin
	// on one is never correct.
	MinShortMarginPct float64
}

// DefaultMarginParams is calibrated against Delta's published product fields
// (initial_margin = 1% on BTC options) and a scan band wide enough to cover a
// realistic adverse day in crypto, which moves far more than an index.
var DefaultMarginParams = MarginParams{
	InitialMarginPct:  0.01,
	ScenarioRangePct:  0.20,
	ScenarioSteps:     81,
	VolShockPct:       0.25,
	MinShortMarginPct: 0.005,
}

// MarginResult explains a requirement rather than just stating it, because an
// unexplained number is one a user cannot sanity-check against their broker.
type MarginResult struct {
	// RequiredUSD is what the account must have free to hold this basket.
	RequiredUSD float64 `json:"requiredUsd"`
	// WorstCaseLossUSD is the largest portfolio loss found across the scan.
	WorstCaseLossUSD float64 `json:"worstCaseLossUsd"`
	// WorstCaseSpot is the underlying price that produced it.
	WorstCaseSpot float64 `json:"worstCaseSpot"`
	// NetPremiumUSD is negative when the basket is a net debit (paid), positive
	// when a net credit (received).
	NetPremiumUSD float64 `json:"netPremiumUsd"`
	// LongPremiumUSD is what the long legs cost. A long-only basket can never
	// lose more than this, and the requirement is capped accordingly.
	LongPremiumUSD float64 `json:"longPremiumUsd"`
	// ExposureUSD is the notional-based component.
	ExposureUSD float64 `json:"exposureUsd"`
	// HedgeCreditUSD is how much the hedges saved versus margining every leg
	// standalone. This is the number that makes the benefit visible.
	HedgeCreditUSD float64 `json:"hedgeCreditUsd"`
	// StandaloneUSD is the sum of per-leg requirements with no netting.
	StandaloneUSD float64 `json:"standaloneUsd"`
	// Basis names which rule bound the answer.
	Basis string `json:"basis"`
}

// PortfolioMargin computes the requirement for a whole basket at once.
//
// The scan is the point: legs are revalued TOGETHER at each stressed spot, so a
// long wing's gain offsets the short's loss in the same scenario. Margining leg
// by leg cannot see that, which is why per-leg margining charges an iron condor
// like a naked strangle.
func PortfolioMargin(legs []Leg, spot float64, p MarginParams) MarginResult {
	if len(legs) == 0 || spot <= 0 {
		return MarginResult{Basis: "empty basket"}
	}
	if p.ScenarioSteps <= 1 {
		p = DefaultMarginParams
	}

	res := MarginResult{}

	var netPremium, longPremium float64
	hasShort := false
	// Exposure is charged on NET SHORT lots per option type, not on gross legs.
	//
	// Summing every leg's notional counts the long hedge as if it added risk,
	// which is backwards: it removes it. That error alone made a defined-risk
	// vertical spread reserve ~3x its own maximum loss, because the bought leg
	// contributed exposure instead of cancelling the sold one.
	netLots := map[OptionType]float64{}
	for _, l := range legs {
		prem := l.PremiumUSD()
		if l.Side == SideBuy {
			netPremium -= prem // debit
			longPremium += prem
		} else {
			netPremium += prem // credit
			hasShort = true
		}
		netLots[l.Type] += l.signedLots()
	}
	res.NetPremiumUSD = netPremium
	res.LongPremiumUSD = longPremium

	// Only lots left short after offsetting carry exposure. Calls and puts are
	// counted separately and both charged: a short strangle is short gamma on
	// both wings, so the two do not cancel each other even though their
	// directional exposures point opposite ways.
	cv := legs[0].ContractValue
	shortNotional := 0.0
	for _, net := range netLots {
		if net < 0 {
			shortNotional += -net * spot * cv
		}
	}
	res.ExposureUSD = shortNotional * p.InitialMarginPct

	// A basket with no short leg cannot lose more than the premium paid. No
	// scenario scan can beat that bound, and reserving more would block trades
	// that carry no tail risk at all.
	if !hasShort {
		res.RequiredUSD = longPremium
		res.WorstCaseLossUSD = longPremium
		res.StandaloneUSD = longPremium
		res.Basis = "long premium only — max loss is the debit paid"
		return res
	}

	// Value the basket now, then at each stressed spot. The worst delta between
	// them is the loss the account must be able to absorb.
	base := valueBasket(legs, spot, p.VolShockPct*0, time.Now())
	worstLoss, worstSpot := 0.0, spot

	lo := spot * (1 - p.ScenarioRangePct)
	hi := spot * (1 + p.ScenarioRangePct)
	step := (hi - lo) / float64(p.ScenarioSteps-1)

	for i := 0; i < p.ScenarioSteps; i++ {
		s := lo + step*float64(i)
		// Vol is shocked UP in every scenario: a short option is hurt by both an
		// adverse move and rising vol, and they arrive together in practice.
		v := valueBasket(legs, s, p.VolShockPct, time.Now())
		if loss := base - v; loss > worstLoss {
			worstLoss, worstSpot = loss, s
		}
	}
	res.WorstCaseLossUSD = worstLoss
	res.WorstCaseSpot = worstSpot

	// Requirement is the scanned loss plus the exposure component, floored so a
	// far-OTM short still reserves something.
	required := worstLoss + res.ExposureUSD
	floor := 0.0
	for _, l := range legs {
		if l.Side == SideSell {
			floor += spot * l.ContractValue * float64(l.Lots) * p.MinShortMarginPct
		}
	}
	if required < floor {
		required = floor
		res.Basis = "short-leg floor"
	} else {
		res.Basis = "worst-case portfolio loss + exposure"
	}
	res.RequiredUSD = required

	// Standalone comparison makes the hedge credit explicit.
	res.StandaloneUSD = standaloneMargin(legs, spot, p)
	if c := res.StandaloneUSD - res.RequiredUSD; c > 0 {
		res.HedgeCreditUSD = c
	}
	return res
}

// valueBasket returns the basket's mark-to-market value at a given spot, with IV
// shocked upward by volShock. Long legs contribute positively, shorts negatively.
func valueBasket(legs []Leg, spot, volShock float64, now time.Time) float64 {
	total := 0.0
	for _, l := range legs {
		total += l.signedLots() * l.ContractValue * legValuePerUnit(l, spot, volShock, now)
	}
	return total
}

// legValuePerUnit prices one unit of underlying for a leg at the stressed spot.
func legValuePerUnit(l Leg, spot, volShock float64, now time.Time) float64 {
	switch l.Type {
	case TypeFuture:
		return spot
	case TypeCall, TypePut:
		t := yearsTo(l.Expiry, now)
		iv := l.IV * (1 + volShock)
		if iv <= 0 || t <= 0 {
			// No usable vol or already expired: fall back to intrinsic. This is
			// deliberately conservative for a short (it cannot understate a deep
			// ITM obligation) even though it ignores remaining time value.
			return intrinsic(l.Type, spot, l.Strike)
		}
		return blackScholes(l.Type, spot, l.Strike, t, iv)
	default:
		return 0
	}
}

func intrinsic(t OptionType, spot, strike float64) float64 {
	if t == TypeCall {
		return math.Max(spot-strike, 0)
	}
	return math.Max(strike-spot, 0)
}

func yearsTo(expiry time.Time, now time.Time) float64 {
	if expiry.IsZero() {
		return 0
	}
	d := expiry.Sub(now).Hours() / 24 / 365
	if d < 0 {
		return 0
	}
	return d
}

// blackScholes prices a European option. Rate is 0: crypto options settle in the
// quote asset and the carry is already in the forward the chain quotes, so
// adding a rate here would double-count it.
func blackScholes(t OptionType, spot, strike, years, iv float64) float64 {
	if years <= 0 || iv <= 0 || spot <= 0 || strike <= 0 {
		return intrinsic(t, spot, strike)
	}
	sqrtT := math.Sqrt(years)
	d1 := (math.Log(spot/strike) + 0.5*iv*iv*years) / (iv * sqrtT)
	d2 := d1 - iv*sqrtT
	if t == TypeCall {
		return spot*normCDF(d1) - strike*normCDF(d2)
	}
	return strike*normCDF(-d2) - spot*normCDF(-d1)
}

func normCDF(x float64) float64 { return 0.5 * math.Erfc(-x/math.Sqrt2) }

// standaloneMargin is what the basket would cost if every leg were margined on
// its own — the behaviour this engine exists to replace. Kept so the hedge
// credit can be shown rather than asserted.
func standaloneMargin(legs []Leg, spot float64, p MarginParams) float64 {
	total := 0.0
	for _, l := range legs {
		one := []Leg{l}
		if l.Side == SideBuy {
			total += l.PremiumUSD()
			continue
		}
		r := shortLegMargin(one[0], spot, p)
		total += r
	}
	return total
}

// shortLegMargin prices a single naked short by the same scan, so standalone and
// portfolio numbers are produced by one method and are genuinely comparable.
func shortLegMargin(l Leg, spot float64, p MarginParams) float64 {
	legs := []Leg{l}
	base := valueBasket(legs, spot, 0, time.Now())
	worst := 0.0
	lo := spot * (1 - p.ScenarioRangePct)
	hi := spot * (1 + p.ScenarioRangePct)
	step := (hi - lo) / float64(p.ScenarioSteps-1)
	for i := 0; i < p.ScenarioSteps; i++ {
		s := lo + step*float64(i)
		if loss := base - valueBasket(legs, s, p.VolShockPct, time.Now()); loss > worst {
			worst = loss
		}
	}
	expo := spot * l.ContractValue * float64(l.Lots) * p.InitialMarginPct
	floor := spot * l.ContractValue * float64(l.Lots) * p.MinShortMarginPct
	if v := worst + expo; v > floor {
		return v
	}
	return floor
}

// GroupBaskets splits positions into independently-margined groups.
//
// Netting is only legitimate within one underlying: a BTC short is not hedged by
// an ETH long, and treating them as one book would hand out a credit the market
// will not honour. Expiry is deliberately NOT part of the key — a calendar
// spread is a real hedge, and splitting by expiry would deny it.
func GroupBaskets(legs []Leg, underlyingOf func(Leg) string) map[string][]Leg {
	out := map[string][]Leg{}
	for _, l := range legs {
		out[underlyingOf(l)] = append(out[underlyingOf(l)], l)
	}
	for k := range out {
		sort.SliceStable(out[k], func(i, j int) bool { return out[k][i].Symbol < out[k][j].Symbol })
	}
	return out
}
