package microstructure

import (
	"math"
	"sort"
	"strings"
	"time"

	"antigravity-engine/internal/alpha"
)

const defaultWindow = 240

type Engine struct {
	window int

	ticks        []alpha.Tick
	tickDeltas   []float64
	candles      []alpha.Candle
	orderBooks   []OrderBookSnapshot
	funding      []FundingSnapshot
	liquidations []LiquidationEvent
}

func NewEngine(window int) *Engine {
	if window <= 0 {
		window = defaultWindow
	}
	return &Engine{window: window}
}

func (e *Engine) AddTick(t alpha.Tick) FeatureSnapshot {
	if t.Timestamp.IsZero() {
		t.Timestamp = time.Now().UTC()
	}
	delta := e.classifyDelta(t)
	e.ticks = appendBounded(e.ticks, t, e.window)
	e.tickDeltas = appendBoundedFloat(e.tickDeltas, delta, e.window)
	return e.Snapshot(t.Symbol)
}

func (e *Engine) AddCandle(c alpha.Candle) FeatureSnapshot {
	if c.Timestamp.IsZero() {
		c.Timestamp = time.Now().UTC()
	}
	e.candles = appendBounded(e.candles, c, e.window)
	return e.Snapshot(c.Symbol)
}

func (e *Engine) AddOrderBook(ob OrderBookSnapshot) FeatureSnapshot {
	if ob.Timestamp.IsZero() {
		ob.Timestamp = time.Now().UTC()
	}
	e.orderBooks = appendBounded(e.orderBooks, ob, min(e.window, 120))
	return e.Snapshot(ob.Symbol)
}

func (e *Engine) AddFunding(f FundingSnapshot) FeatureSnapshot {
	if f.Timestamp.IsZero() {
		f.Timestamp = time.Now().UTC()
	}
	e.funding = appendBounded(e.funding, f, min(e.window, 120))
	return e.Snapshot(f.Symbol)
}

func (e *Engine) AddLiquidation(liq LiquidationEvent) FeatureSnapshot {
	if liq.Timestamp.IsZero() {
		liq.Timestamp = time.Now().UTC()
	}
	e.liquidations = appendBounded(e.liquidations, liq, min(e.window, 240))
	return e.Snapshot(liq.Symbol)
}

func (e *Engine) Snapshot(symbol string) FeatureSnapshot {
	now := time.Now().UTC()
	out := FeatureSnapshot{Symbol: symbol, Timestamp: now, Regime: RegimeRanging, VolatilityRegime: RegimeRanging}
	if len(e.ticks) > 0 {
		t := e.ticks[len(e.ticks)-1]
		out.LastPrice = t.Price
		out.Timestamp = t.Timestamp
		out.Symbol = nonEmpty(symbol, t.Symbol)
	}
	if len(e.candles) > 0 {
		c := e.candles[len(e.candles)-1]
		out.LastPrice = choosePositive(out.LastPrice, c.Close)
		out.Timestamp = c.Timestamp
		out.Symbol = nonEmpty(out.Symbol, c.Symbol)
	}

	out.AggressorBuyVolume, out.AggressorSellVolume = e.aggressorVolumes()
	out.PassiveVolume = math.Max(0, (out.AggressorBuyVolume+out.AggressorSellVolume)*0.35)
	out.RollingCVD = sum(e.tickDeltas)
	out.CVDMomentum = sum(tailFloat(e.tickDeltas, 30)) - sum(tailFloat(e.tickDeltas, 90))
	out.CVDConfirmationScore = clamp01(0.5 + math.Tanh(out.CVDMomentum/max(1, out.AggressorBuyVolume+out.AggressorSellVolume))*0.5)
	out.BullishCVDDivergence, out.BearishCVDDivergence = e.cvdDivergence()

	out.ATR, out.ATRPct = atr(e.candles, 14)
	out.Regime = classifyRegime(e.candles, out.ATRPct)
	if out.Regime == RegimeHighVol {
		out.VolatilityRegime = RegimeHighVol
	} else {
		out.VolatilityRegime = out.Regime
	}

	out.BidAskImbalance = e.bidAskImbalance()
	out.LiquidityWalls, out.LiquidityGaps = e.liquidityZones()
	out.LiquidityZoneProximityScore = proximityScore(out.LastPrice, append(out.LiquidityWalls, out.LiquidityGaps...))
	out.LiquidityConfirmation = out.LiquidityZoneProximityScore >= 0.45 || math.Abs(out.BidAskImbalance) >= 0.25
	out.SwingHigh, out.SwingLow = swingBounds(e.candles, 30)
	out.SweepDirection, out.SweepRejection, out.VolumeSpike = e.detectSweep(out.SwingHigh, out.SwingLow)

	out.FundingRate, out.OpenInterestDelta = e.fundingState()
	out.FundingPressureScore = fundingPressure(out.FundingRate)

	out.LastLiquidationSide, out.LiquidationNotional, out.LiquidationSpike, out.LiquidationExhaustion = e.liquidationState()

	out.BOSDirection, out.CHOCHDirection, out.StructureRetest = e.marketStructure(out.SwingHigh, out.SwingLow)
	out.MarketStructureAlignmentScore = structureScore(out.BOSDirection, out.CHOCHDirection, out.StructureRetest)
	out.FairValueGaps = detectFVGs(e.candles)
	out.OrderBlocks = detectOrderBlocks(e.candles, out.ATR)
	out.VolumeProfile = buildVolumeProfile(e.candles, 48)
	return out
}

