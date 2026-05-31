package microstructure

import (
	"time"

	"antigravity-engine/internal/alpha"
)

type Regime string

const (
	RegimeTrendingBull Regime = "TRENDING_BULL"
	RegimeTrendingBear Regime = "TRENDING_BEAR"
	RegimeRanging      Regime = "RANGING"
	RegimeHighVol      Regime = "HIGH_VOL"
)

type AlphaType string

const (
	AlphaLiquidity     AlphaType = "LIQUIDITY"
	AlphaTrend         AlphaType = "TREND"
	AlphaMeanReversion AlphaType = "MEAN_REVERSION"
	AlphaFunding       AlphaType = "FUNDING"
	AlphaLiquidation   AlphaType = "LIQUIDATION"
)

type StrategyKind string

const (
	StrategyLiquiditySweep       StrategyKind = "LIQUIDITY_SWEEP_REVERSAL"
	StrategyFundingMeanReversion StrategyKind = "FUNDING_RATE_MEAN_REVERSION"
	StrategyCVDDivergence        StrategyKind = "CVD_DIVERGENCE"
	StrategyLiquidationCascade   StrategyKind = "LIQUIDATION_CASCADE_REVERSAL"
	StrategyFVGContinuation      StrategyKind = "FAIR_VALUE_GAP_CONTINUATION"
	StrategyOrderBlockRetest     StrategyKind = "ORDER_BLOCK_RETEST"
	StrategyMSSRetest            StrategyKind = "MSS_CHOCH_RETEST"
)

type OrderBookLevel struct {
	Price float64
	Size  float64
}

type OrderBookSnapshot struct {
	Symbol    string
	Bids      []OrderBookLevel
	Asks      []OrderBookLevel
	Timestamp time.Time
}

type FundingSnapshot struct {
	Symbol       string
	Rate        float64
	OpenInterest float64
	Timestamp   time.Time
}

type LiquidationEvent struct {
	Symbol    string
	Side      string
	Price     float64
	Quantity  float64
	Timestamp time.Time
}

type LiquidityZone struct {
	Price      float64
	Size       float64
	Side       string
	Strength   float64
	LowerBound float64
	UpperBound float64
}

type FairValueGap struct {
	Direction alpha.Action
	Low       float64
	High      float64
	Filled    bool
	CreatedAt time.Time
}

type OrderBlock struct {
	Direction alpha.Action
	Low       float64
	High      float64
	Strength  float64
	CreatedAt time.Time
}

type VolumeProfile struct {
	POC  float64
	HVN  []float64
	LVN  []float64
	Low  float64
	High float64
}

type FeatureSnapshot struct {
	Symbol    string
	Timestamp time.Time

	LastPrice    float64
	ATR          float64
	ATRPct       float64
	Regime       Regime
	VolatilityRegime Regime

	AggressorBuyVolume  float64
	AggressorSellVolume float64
	PassiveVolume       float64
	BidAskImbalance     float64
	RollingCVD          float64
	CVDMomentum         float64
	CVDConfirmationScore float64
	BearishCVDDivergence bool
	BullishCVDDivergence bool

	LiquidityWalls []LiquidityZone
	LiquidityGaps  []LiquidityZone
	LiquidityZoneProximityScore float64
	LiquidityConfirmation       bool
	SweepDirection alpha.Action
	SweepRejection bool
	VolumeSpike    bool

	FundingRate       float64
	OpenInterestDelta float64
	FundingPressureScore float64

	LastLiquidationSide string
	LiquidationNotional float64
	LiquidationSpike    bool
	LiquidationExhaustion bool

	SwingHigh float64
	SwingLow  float64
	BOSDirection   alpha.Action
	CHOCHDirection alpha.Action
	StructureRetest bool
	MarketStructureAlignmentScore float64

	FairValueGaps []FairValueGap
	OrderBlocks   []OrderBlock
	VolumeProfile VolumeProfile
}

type EnrichedSignal struct {
	Signal alpha.Signal
	Kind   StrategyKind
	AlphaType AlphaType
	SignalCluster string

	CVDConfirmationScore         float64
	LiquidityZoneProximityScore  float64
	FundingPressureScore         float64
	MarketStructureAlignmentScore float64
	VolatilityRegime             Regime
	RegimePass                   bool
	LiquidityConfirmation         bool
	FinalConfidence              float64
	Approved                     bool
	RejectReason                 string
}

type StrategyScoreInput struct {
	WinRate              float64
	ProfitFactor         float64
	Sharpe               float64
	CVDConfirmation      float64
	LiquidityEdgeScore   float64
	FundingEdgeScore     float64
	DrawdownPenalty      float64
}

type StrategyScore struct {
	Score      float64
	Promotable bool
}
