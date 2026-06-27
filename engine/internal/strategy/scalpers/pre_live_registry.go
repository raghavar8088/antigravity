package scalpers

// preLiveQualified is the set of 100 strategies that met all promotion criteria
// in backtesting (MinSharpe≥1.0, MinWinRate≥45%, MaxDrawdown≤20%,
// MinProfitFactor≥1.30, MinTrades≥50). Only these are loaded into the
// Pre-Live Trade Engine.
var preLiveQualified = map[string]bool{
	"WMA_ADX_Strong_Bear_Short":       true,
	"WMA9_EFI_ADX_Bear_Short":         true,
	"WMA9_Fisher_ADX_Short":           true,
	"WMA_CMF_Bear_Short":              true,
	"WMA_Bear_Cross_Short":            true,
	"WMA_EFI_Bear_Short":              true,
	"WMA9_CMF_ADX_Bear_Short":         true,
	"WMA9_CMF_Deep_EFI_Short":         true,
	"ATR_CMF_Expand_Bear_Short":       true,
	"EMA8_Cross_CMF_ADX_Short":        true,
	"Price_EMA21_Breakdown_Short":      true,
	"WMA_KST_Bear_Short":              true,
	"StochRSI_50_Cross_Bear_Short":    true,
	"WMA9_Aroon_Bear_Short":           true,
	"WR_CMF_Bear_Short":               true,
	"Fisher_CMF_Confirm_Bear_Short":   true,
	"Chandelier_Bear_Break_Short":     true,
	"WR_Mid_EFI_ADX_Short":            true,
	"WMA13_CMF_Bear_Short":            true,
	"RSI48_Break_CMF_Short":           true,
	"WMA9_Fisher_Aroon_Bear_Short":    true,
	"ADX_Strong_Trend_Bear_Short":     true,
	"MACD_Line_Zero_Cross_Short":      true,
	"WMA13_WMA34_CMF_Deep_Bear_Short": true,
	"MTF_CMF_Both_Neg_Short":          true,
	"Keltner_ADX_Bear_Short":          true,
	"WR_Mid_Cross_Bear_Short":         true,
	"Fisher_Zero_Cross_Short":         true,
	"WR_RSI_Bear_Confirm_Short":       true,
	"WMA13_WMA34_EFI_Bear_Short":      true,
	"Lower_High_CMF_Bear_Short":       true,
	"BB_Mid_Break_Short":              true,
	"EMA9_EFI_Bear_Short":             true,
	"Keltner_Mid_EFI_Bear_Short":      true,
	"Coppock_ADX_Bear_Short":          true,
	"OBV_Slope_Bear_Short":            true,
	"RSI_Mid_Cross_Long":              true,
	"Donchian_CMF_Bear_Short":         true,
	"MACD_Signal_Cross_Short":         true,
	"MTF_RSI_Cross_Confirm_Short":     true,
	"BB_Squeeze_Breakout_Short":       true,
	"WMA13_WMA34_Bear_Cross_Short":    true,
	"CMF_Extreme_Bear_Short":          true,
	"Bearish_Engulfing_Short":         true,
	"WMA_Fisher_Bear_Short":           true,
	"Bearish_Engulf_EFI_ADX_Short":    true,
	"Big_Bear_Candle_Short":           true,
	"EMA9_Cross_EMA21_Short":          true,
	"StochRSI_CMF_Bear_Short":         true,
	"MACD_Hist_Flip_ADX_Short":        true,
	"WMA13_WMA34_ADX_Bear_Short":      true,
	"Close_Low_Range_Bear_Short":      true,
	"MACD_Hist_Sign_Flip_Short":       true,
	"DEMA_CMF_Bear_Short":             true,
	"Close_Low_Range_ADX_Short":       true,
	"CMF_Cross_Bullish_Long":          true,
	"Dual_RSI_Oversold_Long":          true,
	"StochRSI_KD_OB_Short":            true,
	"BB_Squeeze_EFI_ADX_Short":        true,
	"Big_Bear_Candle_ADX_Short":       true,
	"Two_Bar_Bear_Reversal_Short":     true,
	"Stoch_Bear_Cross_Short":          true,
	"OBV_Slope_EFI_ADX_Short":         true,
	"OBV_Bear_CMF_Short":              true,
	"CMF_Deep_Bear_Short":             true,
	"Two_Bar_Bear_Reversal_ADX_Short": true,
	"Lower_High_Confirm_Short":        true,
	"EMA9_ADX_Strong_Bear_Short":      true,
	"Bearish_Momentum_EFI_ADX_Short":  true,
	"ZLEMA_CMF_Bear_Short":            true,
	"WMA5_CMF_Bear_Short":             true,
	"Keltner_CMF_Bear_Short":          true,
	"EMA8_Cross_EFI_ADX_Short":        true,
	"Fisher_Deep_Bear_Short":          true,
	"KST_Below_Zero_Bear_Short":       true,
	"ZLEMA_ADX_Bear_Short":            true,
	"ZLEMA_EFI_Bear_Short":            true,
	"HTF_Aligned_Pullback_Short":      true,
	"BB_Mid_EFI_Bear_Short":           true,
	"BB_Mid_ADX_Bear_Short":           true,
	"Bearish_Momentum_Candle_Short":   true,
	"HMA_Slope_Turn_Short":            true,
	"MACD_Signal_Cross_Long":          true,
	"CMF_Cross_Bearish_Short":         true,
	"DEMA_EFI_Bear_Short":             true,
	"EFI_Bullish_Cross_Long":          true,
	"ZLEMA_Fisher_Bear_Short":         true,
	"EMA8_Cross_EMA50_Short":          true,
	"KST_ADX_Bear_Short":              true,
	"Donchian_Mid_Break_Short":        true,
	"DEMA_ADX_Strong_Bear_Short":      true,
	"HMA_CMF_Bear_Short":              true,
	"Donchian_Breakdown_Short":        true,
	"BB_Width_Expand_Bear_Short":      true,
	"CMF_OBV_Confluence_Bear_Short":   true,
	"WilliamsR_Overbought_Short":      true,
	"Three_Bear_Candles_ADX_Short":    true,
	"Keltner_Lower_EFI_ADX_Short":     true,
	"Keltner_Lower_Break_Short":       true,
	"Three_Bear_Candles_Short":        true,
}

// BuildPreLiveStrategies returns the 100 backtested-qualified strategies for the
// Pre-Live Trade Engine. It pulls from all available strategy sources (ported +
// curated) and filters by the qualifying whitelist. No shadow overlay, no
// FilterWinnersOnly — the whitelist IS the quality gate.
func BuildPreLiveStrategies() []RegistryEntry {
	// Collect all candidate strategies from both sources.
	var all []RegistryEntry
	all = append(all, BuildPortedStrategies()...)
	all = append(all, BuildAllScalpers()...)
	all = append(all, buildExpansionPack()...)
	all = append(all, buildImmortalEditionPack()...)

	// Deduplicate by name (ported strategies may also appear in curated).
	seen := make(map[string]bool, len(all))
	var deduped []RegistryEntry
	for _, e := range all {
		name := e.Strategy.Name()
		if seen[name] {
			continue
		}
		seen[name] = true
		deduped = append(deduped, e)
	}

	// Filter to the qualifying 100.
	var result []RegistryEntry
	for _, e := range deduped {
		if preLiveQualified[e.Strategy.Name()] {
			result = append(result, e)
		}
	}
	return result
}