func (e *Engine) classifyDelta(t alpha.Tick) float64 {
	side := strings.ToUpper(strings.TrimSpace(t.Side))
	qty := math.Abs(t.Quantity)
	switch side {
	case "BUY", "BID_LIFT", "AGGRESSOR_BUY", "LONG":
		return qty
	case "SELL", "ASK_HIT", "AGGRESSOR_SELL", "SHORT":
		return -qty
	}
	if len(e.ticks) == 0 || t.Price >= e.ticks[len(e.ticks)-1].Price {
		return qty
	}
	return -qty
}

func (e *Engine) aggressorVolumes() (float64, float64) {
	buy, sell := 0.0, 0.0
	for _, d := range e.tickDeltas {
		if d >= 0 {
			buy += d
		} else {
			sell += -d
		}
	}
	return buy, sell
}

func (e *Engine) bidAskImbalance() float64 {
	if len(e.orderBooks) == 0 {
		return 0
	}
	ob := e.orderBooks[len(e.orderBooks)-1]
	bid, ask := depth(ob.Bids, 10), depth(ob.Asks, 10)
	if bid+ask == 0 {
		return 0
	}
	return (bid - ask) / (bid + ask)
}

func (e *Engine) liquidityZones() ([]LiquidityZone, []LiquidityZone) {
	if len(e.orderBooks) == 0 {
		return nil, nil
	}
	ob := e.orderBooks[len(e.orderBooks)-1]
	walls := detectWalls("bid", ob.Bids)
	walls = append(walls, detectWalls("ask", ob.Asks)...)
	gaps := detectGaps("bid", ob.Bids)
	gaps = append(gaps, detectGaps("ask", ob.Asks)...)
	return walls, gaps
}

func (e *Engine) detectSweep(swingHigh, swingLow float64) (alpha.Action, bool, bool) {
	if len(e.candles) < 12 || swingHigh <= 0 || swingLow <= 0 {
		return alpha.ActionHold, false, false
	}
	last := e.candles[len(e.candles)-1]
	avgVol := averageVolumes(e.candles[:len(e.candles)-1], 20)
	spike := avgVol > 0 && last.Volume >= avgVol*1.45
	rejectHigh := last.High > swingHigh && last.Close < swingHigh
	rejectLow := last.Low < swingLow && last.Close > swingLow
	if rejectHigh && spike {
		return alpha.ActionSell, true, true
	}
	if rejectLow && spike {
		return alpha.ActionBuy, true, true
	}
	return alpha.ActionHold, rejectHigh || rejectLow, spike
}

func (e *Engine) fundingState() (float64, float64) {
	if len(e.funding) == 0 {
		return 0, 0
	}
	last := e.funding[len(e.funding)-1]
	if len(e.funding) < 2 {
		return last.Rate, 0
	}
	prev := e.funding[len(e.funding)-2]
	return last.Rate, last.OpenInterest - prev.OpenInterest
}

