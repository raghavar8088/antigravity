package strategy

import (
	"time"

	"antigravity-engine/internal/alpha"
	"antigravity-engine/internal/alpha/cvd"
	alphadelta "antigravity-engine/internal/alpha/delta"
	"antigravity-engine/internal/alpha/funding"
	"antigravity-engine/internal/alpha/fvg"
	"antigravity-engine/internal/alpha/liquidity"
	"antigravity-engine/internal/alpha/mss"
	"antigravity-engine/internal/alpha/orderblock"
	"antigravity-engine/internal/alpha/quality"
	"antigravity-engine/internal/alpha/session"
	"antigravity-engine/internal/alpha/volumeprofile"
	"antigravity-engine/internal/marketdata"
)

type alphaModule string

const (
	alphaCVD        alphaModule = "CVD"
	alphaDelta      alphaModule = "DELTA"
	alphaLiquidity  alphaModule = "LIQUIDITY"
	alphaFVG        alphaModule = "FVG"
	alphaOrderBlock alphaModule = "ORDER_BLOCK"
	alphaMSS        alphaModule = "MSS"
	alphaPOC        alphaModule = "POC"
	alphaSession    alphaModule = "SESSION"
	alphaConfluence alphaModule = "CONFLUENCE"
)

type InstitutionalAlphaScalper struct {
	baseScalper
	module alphaModule

	candles         []alpha.Candle
	cvdEngine       *cvd.Engine
	cvdCache        *cvd.Cache
	deltaEngine     *alphadelta.Engine
	liquidityEngine *liquidity.Engine
	fvgStrategy     *fvg.Strategy
	orderBlocks     *orderblock.Engine
	mssEngine       *mss.Engine
	sessionEngine   *session.Engine
	profileStrategy *volumeprofile.Strategy
	fundingCache    *funding.Cache
	fundingEngine   *funding.Engine
	fundingStrategy *funding.Strategy
}

func newInstitutionalAlphaScalper(name string, module alphaModule) *InstitutionalAlphaScalper {
	fundingCache, _ := funding.NewCache("data/alpha/funding.ndjson")
	fundingEngine := funding.NewEngine(fundingCache, nil)
	return &InstitutionalAlphaScalper{
		baseScalper:     baseScalper{name: name, maxBuf: defaultBufSize},
		module:          module,
		cvdEngine:       cvd.NewEngine(),
		cvdCache:        cvd.NewCache(2000),
		deltaEngine:     alphadelta.NewEngine(1000),
		liquidityEngine: liquidity.NewEngine(200),
		fvgStrategy:     fvg.NewStrategy(),
		orderBlocks:     orderblock.NewEngine(),
		mssEngine:       mss.NewEngine(),
		sessionEngine:   session.NewEngine(),
		profileStrategy: volumeprofile.NewStrategy(),
		fundingCache:    fundingCache,
		fundingEngine:   fundingEngine,
		fundingStrategy: funding.NewStrategy(fundingEngine),
	}
}

func NewFundingMeanReversionAlpha() *InstitutionalAlphaScalper {
	return newInstitutionalAlphaScalper("FundingMeanReversion_Alpha", alphaConfluence)
}

func NewCVDDivergenceAlpha() *InstitutionalAlphaScalper {
	return newInstitutionalAlphaScalper("CVDDivergence_Alpha", alphaCVD)
}

func NewDeltaAbsorptionAlpha() *InstitutionalAlphaScalper {
	return newInstitutionalAlphaScalper("DeltaAbsorption_Alpha", alphaDelta)
}

func NewLiquiditySweepReversalAlpha() *InstitutionalAlphaScalper {
	return newInstitutionalAlphaScalper("LiquiditySweepReversal_Alpha", alphaLiquidity)
}

func NewFVGRetestAlpha() *InstitutionalAlphaScalper {
	return newInstitutionalAlphaScalper("FVGRetest_Alpha", alphaFVG)
}

func NewOrderBlockRetestAlpha() *InstitutionalAlphaScalper {
	return newInstitutionalAlphaScalper("OrderBlockRetest_Alpha", alphaOrderBlock)
}

func NewMSSContinuationAlpha() *InstitutionalAlphaScalper {
	return newInstitutionalAlphaScalper("MSSContinuation_Alpha", alphaMSS)
}

func NewPOCBounceAlpha() *InstitutionalAlphaScalper {
	return newInstitutionalAlphaScalper("POCBounce_Alpha", alphaPOC)
}

func NewSessionExpansionAlpha() *InstitutionalAlphaScalper {
	return newInstitutionalAlphaScalper("SessionExpansion_Alpha", alphaSession)
}

