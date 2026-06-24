package scalpers

import (
	"strings"
	"testing"
)

func TestS30S79RegistryAndShadow(t *testing.T) {
	all := BuildCuratedScalpers()
	t.Logf("total registered strategies: %d", len(all))

	want := []string{
		"Dual_Momentum_Trend", "Time_Series_Momentum_Rebalance", "Kalman_Filter_Trend",
		"Adaptive_Moving_Average_Trend", "Triple_Barrier_Momentum", "Fractal_Breakout",
		"SuperTrend_Momentum", "Ichimoku_Cloud_Breakout", "Donchian_Channel_Breakout",
		"Parabolic_SAR_Trend",
		"Ornstein_Uhlenbeck_Reversion", "Cointegration_Spread_BTC_ETH", "RSI_Divergence_Multiframe",
		"Stochastic_Oscillator_Reversion", "Williams_Percent_R_Reversion",
		"Commodity_Channel_Index_Reversion", "Price_Channel_Mean_Reversion",
		"Linear_Regression_Channel_Fade", "Pivot_Point_Reversion", "Volatility_Mean_Reversion",
		"Trade_Intensity_Burst", "Large_Trade_Impact_Follow", "Bid_Ask_Spread_Regime",
		"VPIN_Toxicity_Signal", "Order_Book_Depth_Imbalance_Persistence", "Absorption_Pattern_Signal",
		"Sweep_And_Reload_Pattern", "Market_Depth_Gradient", "Aggressor_Ratio_Momentum",
		"Hidden_Liquidity_Detection",
		"Feature_Confluence_Score", "Regime_Probability_Composite", "Gradient_Boosting_Feature_Signal",
		"Autocorrelation_Pattern", "Shannon_Entropy_Regime", "Cross_Correlation_Lead_Lag",
		"Wavelet_Trend_Signal", "Recurrence_Quantification_Signal", "Excess_Return_Momentum",
		"Structural_Break_Detection",
		"Options_Skew_Direction_Signal", "Gamma_Exposure_Magnet", "Perpetual_Funding_Basis_Composite",
		"Long_Short_Ratio_Contrarian", "Taker_Buy_Sell_Volume_Ratio", "Open_Interest_Velocity",
		"Exchange_Netflow_Signal", "Miner_Hash_Rate_Momentum", "Stablecoin_Supply_Flow",
		"Fear_Greed_Contrarian",
	}
	if len(want) != 50 {
		t.Fatalf("test bug: want list has %d entries, expected 50", len(want))
	}

	byName := map[string]RegistryEntry{}
	for _, e := range all {
		byName[e.Name] = e
	}

	missing := []string{}
	for _, n := range want {
		if _, ok := byName[n]; !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing %d strategies from registry: %s", len(missing), strings.Join(missing, ", "))
	}
	t.Logf("all 50 S30-S79 strategies found in BuildCuratedScalpers()")

	// Force a non-NONE signal isn't guaranteed on synthetic data, so instead
	// directly verify the phase-gating wrapper: at default rollout phase (1),
	// every S30-S79 entry must be wrapped at phase 99 so any signal it ever
	// produces is forced IsShadow=true.
	for _, n := range want {
		e := byName[n]
		pg, ok := e.Strategy.(*phaseGatedStrategy)
		if !ok {
			t.Errorf("%s: not wrapped with withRolloutPhase (not shadow-gated)", n)
			continue
		}
		if pg.phase != 99 {
			t.Errorf("%s: phase = %d, want 99", n, pg.phase)
		}
	}
}