func (e *Engine) liquidationState() (string, float64, bool, bool) {
	if len(e.liquidations) == 0 {
		return "", 0, false, false
	}
	recent := tailLiquidations(e.liquidations, 20)
	notionals := make([]float64, 0, len(recent))
	total := 0.0
	side := recent[len(recent)-1].Side
	for _, liq := range recent {
		n := math.Abs(liq.Price * liq.Quantity)
		notionals = append(notionals, n)
		total += n
	}
	threshold := quantile(notionals, 0.80) * 2
	spike := threshold > 0 && total >= threshold
	exhaustion := spike && len(e.candles) >= 3 && candleBodyPct(e.candles[len(e.candles)-1]) < candleBodyPct(e.candles[len(e.candles)-2])
	return side, total, spike, exhaustion
}

func (e *Engine) marketStructure(swingHigh, swingLow float64) (alpha.Action, alpha.Action, bool) {
	if len(e.candles) < 25 || swingHigh <= 0 || swingLow <= 0 {
		return alpha.ActionHold, alpha.ActionHold, false
	}
	last := e.candles[len(e.candles)-1]
	prev := e.candles[len(e.candles)-2]
	bos := alpha.ActionHold
	if last.Close > swingHigh {
		bos = alpha.ActionBuy
	} else if last.Close < swingLow {
		bos = alpha.ActionSell
	}
	choch := alpha.ActionHold
	priorTrend := last.Close - e.candles[len(e.candles)-20].Close
	if priorTrend < 0 && last.Close > swingHigh {
		choch = alpha.ActionBuy
	}
	if priorTrend > 0 && last.Close < swingLow {
		choch = alpha.ActionSell
	}
	retest := (prev.Close > swingHigh && last.Low <= swingHigh && last.Close >= swingHigh) || (prev.Close < swingLow && last.High >= swingLow && last.Close <= swingLow)
	return bos, choch, retest
}

func classifyRegime(candles []alpha.Candle, atrPct float64) Regime {
	if len(candles) < 30 {
		return RegimeRanging
	}
	if atrPct >= 0.75 {
		return RegimeHighVol
	}
	last := candles[len(candles)-1].Close
	first := candles[len(candles)-30].Close
	movePct := (last - first) / max(1, first) * 100
	if movePct > math.Max(0.35, atrPct*1.4) {
		return RegimeTrendingBull
	}
	if movePct < -math.Max(0.35, atrPct*1.4) {
		return RegimeTrendingBear
	}
	return RegimeRanging
}

func detectFVGs(candles []alpha.Candle) []FairValueGap {
	if len(candles) < 3 {
		return nil
	}
	out := make([]FairValueGap, 0)
	start := maxInt(2, len(candles)-40)
	for i := start; i < len(candles); i++ {
		c1, c3 := candles[i-2], candles[i]
		if c1.High < c3.Low {
			out = append(out, FairValueGap{Direction: alpha.ActionBuy, Low: c1.High, High: c3.Low, CreatedAt: c3.Timestamp})
		}
		if c1.Low > c3.High {
			out = append(out, FairValueGap{Direction: alpha.ActionSell, Low: c3.High, High: c1.Low, CreatedAt: c3.Timestamp})
		}
	}
	return out
}

func detectOrderBlocks(candles []alpha.Candle, atrValue float64) []OrderBlock {
	if len(candles) < 5 {
		return nil
	}
	if atrValue <= 0 {
		atrValue, _ = atr(candles, 14)
	}
	out := make([]OrderBlock, 0)
	start := maxInt(3, len(candles)-50)
	for i := start; i < len(candles); i++ {
		last := candles[i]
		prior := candles[i-1]
		impulse := math.Abs(last.Close-prior.Open) >= math.Max(atrValue*1.2, prior.Close*0.002)
		if !impulse {
			continue
		}
		if prior.Close < prior.Open && last.Close > prior.High {
			out = append(out, OrderBlock{Direction: alpha.ActionBuy, Low: prior.Low, High: prior.High, Strength: math.Abs(last.Close-prior.Open) / max(1, atrValue), CreatedAt: last.Timestamp})
		}
		if prior.Close > prior.Open && last.Close < prior.Low {
			out = append(out, OrderBlock{Direction: alpha.ActionSell, Low: prior.Low, High: prior.High, Strength: math.Abs(last.Close-prior.Open) / max(1, atrValue), CreatedAt: last.Timestamp})
		}
	}
	return out
}

