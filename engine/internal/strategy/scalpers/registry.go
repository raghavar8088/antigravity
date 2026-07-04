package scalpers

// BuildAllScalpers returns every strategy in the scalpers pack as a RegistryEntry.
// Used for backtesting, monitoring dashboards, and the curated selector.
func BuildAllScalpers() []RegistryEntry {
	entries := []RegistryEntry{
		{
			Strategy:        &EMARibbonTrendRider{},
			Name:            "EMA_Ribbon_Trend_Rider",
			Description:     "1h ribbon alignment + 15m pullback entry with CVD confirmation",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"15m", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy:        &BollingerMeanReversion{},
			Name:            "Bollinger_Mean_Reversion",
			Description:     "Compressed BB on 15m, fade at 5m band extremes with RSI + order book wall",
			Regimes:         []Regime{RegimeRanging},
			Timeframes:      []string{"5m", "15m"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy:        &LiquiditySweepReversal{},
			Name:            "Liquidity_Sweep_Reversal",
			Description:     "Stop-hunt spike through swing level closes back inside; CVD + funding spike",
			Regimes:         []Regime{RegimeVolatile},
			Timeframes:      []string{"1m", "5m"},
			MaxPositions:    1,
			OHLCVCompatible: false, // requires live CVD
		},
		{
			Strategy:        &ADXMomentumBreakout{},
			Name:            "ADX_Momentum_Breakout",
			Description:     "4h ADX>25 confirms trend; 15m breakout with volume surge + MACD histogram",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"15m", "4h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy:        &VWAPInstitutionalFade{},
			Name:            "VWAP_Institutional_Fade",
			Description:     "Price extends >1.5×ATR from session VWAP; fade back; London/NY only",
			Regimes:         []Regime{RegimeRanging},
			Timeframes:      []string{"5m", "15m"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy:        &CVDDivergenceSniper{},
			Name:            "CVD_Divergence_Sniper",
			Description:     "Price-CVD divergence on 5m; fires in all regimes except UNKNOWN",
			Regimes:         []Regime{RegimeTrending, RegimeRanging, RegimeVolatile},
			Timeframes:      []string{"5m", "15m"},
			MaxPositions:    1,
			OHLCVCompatible: false, // requires live CVD feed
		},
		{
			Strategy:        &OpeningRangeBreakout{},
			Name:            "Opening_Range_Breakout",
			Description:     "NY session ORB (14:00-14:30 UTC range), breakout with volume + 1h bias",
			Regimes:         []Regime{RegimeTrending, RegimeRanging, RegimeVolatile},
			Timeframes:      []string{"5m", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy:        &FundingRateFade{},
			Name:            "Funding_Rate_Fade",
			Description:     "Fade extreme funding rates: short when >0.05%, long when <-0.05%; requires 2 consecutive readings",
			Regimes:         []Regime{RegimeTrending, RegimeRanging},
			Timeframes:      []string{"1h"},
			MaxPositions:    1,
			OHLCVCompatible: true, // funding rates downloaded
		},
		{
			Strategy:        &OIDivergence{},
			Name:            "OI_Divergence",
			Description:     "OI rising+price falling=bearish; OI falling+price rising=bullish/exhaustion",
			Regimes:         []Regime{RegimeTrending, RegimeRanging},
			Timeframes:      []string{"15m", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: false, // requires historical OI (unavailable >30 days on Binance)
		},
	}
	entries = append(entries, buildNewStrategiesBatch10()...)
	return entries
}

// buildPortedStrategiesBase returns RegistryEntry wrappers for the base ported strategies (S101–S150).
// All enter in shadow mode via withShadowOverride() applied in BuildCuratedScalpers.
func buildPortedStrategiesBase() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &ZLEMAMomentumScalp{}, Name: "ZLEMA_Momentum_Scalp", Description: "ZLEMA cross on 1m with CVD + RSI", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1m", "5m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HullMAScalp{}, Name: "Hull_MA_Scalp", Description: "HMA(9/16) cross on 5m + OB imbalance", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"5m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires order book
		{Strategy: &WilliamsRHammer{}, Name: "WilliamsR_Hammer", Description: "WilliamsR oversold + hammer + MACD hist", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"5m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FisherTransformSignal{}, Name: "Fisher_Transform_Signal", Description: "Fisher extreme + CVD", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"5m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &StochRSICross{}, Name: "StochRSI_Cross", Description: "StochRSI K/D cross from extreme + volume", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"5m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &VolatilitySqueezeEntry{}, Name: "Volatility_Squeeze_Entry", Description: "BB squeeze + expansion bar", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &NarrowRangeBreakout{}, Name: "Narrow_Range_Breakout", Description: "NR7 + breakout + volume surge", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CMFBBTouch{}, Name: "CMF_BB_Touch", Description: "CMF bull + BB lower touch", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &AggressorExhaustion{}, Name: "Aggressor_Exhaustion", Description: "Buy aggressor exhaustion + BB upper short", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1m", "5m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires tick aggressor data
		{Strategy: &TradeCountSpike{}, Name: "Trade_Count_Spike", Description: "Trade count >3x avg + CVD direction", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1m"}, MaxPositions: 1, OHLCVCompatible: false},  // requires trade count feed
		{Strategy: &EffectiveSpreadCompress{}, Name: "Effective_Spread_Compress", Description: "Spread compressed + MACD direction", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"5m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires live spread
		{Strategy: &LastTradeSizeWhale{}, Name: "Last_Trade_Size_Whale", Description: "Whale print + CVD + OB imbalance", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"5m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires tick data + CVD
		{Strategy: &MultiTFZScoreLong{}, Name: "Multi_TF_ZScore_Long", Description: "Z-score extreme low on 5m/15m/1h", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"5m", "15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MultiTFZScoreShort{}, Name: "Multi_TF_ZScore_Short", Description: "Z-score extreme high on 5m/15m/1h", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"5m", "15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &SupertrendRider{}, Name: "Supertrend_Rider", Description: "Supertrend direction aligned 5m+1h", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"5m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ChandelierFollow{}, Name: "Chandelier_Follow", Description: "Chandelier Exit with ADX + 4h bias", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m", "4h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &PSARTrend{}, Name: "PSAR_Trend", Description: "PSAR direction + ADX + CVD", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DEMAPullback{}, Name: "DEMA_Pullback", Description: "DEMA(21/55) bull + price at DEMA21", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KeltnerBreakout{}, Name: "Keltner_Breakout", Description: "Price breaks Keltner channel + ADX", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &PivotLevelBreak{}, Name: "Pivot_Level_Break", Description: "4h pivot R1/S1 break + vol surge + CVD", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m", "4h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EFIMomentum{}, Name: "EFI_Momentum", Description: "Elder Force Index + ADX + CVD on 15m", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DonchianSqueeze{}, Name: "Donchian_Squeeze", Description: "Donchian(20) squeeze vs Donchian(55) breakout", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OBVDivergence1h{}, Name: "OBV_Divergence_1h", Description: "OBV/price divergence over 10 bars on 1h", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WilliamsRSwing{}, Name: "WilliamsR_Swing", Description: "WilliamsR extreme on 1h+4h swing", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h", "4h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FourHMomentumBurst{}, Name: "4H_Momentum_Burst", Description: "4h MACD burst + ADX25 + CVD", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &IchimokuCloudBreak{}, Name: "Ichimoku_Cloud_Break", Description: "Price breaks Ichimoku cloud on 4h", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HMASlopeShift{}, Name: "HMA_Slope_Shift", Description: "HMA(20) slope direction change + CVD", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &StochRSI1hReversal{}, Name: "StochRSI_1h_Reversal", Description: "StochRSI extreme K/D cross on 1h", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ZLEMA1hCross{}, Name: "ZLEMA_1h_Cross", Description: "ZLEMA(21/55) cross on 1h + CVD", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MultiTFSupertrendCascade{}, Name: "Multi_TF_Supertrend_Cascade", Description: "Supertrend aligned 15m+1h+4h", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m", "1h", "4h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FundingRateExtremeLong{}, Name: "Funding_Rate_Extreme_Long", Description: "Extreme negative funding mean reversion long", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: true},  // funding rates downloaded
		{Strategy: &FundingRateExtremeShort{}, Name: "Funding_Rate_Extreme_Short", Description: "Extreme positive funding mean reversion short", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: true}, // funding rates downloaded
		{Strategy: &OISpikeFollow{}, Name: "OI_Spike_Follow", Description: "OI spike >2% + CVD direction follow", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires live OI
		{Strategy: &OIUnwindFade{}, Name: "OI_Unwind_Fade", Description: "OI unwind >3% + BB extreme fade", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires live OI
		{Strategy: &LiquidationCascadeReversal{}, Name: "Liquidation_Cascade_Reversal", Description: "Liq cascade spike at BB extreme reversal", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"5m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires liquidation feed
		{Strategy: &DVOLExtremeRegime{}, Name: "DVOL_Extreme_Regime", Description: "DVOL extreme fear/complacency mean rev", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: false}, // requires DVOL index
		{Strategy: &FundingOIConfirm{}, Name: "Funding_OI_Confirm", Description: "Funding + OI + ADX triple confirmation", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: false}, // requires live OI
		{Strategy: &OptionIVCrush{}, Name: "Option_IV_Crush", Description: "DVOL drop >5% + CVD directional follow", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: false}, // requires DVOL + CVD
		{Strategy: &OptionIVExpansion{}, Name: "Option_IV_Expansion", Description: "DVOL rise >5% + CVD breakout follow", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires DVOL + CVD
		{Strategy: &ETHCorrelationDivergence{}, Name: "ETH_Correlation_Divergence", Description: "ETH/BTC correlation divergence signal", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires ETH feed
		{Strategy: &SentimentTrendCombo{}, Name: "Sentiment_Trend_Combo", Description: "Funding+OI sentiment + ADX trend combo", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h"}, MaxPositions: 1, OHLCVCompatible: false}, // requires live OI
		{Strategy: &DVOLFundingMacroLong{}, Name: "DVOL_Funding_Macro_Long", Description: "High DVOL + neg funding long mean-rev", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: false}, // requires DVOL
		{Strategy: &DVOLFundingMacroShort{}, Name: "DVOL_Funding_Macro_Short", Description: "Low DVOL + high funding short", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: false}, // requires DVOL
		{Strategy: &ETHStrengthBTCLag{}, Name: "ETH_Strength_BTC_Lag", Description: "ETH strength proxy with BTC EMA bull bias", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1h"}, MaxPositions: 1, OHLCVCompatible: false}, // requires ETH feed
		{Strategy: &LiquidationHuntLong{}, Name: "Liquidation_Hunt_Long", Description: "Short liq squeeze + CVD bull momentum", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"5m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires liquidation + CVD
		{Strategy: &LiquidationHuntShort{}, Name: "Liquidation_Hunt_Short", Description: "Long liq flush + CVD bear momentum", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"5m"}, MaxPositions: 1, OHLCVCompatible: false}, // requires liquidation + CVD
		{Strategy: &OITrendFundingFilter{}, Name: "OI_Trend_Funding_Filter", Description: "OI trend + funding sign + ADX filter", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h"}, MaxPositions: 1, OHLCVCompatible: false}, // requires live OI
		{Strategy: &MultiSignalConfluenceLong{}, Name: "Multi_Signal_Confluence_Long", Description: "4-of-5 indicator confluence long", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MultiSignalConfluenceShort{}, Name: "Multi_Signal_Confluence_Short", Description: "4-of-5 indicator confluence short", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &VolatilityRegimeShift{}, Name: "Volatility_Regime_Shift", Description: "ATR expansion regime shift + BB direction", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},

		// ── High-Win-Rate family (HW1–HW10) ─────────────────────────────────────
		{Strategy: &HTFAlignedPullbackLong{}, Name: "HTF_Aligned_Pullback_Long", Description: "4h+1h uptrend + 15m RSI pullback entry long", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h", "15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HTFAlignedPullbackShort{}, Name: "HTF_Aligned_Pullback_Short", Description: "4h+1h downtrend + 15m RSI pullback entry short", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h", "15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBSqueezeBreakoutLong{}, Name: "BB_Squeeze_Breakout_Long", Description: "BB squeeze + upper break + MACD + volume long", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBSqueezeBreakoutShort{}, Name: "BB_Squeeze_Breakout_Short", Description: "BB squeeze + lower break + MACD + volume short", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DualRSIOversoldLong{}, Name: "Dual_RSI_Oversold_Long", Description: "1h+15m RSI both oversold, 15m recovering long", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DualRSIOverboughtShort{}, Name: "Dual_RSI_Overbought_Short", Description: "1h+15m RSI both overbought, 15m fading short", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MACDZeroCrossTrendLong{}, Name: "MACD_Zero_Cross_Trend_Long", Description: "15m MACD zero-cross in 4h uptrend long", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m", "1h", "4h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MACDZeroCrossTrendShort{}, Name: "MACD_Zero_Cross_Trend_Short", Description: "15m MACD zero-cross in 4h downtrend short", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"15m", "1h", "4h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FundingExtremeContrarianLong{}, Name: "Funding_Extreme_Contrarian_Long", Description: "Extreme neg funding + 1h recovery long (short squeeze)", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FundingExtremeContrarianShort{}, Name: "Funding_Extreme_Contrarian_Short", Description: "Extreme pos funding + 1h exhaustion short (long liquidation)", Regimes: []Regime{RegimeTrending, RegimeRanging}, Timeframes: []string{"15m", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}

// BuildPortedStrategies combines base ported strategies with HW v2–v9 families.
func BuildPortedStrategies() []RegistryEntry {
	base := buildPortedStrategiesBase()
	base = append(base, BuildHWV2Strategies()...)
	base = append(base, BuildHWV3Strategies()...)
	base = append(base, BuildHWV4Strategies()...)
	base = append(base, BuildHWV5Strategies()...)
	base = append(base, BuildHWV6Strategies()...)
	base = append(base, BuildHWV7Strategies()...)
	base = append(base, BuildHWV8Strategies()...)
	base = append(base, BuildHWV9Strategies()...)
	base = append(base, BuildHWV10Strategies()...)
	base = append(base, BuildHWV11Strategies()...)
	base = append(base, BuildHWV12Strategies()...)
	base = append(base, BuildHWV13Strategies()...)
	return base
}