func (s *InstitutionalAlphaScalper) OnTick(t marketdata.Tick) []Signal {
	row := alpha.Tick{Symbol: t.Symbol, Price: t.Price, Quantity: t.Quantity, Side: t.Side, Timestamp: unixMs(t.TimeMs)}
	td := s.cvdEngine.AddTick(row)
	s.cvdCache.Add(td)
	s.deltaEngine.Add(t.Price, td.Delta)
	if s.module == alphaCVD || s.module == alphaDelta || s.module == alphaConfluence {
		return s.evaluate(t.Symbol)
	}
	return holdSignal()
}

func (s *InstitutionalAlphaScalper) OnCandle(t marketdata.Tick) []Signal {
	s.feed(t.Price)
	c := s.toCandle(t)
	s.candles = append(s.candles, c)
	if len(s.candles) > defaultBufSize {
		s.candles = s.candles[len(s.candles)-defaultBufSize:]
	}
	return s.evaluate(t.Symbol)
}

func (s *InstitutionalAlphaScalper) evaluate(symbol string) []Signal {
	if len(s.prices) < 20 && len(s.candles) < 20 {
		return holdSignal()
	}
	var sig alpha.Signal
	switch s.module {
	case alphaCVD:
		sig = cvd.NewStrategy(s.cvdCache).Evaluate(symbol, s.prices)
	case alphaDelta:
		sig = alphadelta.NewStrategy(s.deltaEngine).Evaluate(symbol)
	case alphaLiquidity:
		event := s.liquidityEngine.Add(lastCandle(s.candles))
		if event.Direction != alpha.ActionHold {
			sig = alpha.Signal{Source: "LiquiditySweepReversal", Symbol: symbol, Action: event.Direction, Confidence: event.Confidence, Reason: "liquidity sweep reversal", StopLossPct: 0.30, TakeProfitPct: 0.85, Timestamp: time.Now().UTC()}
		}
	case alphaFVG:
		sig = s.fvgStrategy.Evaluate(s.candles)
	case alphaOrderBlock:
		sig = s.orderBlocks.Evaluate(s.candles)
	case alphaMSS:
		sig = s.mssEngine.Evaluate(s.candles)
	case alphaPOC:
		sig = s.profileStrategy.Evaluate(s.candles)
	case alphaSession:
		snap := s.sessionEngine.Analyze(s.candles)
		if snap.Expansion && snap.Bias != alpha.ActionHold {
			sig = alpha.Signal{Source: "SessionExpansion", Symbol: symbol, Action: snap.Bias, Confidence: alpha.Clamp(0.62+absFloat(snap.Momentum)/4, 0.62, 0.88), Reason: string(snap.Name) + " expansion bias", StopLossPct: 0.35, TakeProfitPct: 0.75, Timestamp: time.Now().UTC()}
		}
	case alphaConfluence:
		sig = s.confluence(symbol)
	}
	if sig.Action == "" || sig.Action == alpha.ActionHold {
		return holdSignal()
	}
	score := s.qualityFor(sig)
	if !quality.MandatoryPass(score.Score) {
		return holdSignal()
	}
	sig.QualityScore = score.Score
	return []Signal{{
		Symbol: sig.Symbol, Action: actionFromAlpha(sig.Action), TargetSize: defaultQty,
		Confidence: sig.Confidence, StopLossPct: sig.StopLossPct, TakeProfitPct: sig.TakeProfitPct,
		AIReasoning: sig.Reason,
	}}
}

func (s *InstitutionalAlphaScalper) confluence(symbol string) alpha.Signal {
	if s.fundingStrategy != nil && s.fundingEngine != nil && len(s.prices) >= 20 {
		exchangeSymbol := normalizeFundingSymbol(symbol)
		rows := s.fundingEngine.Metrics("binance", exchangeSymbol, 30*24*time.Hour)
		history := s.fundingEngineSnapshot("binance", exchangeSymbol)
		if len(history) > 0 {
			fundingSig := s.fundingStrategy.Evaluate(funding.StrategyInput{
				Symbol:        symbol,
				Exchange:      "binance",
				Closes:        s.prices,
				Support:       alpha.Lowest(alpha.Tail(s.prices, 40)),
				Resistance:    alpha.Highest(alpha.Tail(s.prices, 40)),
				LatestFunding: history[len(history)-1],
				Metrics:       rows,
			})
			if fundingSig.Action != alpha.ActionHold {
				return fundingSig
			}
		}
	}
	cvdSig := cvd.NewStrategy(s.cvdCache).Evaluate(symbol, s.prices)
	mssSig := s.mssEngine.Evaluate(s.candles)
	profileSig := s.profileStrategy.Evaluate(s.candles)
	votes := map[alpha.Action]int{}
	for _, sig := range []alpha.Signal{cvdSig, mssSig, profileSig} {
		if sig.Action != alpha.ActionHold {
			votes[sig.Action]++
		}
	}
	if votes[alpha.ActionBuy] >= 2 {
		return alpha.Signal{Source: "FundingMeanReversion", Symbol: symbol, Action: alpha.ActionBuy, Confidence: 0.78, Reason: "alpha confluence buy", StopLossPct: 0.35, TakeProfitPct: 0.85, Timestamp: time.Now().UTC()}
	}
	if votes[alpha.ActionSell] >= 2 {
		return alpha.Signal{Source: "FundingMeanReversion", Symbol: symbol, Action: alpha.ActionSell, Confidence: 0.78, Reason: "alpha confluence sell", StopLossPct: 0.35, TakeProfitPct: 0.85, Timestamp: time.Now().UTC()}
	}
	return alpha.Hold("FundingMeanReversion", symbol)
}

