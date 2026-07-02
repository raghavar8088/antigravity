package scalpers

// preLiveQualified is the set of strategies that met all promotion criteria
// in backtesting. Only these are loaded into the Pre-Live Trade Engine.
//
// ─── Whitelist last rebuilt: 2026-07-02 ──────────────────────────────────────
// Backtest period : 2023-01-01 → 2024-12-31
// Total strategies tested : 283 (OHLCV-compatible; 23 skipped, require live feeds)
// Passing strict criteria : 42  (VALIDATED)
// Passing relaxed criteria: 0   (all qualified strategies met strict thresholds)
// Excluded                : 241
//
// Promotion thresholds used (engine/internal/backtest/promotion.go):
//   MinSharpe       ≥ 1.0
//   MinWinRate      ≥ 45%
//   MinProfitFactor ≥ 1.30
//   MinTrades       ≥ 50
//   MaxDrawdown     ≤ 20%
//
// Top-3 performing strategies by Sharpe:
//   1. ZLEMA_ADX_Bear_Short      sharpe=76.19 wr=91.0% pf=3.26 trades=67  dd=0.01%
//   2. MACD_Line_Zero_Cross_Short sharpe=69.85 wr=86.3% pf=3.01 trades=51  dd=0.03%
//   3. ZLEMA_EFI_Bear_Short       sharpe=61.39 wr=90.9% pf=2.69 trades=55  dd=0.02%
var preLiveQualified = map[string]bool{
	// ── Rank  14 ──────────────────────────────────────────────────────────────
	"ZLEMA_ADX_Bear_Short": true, // sharpe=76.19 wr=91.0% pf=3.26 trades=67  dd=0.01%
	// ── Rank  16 ──────────────────────────────────────────────────────────────
	"MACD_Line_Zero_Cross_Short": true, // sharpe=69.85 wr=86.3% pf=3.01 trades=51  dd=0.03%
	// ── Rank  21 ──────────────────────────────────────────────────────────────
	"ZLEMA_EFI_Bear_Short": true, // sharpe=61.39 wr=90.9% pf=2.69 trades=55  dd=0.02%
	// ── Rank  23 ──────────────────────────────────────────────────────────────
	"ADX_Strong_Trend_Bear_Short": true, // sharpe=60.25 wr=86.4% pf=2.67 trades=66  dd=0.02%
	// ── Rank  24 ──────────────────────────────────────────────────────────────
	"ZLEMA_Fisher_Bear_Short": true, // sharpe=60.01 wr=91.3% pf=2.65 trades=69  dd=0.02%
	// ── Rank  25 ──────────────────────────────────────────────────────────────
	"CMF_OBV_Confluence_Bear_Short": true, // sharpe=56.85 wr=86.4% pf=2.26 trades=66  dd=0.01%
	// ── Rank  38 ──────────────────────────────────────────────────────────────
	"Big_Bear_Candle_Short": true, // sharpe=48.05 wr=85.7% pf=2.16 trades=84  dd=0.03%
	// ── Rank  41 ──────────────────────────────────────────────────────────────
	"Donchian_Breakdown_Short": true, // sharpe=45.94 wr=87.5% pf=2.09 trades=168 dd=0.04%
	// ── Rank  42 ──────────────────────────────────────────────────────────────
	"Big_Bear_Candle_ADX_Short": true, // sharpe=45.32 wr=85.5% pf=2.08 trades=83  dd=0.03%
	// ── Rank  44 ──────────────────────────────────────────────────────────────
	"Donchian_CMF_Bear_Short": true, // sharpe=44.11 wr=85.6% pf=2.01 trades=139 dd=0.04%
	// ── Rank  46 ──────────────────────────────────────────────────────────────
	"Chandelier_Bear_Break_Short": true, // sharpe=40.05 wr=86.4% pf=1.99 trades=59  dd=0.02%
	// ── Rank  47 ──────────────────────────────────────────────────────────────
	"Dual_RSI_Oversold_Long": true, // sharpe=38.40 wr=83.5% pf=2.11 trades=91  dd=0.04%
	// ── Rank  48 ──────────────────────────────────────────────────────────────
	"OBV_Bear_CMF_Short": true, // sharpe=37.74 wr=85.3% pf=1.84 trades=129 dd=0.04%
	// ── Rank  49 ──────────────────────────────────────────────────────────────
	"Three_Bear_Candles_ADX_Short": true, // sharpe=36.82 wr=87.1% pf=1.83 trades=171 dd=0.04%
	// ── Rank  51 ──────────────────────────────────────────────────────────────
	"KST_Below_Zero_Bear_Short": true, // sharpe=35.48 wr=76.5% pf=1.74 trades=85  dd=0.05%
	// ── Rank  52 ──────────────────────────────────────────────────────────────
	"ZLEMA_CMF_Bear_Short": true, // sharpe=35.39 wr=88.7% pf=1.83 trades=71  dd=0.04%
	// ── Rank  53 ──────────────────────────────────────────────────────────────
	"RSI_Mid_Cross_Long": true, // sharpe=35.21 wr=83.3% pf=1.88 trades=60  dd=0.03%
	// ── Rank  54 ──────────────────────────────────────────────────────────────
	"ZLEMA_Bear_Cross_Short": true, // sharpe=35.05 wr=82.7% pf=1.67 trades=127 dd=0.06%
	// ── Rank  56 ──────────────────────────────────────────────────────────────
	"EMA8_Cross_EFI_ADX_Short": true, // sharpe=34.10 wr=86.3% pf=1.76 trades=73  dd=0.03%
	// ── Rank  57 ──────────────────────────────────────────────────────────────
	"HTF_Aligned_Pullback_Long": true, // sharpe=32.57 wr=83.9% pf=1.76 trades=56  dd=0.04%
	// ── Rank  58 ──────────────────────────────────────────────────────────────
	"Three_Bear_Candles_Short": true, // sharpe=32.37 wr=87.0% pf=1.71 trades=177 dd=0.04%
	// ── Rank  59 ──────────────────────────────────────────────────────────────
	"OBV_Slope_EFI_ADX_Short": true, // sharpe=31.85 wr=83.4% pf=1.64 trades=199 dd=0.07%
	// ── Rank  60 ──────────────────────────────────────────────────────────────
	"EMA8_Cross_EMA50_Short": true, // sharpe=30.65 wr=74.5% pf=1.60 trades=55  dd=0.04%
	// ── Rank  61 ──────────────────────────────────────────────────────────────
	"CMF_Extreme_Bear_Short": true, // sharpe=29.96 wr=83.5% pf=1.63 trades=91  dd=0.06%
	// ── Rank  62 ──────────────────────────────────────────────────────────────
	"WMA13_WMA34_Bear_Cross_Short": true, // sharpe=29.26 wr=80.3% pf=1.54 trades=61  dd=0.04%
	// ── Rank  63 ──────────────────────────────────────────────────────────────
	"WMA13_WMA34_EFI_Bear_Short": true, // sharpe=28.88 wr=80.0% pf=1.53 trades=60  dd=0.04%
	// ── Rank  64 ──────────────────────────────────────────────────────────────
	"BB_Squeeze_EFI_ADX_Short": true, // sharpe=27.96 wr=85.6% pf=1.61 trades=202 dd=0.05%
	// ── Rank  67 ──────────────────────────────────────────────────────────────
	"BB_Squeeze_Breakout_Short": true, // sharpe=25.04 wr=84.8% pf=1.52 trades=59  dd=0.04%
	// ── Rank  70 ──────────────────────────────────────────────────────────────
	"Coppock_ADX_Bear_Short": true, // sharpe=24.38 wr=70.4% pf=1.47 trades=54  dd=0.08%
	// ── Rank  71 ──────────────────────────────────────────────────────────────
	"Close_Low_Range_Bear_Short": true, // sharpe=24.02 wr=85.9% pf=1.53 trades=191 dd=0.08%
	// ── Rank  72 ──────────────────────────────────────────────────────────────
	"WMA13_WMA34_ADX_Bear_Short": true, // sharpe=23.95 wr=78.6% pf=1.42 trades=56  dd=0.04%
	// ── Rank  74 ──────────────────────────────────────────────────────────────
	"Squeeze_Fired_Long": true, // sharpe=23.62 wr=79.6% pf=1.48 trades=108 dd=0.09%
	// ── Rank  75 ──────────────────────────────────────────────────────────────
	"CMF_Cross_Bearish_Short": true, // sharpe=23.08 wr=82.9% pf=1.45 trades=105 dd=0.05%
	// ── Rank  76 ──────────────────────────────────────────────────────────────
	"BB_Width_Expand_Bear_Short": true, // sharpe=22.52 wr=83.6% pf=1.48 trades=298 dd=0.10%
	// ── Rank  80 ──────────────────────────────────────────────────────────────
	"HMA_CMF_Bear_Short": true, // sharpe=20.82 wr=79.8% pf=1.40 trades=268 dd=0.07%
	// ── Rank  81 ──────────────────────────────────────────────────────────────
	"Close_Low_Range_ADX_Short": true, // sharpe=20.72 wr=83.7% pf=1.42 trades=239 dd=0.09%
	// ── Rank  85 ──────────────────────────────────────────────────────────────
	"Lower_High_CMF_Bear_Short": true, // sharpe=18.81 wr=81.6% pf=1.36 trades=196 dd=0.10%
	// ── Rank  86 ──────────────────────────────────────────────────────────────
	"EFI_Bullish_Cross_Long": true, // sharpe=18.67 wr=81.0% pf=1.37 trades=168 dd=0.07%
	// ── Rank  87 ──────────────────────────────────────────────────────────────
	"Aroon_Bear_Strong_Short": true, // sharpe=17.97 wr=71.0% pf=1.30 trades=69  dd=0.07%
	// ── Rank  88 ──────────────────────────────────────────────────────────────
	"ADX_Surge_Breakout_Long": true, // sharpe=17.13 wr=80.0% pf=1.33 trades=120 dd=0.05%
	// ── Rank  89 ──────────────────────────────────────────────────────────────
	"BB_Upper_Rejection_Short": true, // sharpe=16.98 wr=76.9% pf=1.41 trades=65  dd=0.06%
	// ── Rank  91 ──────────────────────────────────────────────────────────────
	"Fisher_Deep_Bear_Short": true, // sharpe=16.06 wr=83.2% pf=1.30 trades=214 dd=0.11%
}

// PreLiveWhitelistSize returns the number of names in the preLiveQualified
// whitelist. Used at startup to detect silent strategy-name mismatches between
// the whitelist and the strategy builder functions.
func PreLiveWhitelistSize() int {
	return len(preLiveQualified)
}

// BuildPreLiveStrategies returns the backtested-qualified strategies for the
// Pre-Live Trade Engine. It pulls from all available strategy sources (ported +
// curated + research) and filters by the qualifying whitelist. No shadow overlay,
// no FilterWinnersOnly — the whitelist IS the quality gate.
func BuildPreLiveStrategies() []RegistryEntry {
	// Collect all candidate strategies from both sources.
	var all []RegistryEntry
	all = append(all, BuildPortedStrategies()...)
	all = append(all, BuildAllScalpers()...)
	all = append(all, buildExpansionPack()...)
	all = append(all, buildImmortalEditionPack()...)
	all = append(all, buildResearchStrategies()...)

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