func buildVolumeProfile(candles []alpha.Candle, buckets int) VolumeProfile {
	if len(candles) == 0 || buckets <= 0 {
		return VolumeProfile{}
	}
	tail := tailCandles(candles, 120)
	lo, hi := tail[0].Low, tail[0].High
	for _, c := range tail {
		lo = math.Min(lo, c.Low)
		hi = math.Max(hi, c.High)
	}
	if hi <= lo {
		return VolumeProfile{POC: hi, Low: lo, High: hi}
	}
	vols := make([]float64, buckets)
	step := (hi - lo) / float64(buckets)
	for _, c := range tail {
		idx := int((c.Close - lo) / step)
		if idx < 0 {
			idx = 0
		}
		if idx >= buckets {
			idx = buckets - 1
		}
		vols[idx] += c.Volume
	}
	maxVol := 0.0
	pocIdx := 0
	for i, v := range vols {
		if v > maxVol {
			maxVol = v
			pocIdx = i
		}
	}
	profile := VolumeProfile{POC: lo + (float64(pocIdx)+0.5)*step, Low: lo, High: hi}
	for i, v := range vols {
		price := lo + (float64(i)+0.5)*step
		if maxVol > 0 && v >= maxVol*0.70 {
			profile.HVN = append(profile.HVN, price)
		}
		if maxVol > 0 && v <= maxVol*0.25 {
			profile.LVN = append(profile.LVN, price)
		}
	}
	return profile
}

func (e *Engine) cvdDivergence() (bool, bool) {
	if len(e.candles) < 30 || len(e.tickDeltas) < 30 {
		return false, false
	}
	recent := tailCandles(e.candles, 30)
	mid := len(recent) / 2
	leftHigh, leftLow := candleHighLow(recent[:mid])
	rightHigh, rightLow := candleHighLow(recent[mid:])
	leftCVD := sum(e.tickDeltas[maxInt(0, len(e.tickDeltas)-30):maxInt(0, len(e.tickDeltas)-15)])
	rightCVD := sum(tailFloat(e.tickDeltas, 15))
	bullish := rightLow < leftLow && rightCVD > leftCVD
	bearish := rightHigh > leftHigh && rightCVD < leftCVD
	return bullish, bearish
}

func atr(candles []alpha.Candle, period int) (float64, float64) {
	if len(candles) < 2 {
		return 0, 0
	}
	start := maxInt(1, len(candles)-period)
	values := make([]float64, 0, len(candles)-start)
	for i := start; i < len(candles); i++ {
		c, prev := candles[i], candles[i-1]
		tr := math.Max(c.High-c.Low, math.Max(math.Abs(c.High-prev.Close), math.Abs(c.Low-prev.Close)))
		values = append(values, tr)
	}
	v := average(values)
	price := candles[len(candles)-1].Close
	return v, v / max(1, price) * 100
}

func detectWalls(side string, levels []OrderBookLevel) []LiquidityZone {
	if len(levels) == 0 {
		return nil
	}
	sizes := make([]float64, 0, len(levels))
	for _, l := range levels {
		sizes = append(sizes, l.Size)
	}
	avg := average(sizes)
	out := make([]LiquidityZone, 0)
	for _, l := range levels {
		if avg > 0 && l.Size >= avg*2.5 {
			width := l.Price * 0.00025
			out = append(out, LiquidityZone{Price: l.Price, Size: l.Size, Side: side, Strength: clamp01(l.Size / (avg * 5)), LowerBound: l.Price - width, UpperBound: l.Price + width})
		}
	}
	return out
}