func (s *InstitutionalAlphaScalper) fundingEngineSnapshot(exchange, symbol string) []funding.FundingSnapshot {
	if s.fundingEngine == nil {
		return nil
	}
	cache := s.fundingEngineCache()
	if cache == nil {
		return nil
	}
	return cache.History(exchange, symbol, 30*24*time.Hour)
}

func (s *InstitutionalAlphaScalper) fundingEngineCache() *funding.Cache {
	return s.fundingCache
}

func normalizeFundingSymbol(symbol string) string {
	switch symbol {
	case "BTC-USD", "BTC-USDT":
		return "BTCUSDT"
	default:
		return symbol
	}
}

func (s *InstitutionalAlphaScalper) qualityFor(sig alpha.Signal) quality.Result {
	source := 1.0
	switch sig.Source {
	case "CVDDivergence":
		return quality.Score(quality.Inputs{CVD: source, Delta: 0.4, MSS: 0.4, Session: 0.5, VolumeProfile: 0.5, Liquidity: 0.5, Funding: 0.5, OrderBlock: 0.4, FVG: 0.4})
	case "LiquiditySweepReversal":
		return quality.Score(quality.Inputs{Liquidity: source, CVD: 0.7, Delta: 0.5, MSS: 0.6, VolumeProfile: 0.5, Session: 0.5, Funding: 0.4, OrderBlock: 0.5, FVG: 0.4})
	case "FVGRetest":
		return quality.Score(quality.Inputs{FVG: source, MSS: 0.7, Liquidity: 0.5, CVD: 0.5, Delta: 0.4, Session: 0.5, VolumeProfile: 0.5, Funding: 0.4, OrderBlock: 0.6})
	case "OrderBlockRetest":
		return quality.Score(quality.Inputs{OrderBlock: source, Liquidity: 0.6, FVG: 0.5, MSS: 0.7, CVD: 0.5, Delta: 0.4, VolumeProfile: 0.5, Session: 0.5, Funding: 0.4})
	case "MSSContinuation":
		return quality.Score(quality.Inputs{MSS: source, CVD: 0.6, Delta: 0.5, Session: 0.6, VolumeProfile: 0.5, Liquidity: 0.5, FVG: 0.5, OrderBlock: 0.4, Funding: 0.4})
	default:
		return quality.Score(quality.Inputs{Funding: 0.7, CVD: 0.7, Delta: 0.6, Liquidity: 0.6, OrderBlock: 0.5, FVG: 0.5, MSS: 0.7, VolumeProfile: 0.5, Session: 0.5})
	}
}

func (s *InstitutionalAlphaScalper) toCandle(t marketdata.Tick) alpha.Candle {
	ts := unixMs(t.TimeMs)
	open := t.Price
	if len(s.prices) >= 2 {
		open = s.prices[len(s.prices)-2]
	}
	high, low := open, t.Price
	if high < low {
		high, low = low, high
	}
	return alpha.Candle{Symbol: t.Symbol, Open: open, High: high, Low: low, Close: t.Price, Volume: t.Quantity, Timestamp: ts}
}

func lastCandle(candles []alpha.Candle) alpha.Candle {
	if len(candles) == 0 {
		return alpha.Candle{}
	}
	return candles[len(candles)-1]
}

func actionFromAlpha(action alpha.Action) Action {
	if action == alpha.ActionSell {
		return ActionSell
	}
	if action == alpha.ActionBuy {
		return ActionBuy
	}
	return ActionHold
}

func unixMs(ms int64) time.Time {
	if ms <= 0 {
		return time.Now().UTC()
	}
	return time.UnixMilli(ms).UTC()
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