func detectGaps(side string, levels []OrderBookLevel) []LiquidityZone {
	if len(levels) < 3 {
		return nil
	}
	sortedLevels := append([]OrderBookLevel(nil), levels...)
	sort.Slice(sortedLevels, func(i, j int) bool { return sortedLevels[i].Price < sortedLevels[j].Price })
	gaps := make([]float64, 0, len(sortedLevels)-1)
	for i := 1; i < len(sortedLevels); i++ {
		gaps = append(gaps, math.Abs(sortedLevels[i].Price-sortedLevels[i-1].Price))
	}
	med := quantile(gaps, 0.50)
	out := make([]LiquidityZone, 0)
	for i := 1; i < len(sortedLevels); i++ {
		gap := math.Abs(sortedLevels[i].Price - sortedLevels[i-1].Price)
		if med > 0 && gap >= med*2.5 {
			lo, hi := sortedLevels[i-1].Price, sortedLevels[i].Price
			out = append(out, LiquidityZone{Price: (lo + hi) / 2, Side: side + "_gap", Strength: clamp01(gap / (med * 5)), LowerBound: lo, UpperBound: hi})
		}
	}
	return out
}

func proximityScore(price float64, zones []LiquidityZone) float64 {
	if price <= 0 || len(zones) == 0 {
		return 0
	}
	best := 0.0
	for _, z := range zones {
		if price >= z.LowerBound && price <= z.UpperBound {
			best = math.Max(best, 1)
			continue
		}
		dist := math.Abs(price-z.Price) / price
		best = math.Max(best, clamp01(1-dist/0.006)*math.Max(0.4, z.Strength))
	}
	return best
}

func swingBounds(candles []alpha.Candle, period int) (float64, float64) {
	if len(candles) < 3 {
		return 0, 0
	}
	window := tailCandles(candles, period)
	high, low := window[0].High, window[0].Low
	for _, c := range window[:len(window)-1] {
		high = math.Max(high, c.High)
		low = math.Min(low, c.Low)
	}
	return high, low
}

func fundingPressure(rate float64) float64 {
	if rate >= 0.001 {
		return clamp01(rate / 0.002)
	}
	if rate <= -0.0005 {
		return clamp01(math.Abs(rate) / 0.001)
	}
	return clamp01(math.Abs(rate) / 0.001)
}

func structureScore(bos, choch alpha.Action, retest bool) float64 {
	score := 0.25
	if bos != alpha.ActionHold {
		score += 0.25
	}
	if choch != alpha.ActionHold {
		score += 0.30
	}
	if retest {
		score += 0.20
	}
	return clamp01(score)
}

func depth(levels []OrderBookLevel, n int) float64 {
	total := 0.0
	for i, l := range levels {
		if i >= n {
			break
		}
		total += l.Size
	}
	return total
}

func averageVolumes(candles []alpha.Candle, n int) float64 {
	tail := tailCandles(candles, n)
	values := make([]float64, 0, len(tail))
	for _, c := range tail {
		values = append(values, c.Volume)
	}
	return average(values)
}

func candleBodyPct(c alpha.Candle) float64 {
	return math.Abs(c.Close-c.Open) / max(1, c.Close) * 100
}

func candleHighLow(candles []alpha.Candle) (float64, float64) {
	if len(candles) == 0 {
		return 0, 0
	}
	hi, lo := candles[0].High, candles[0].Low
	for _, c := range candles[1:] {
		hi = math.Max(hi, c.High)
		lo = math.Min(lo, c.Low)
	}
	return hi, lo
}

func tailCandles(values []alpha.Candle, n int) []alpha.Candle {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}

func tailFloat(values []float64, n int) []float64 {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}

func tailLiquidations(values []LiquidationEvent, n int) []LiquidationEvent {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return sum(values) / float64(len(values))
}

func sum(values []float64) float64 {
	out := 0.0
	for _, v := range values {
		out += v
	}
	return out
}

func quantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	idx := int(math.Round(clamp01(q) * float64(len(cp)-1)))
	return cp[idx]
}

func appendBounded[T any](values []T, value T, maxLen int) []T {
	values = append(values, value)
	if maxLen > 0 && len(values) > maxLen {
		return values[len(values)-maxLen:]
	}
	return values
}

func appendBoundedFloat(values []float64, value float64, maxLen int) []float64 {
	values = append(values, value)
	if maxLen > 0 && len(values) > maxLen {
		return values[len(values)-maxLen:]
	}
	return values
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func choosePositive(a, b float64) float64 {
	if a > 0 {
		return a
	}
	return b
}
