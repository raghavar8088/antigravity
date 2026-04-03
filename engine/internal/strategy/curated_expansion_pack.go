package strategy

import (
	"fmt"
	"strings"
)

// buildExpansionPack adds 301 unique strategies so the curated runtime pack
// reaches a clean total of 600 strategies.
func buildExpansionPack() []RegistryEntry {
	entries := make([]RegistryEntry, 0, 301)
	appendEntry := func(s Strategy, category, timeframe string) {
		entries = append(entries, RegistryEntry{
			Strategy:  s,
			Category:  category,
			Timeframe: timeframe,
		})
	}

	// 40 EMA crossover variants
	for _, fast := range []int{2, 3, 4, 5, 6} {
		for idx, slow := range []int{8, 10, 12, 15, 18, 21, 26, 34} {
			adxMin := float64(18 + (idx/2)*2)
			rsiLo := float64(45 - (idx % 3))
			rsiHi := float64(68 + (idx % 3))
			slPct := 0.15 + 0.01*float64((idx+fast)%4)
			tpPct := slPct * 2.2
			timeframe := "1m"
			if slow >= 26 {
				timeframe = "5m"
			}
			appendEntry(
				newEMACrossV2(
					fmt.Sprintf("XP_EMA_%d_%d_Cross", fast, slow),
					fast, slow, adxMin, rsiLo, rsiHi, slPct, tpPct,
				),
				"Trend",
				timeframe,
			)
		}
	}

	// 30 RSI threshold variants
	rsiThresholdConfigs := []struct {
		buyLo  float64
		buyHi  float64
		adxMin float64
		slPct  float64
		tpPct  float64
	}{
		{22, 28, 12, 0.18, 0.42},
		{28, 34, 14, 0.17, 0.40},
		{32, 38, 15, 0.17, 0.40},
		{40, 45, 18, 0.18, 0.42},
		{48, 52, 20, 0.18, 0.42},
		{55, 60, 22, 0.19, 0.44},
	}
	for _, period := range []int{7, 9, 14, 21, 28} {
		for _, cfg := range rsiThresholdConfigs {
			timeframe := "1m"
			if period >= 21 {
				timeframe = "5m"
			}
			appendEntry(
				newRSIThreshold(
					fmt.Sprintf(
						"XP_RSI_%d_%s_%s",
						period,
						floatToken(cfg.buyLo),
						floatToken(cfg.buyHi),
					),
					period, cfg.buyLo, cfg.buyHi, cfg.adxMin, cfg.slPct, cfg.tpPct,
				),
				"Mean Reversion",
				timeframe,
			)
		}
	}

	// 20 RSI slope variants
	rsiSlopeConfigs := []struct {
		lookback    int
		slopeThresh float64
		adxMin      float64
		slPct       float64
		tpPct       float64
	}{
		{2, 4.0, 16, 0.16, 0.38},
		{3, 5.0, 18, 0.17, 0.40},
		{4, 6.0, 18, 0.18, 0.42},
		{5, 8.0, 20, 0.18, 0.42},
		{6, 10.0, 22, 0.19, 0.44},
	}
	for _, period := range []int{7, 9, 14, 21} {
		for _, cfg := range rsiSlopeConfigs {
			timeframe := "1m"
			if period >= 21 || cfg.lookback >= 6 {
				timeframe = "5m"
			}
			appendEntry(
				newRSISlopeScalper(
					fmt.Sprintf(
						"XP_RSI_Slope_%d_%d_%s",
						period,
						cfg.lookback,
						floatToken(cfg.slopeThresh),
					),
					period, cfg.lookback, cfg.slopeThresh, cfg.adxMin, cfg.slPct, cfg.tpPct,
				),
				"Mean Rev Elite",
				timeframe,
			)
		}
	}

	// 25 Bollinger signal variants
	bbSignalConfigs := []struct {
		mode     string
		mult     float64
		adxMin   float64
		adxMax   float64
		rsiMin   float64
		rsiMax   float64
		slPct    float64
		tpPct    float64
		category string
	}{
		{"bounce_lower", 1.8, 0, 18, 28, 50, 0.17, 0.40, "Mean Reversion"},
		{"bounce_lower", 2.2, 0, 22, 26, 48, 0.18, 0.42, "Mean Reversion"},
		{"mid_cross", 2.0, 18, 0, 45, 70, 0.18, 0.42, "Trend"},
		{"breakout", 2.0, 22, 0, 52, 75, 0.19, 0.46, "Breakout Elite"},
		{"breakout", 2.5, 24, 0, 54, 78, 0.20, 0.50, "Breakout Elite"},
	}
	for _, period := range []int{14, 20, 30, 40, 50} {
		for _, cfg := range bbSignalConfigs {
			timeframe := "1m"
			if period >= 40 {
				timeframe = "5m"
			}
			appendEntry(
				newBBScalper(
					fmt.Sprintf(
						"XP_BB_%s_%d_%s",
						modeToken(cfg.mode),
						period,
						floatToken(cfg.mult),
					),
					cfg.mode, period, cfg.mult, cfg.adxMin, cfg.adxMax, cfg.rsiMin, cfg.rsiMax, cfg.slPct, cfg.tpPct,
				),
				cfg.category,
				timeframe,
			)
		}
	}

	// 15 Bollinger width variants
	for _, period := range []int{14, 20, 30, 40, 50} {
		for idx, mult := range []float64{1.8, 2.0, 2.5} {
			timeframe := "1m"
			if period >= 40 {
				timeframe = "5m"
			}
			appendEntry(
				newBBWidth(
					fmt.Sprintf("XP_BB_Width_%d_%s", period, floatToken(mult)),
					period,
					mult,
					float64(18+idx*2),
					44,
					72,
					0.17+0.01*float64(idx),
					0.40+0.04*float64(idx),
				),
				"Volatility",
				timeframe,
			)
		}
	}

	// 25 VWAP variants
	vwapConfigs := []struct {
		mode     string
		devPct   float64
		adxMin   float64
		rsiMin   float64
		rsiMax   float64
		slPct    float64
		tpPct    float64
		category string
	}{
		{"cross", 0, 18, 45, 70, 0.17, 0.40, "Trend"},
		{"cross", 0, 22, 45, 70, 0.18, 0.42, "Trend"},
		{"deviation", 0.20, 12, 36, 56, 0.16, 0.38, "Mean Rev Elite"},
		{"deviation", 0.45, 16, 30, 52, 0.18, 0.44, "Mean Rev Elite"},
		{"pullback", 0, 22, 45, 65, 0.18, 0.42, "Trend"},
	}
	for _, period := range []int{20, 30, 40, 55, 80} {
		for cfgIdx, cfg := range vwapConfigs {
			timeframe := "1m"
			if period >= 55 {
				timeframe = "5m"
			}
			appendEntry(
				newVWAPScalper(
					fmt.Sprintf(
						"XP_VWAP_%s_%d_%s_%d",
						modeToken(cfg.mode),
						period,
						floatToken(cfg.devPct),
						cfgIdx+1,
					),
					cfg.mode, period, cfg.devPct, cfg.adxMin, cfg.rsiMin, cfg.rsiMax, cfg.slPct, cfg.tpPct,
				),
				cfg.category,
				timeframe,
			)
		}
	}

	// 25 MACD variants
	macdBases := []struct {
		mode   string
		fast   int
		slow   int
		sig    int
		adxMin float64
		slPct  float64
		tpPct  float64
	}{
		{"cross", 5, 13, 3, 18, 0.17, 0.40},
		{"cross", 8, 17, 5, 20, 0.18, 0.42},
		{"zero_cross", 12, 26, 9, 20, 0.19, 0.44},
		{"hist_momentum", 8, 21, 5, 18, 0.18, 0.42},
		{"cross", 20, 40, 9, 22, 0.20, 0.48},
	}
	macdBands := [][2]float64{
		{44, 68},
		{45, 70},
		{46, 72},
		{48, 74},
		{42, 66},
	}
	for _, base := range macdBases {
		for bandIdx, band := range macdBands {
			timeframe := "1m"
			if base.slow >= 40 || bandIdx == len(macdBands)-1 {
				timeframe = "5m"
			}
			appendEntry(
				newMACDScalperV2(
					fmt.Sprintf(
						"XP_MACD_%s_%d_%d_%d_%d",
						modeToken(base.mode),
						base.fast,
						base.slow,
						base.sig,
						bandIdx+1,
					),
					base.mode, base.fast, base.slow, base.sig, base.adxMin, band[0], band[1], base.slPct, base.tpPct,
				),
				"Momentum Elite",
				timeframe,
			)
		}
	}

	// 20 N-bar breakout variants
	for _, bars := range []int{4, 6, 8, 10, 12, 14, 16, 18, 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60, 64} {
		timeframe := "1m"
		if bars >= 20 {
			timeframe = "5m"
		}
		adxMin := 16.0
		if bars >= 12 {
			adxMin = 18.0
		}
		if bars >= 24 {
			adxMin = 22.0
		}
		slPct := 0.16
		if bars >= 12 {
			slPct = 0.18
		}
		if bars >= 24 {
			slPct = 0.20
		}
		appendEntry(
			newNBarBreakout(
				fmt.Sprintf("XP_NBar_%d_Break", bars),
				bars,
				adxMin,
				52,
				74,
				slPct,
				slPct*2.4,
			),
			"Breakout Elite",
			timeframe,
		)
	}

	// 20 Triple EMA variants
	for _, combo := range [][3]int{
		{3, 8, 21}, {4, 9, 18}, {5, 10, 20}, {5, 13, 34}, {6, 14, 30},
		{7, 21, 50}, {8, 21, 55}, {9, 18, 36}, {10, 20, 40}, {10, 30, 60},
		{12, 24, 48}, {13, 34, 55}, {15, 30, 60}, {20, 50, 100}, {21, 55, 89},
		{4, 12, 26}, {6, 18, 42}, {8, 34, 89}, {9, 26, 55}, {14, 28, 70},
	} {
		timeframe := "1m"
		if combo[2] >= 50 {
			timeframe = "5m"
		}
		appendEntry(
			newTripleEMAV2(
				fmt.Sprintf("XP_Triple_%d_%d_%d", combo[0], combo[1], combo[2]),
				combo[0],
				combo[1],
				combo[2],
				18+float64(combo[2]%10)/2,
				45,
				70,
				0.17+float64(combo[0]%3)*0.01,
				0.40+float64(combo[1]%3)*0.04,
			),
			"Trend",
			timeframe,
		)
	}

	// 15 CCI variants
	cciConfigs := []struct {
		mode     string
		category string
	}{
		{"zero_cross", "Momentum Elite"},
		{"extreme_bounce", "Mean Reversion"},
		{"trend", "Trend"},
	}
	for _, period := range []int{10, 14, 20, 30, 40} {
		for _, cfg := range cciConfigs {
			timeframe := "1m"
			if period >= 30 {
				timeframe = "5m"
			}
			appendEntry(
				newCCIScalper(
					fmt.Sprintf("XP_CCI_%s_%d", modeToken(cfg.mode), period),
					cfg.mode,
					period,
					18+float64(period/10),
					30+float64(period/4),
					72,
					0.17+float64(period%3)*0.01,
					0.40+float64(period%3)*0.04,
				),
				cfg.category,
				timeframe,
			)
		}
	}

	// 15 stochastic variants
	stochConfigs := []struct {
		mode     string
		smooth   int
		category string
	}{
		{"cross", 3, "Mean Reversion"},
		{"oversold", 3, "Mean Reversion"},
		{"trend", 5, "Trend"},
	}
	for _, period := range []int{5, 9, 14, 21, 28} {
		for _, cfg := range stochConfigs {
			timeframe := "1m"
			if period >= 21 {
				timeframe = "5m"
			}
			appendEntry(
				newStochScalper(
					fmt.Sprintf("XP_Stoch_%s_%d_%d", modeToken(cfg.mode), period, cfg.smooth),
					cfg.mode,
					period,
					cfg.smooth,
					18+float64(period/7),
					44,
					70,
					0.16+float64(period%4)*0.01,
					0.38+float64(period%4)*0.04,
				),
				cfg.category,
				timeframe,
			)
		}
	}

	// 15 ATR variants
	atrConfigs := []struct {
		atrPeriod int
		emaPeriod int
	}{
		{7, 14}, {10, 20}, {14, 20}, {14, 50}, {21, 55},
	}
	atrModes := []struct {
		mode     string
		category string
	}{
		{"momentum", "Volatility"},
		{"channel_break", "Breakout Elite"},
		{"contraction", "Volatility"},
	}
	for _, cfg := range atrConfigs {
		for _, mode := range atrModes {
			timeframe := "1m"
			if cfg.emaPeriod >= 50 || cfg.atrPeriod >= 21 {
				timeframe = "5m"
			}
			appendEntry(
				newATRScalper(
					fmt.Sprintf("XP_ATR_%s_%d_%d", modeToken(mode.mode), cfg.atrPeriod, cfg.emaPeriod),
					mode.mode,
					cfg.atrPeriod,
					cfg.emaPeriod,
					18+float64(cfg.atrPeriod/4),
					46,
					70,
					0.17+float64(cfg.atrPeriod%3)*0.01,
					0.40+float64(cfg.emaPeriod%3)*0.04,
				),
				mode.category,
				timeframe,
			)
		}
	}

	// 12 ROC variants
	for _, period := range []int{3, 5, 7, 9, 12, 14} {
		for _, threshold := range []float64{0.30, 0.60} {
			timeframe := "1m"
			if period >= 14 {
				timeframe = "5m"
			}
			appendEntry(
				newROCScalper(
					fmt.Sprintf("XP_ROC_%d_%s", period, floatToken(threshold)),
					period,
					threshold,
					16+float64(period/2),
					46,
					70,
					0.16+float64(period%3)*0.01,
					0.38+float64(period%3)*0.04,
				),
				"Momentum Elite",
				timeframe,
			)
		}
	}

	// 8 Williams %R variants
	for _, period := range []int{7, 10, 14, 21} {
		for _, cfg := range []struct {
			mode     string
			category string
		}{
			{"bounce", "Mean Reversion"},
			{"trend", "Trend"},
		} {
			timeframe := "1m"
			if period >= 21 {
				timeframe = "5m"
			}
			appendEntry(
				newWilliamsRV2(
					fmt.Sprintf("XP_WR_%s_%d", modeToken(cfg.mode), period),
					cfg.mode,
					period,
					16+float64(period/4),
					30,
					72,
					0.16+float64(period%3)*0.01,
					0.38+float64(period%3)*0.04,
				),
				cfg.category,
				timeframe,
			)
		}
	}

	// 6 PSAR + EMA variants
	for _, emaPeriod := range []int{9, 14, 20} {
		for _, step := range []float64{0.01, 0.02} {
			timeframe := "1m"
			if emaPeriod >= 20 {
				timeframe = "5m"
			}
			appendEntry(
				newPsarEMA(
					fmt.Sprintf("XP_PSAR_EMA_%d_%s", emaPeriod, floatToken(step)),
					emaPeriod,
					step,
					0.20,
					18+float64(emaPeriod/5),
					46,
					70,
					0.16+step,
					0.38+step*4,
				),
				"Trend",
				timeframe,
			)
		}
	}

	// 5 Hull MA variants
	for _, period := range []int{12, 16, 20, 30, 50} {
		timeframe := "1m"
		if period >= 30 {
			timeframe = "5m"
		}
		appendEntry(
			newHullMAV2(
				fmt.Sprintf("XP_Hull_%d", period),
				period,
				18+float64(period/10),
				45,
				70,
				0.17+float64(period%3)*0.01,
				0.40+float64(period%3)*0.04,
			),
			"Trend",
			timeframe,
		)
	}

	// 5 Keltner variants
	keltnerConfigs := []struct {
		mode      string
		emaPeriod int
		atrPeriod int
		mult      float64
		category  string
	}{
		{"break", 20, 14, 1.5, "Breakout Elite"},
		{"break", 30, 14, 2.0, "Breakout Elite"},
		{"bounce", 20, 14, 1.5, "Mean Reversion"},
		{"midline", 20, 14, 1.5, "Trend"},
		{"midline", 50, 14, 2.0, "Trend"},
	}
	for _, cfg := range keltnerConfigs {
		timeframe := "1m"
		if cfg.emaPeriod >= 50 {
			timeframe = "5m"
		}
		appendEntry(
			newKeltnerV2(
				fmt.Sprintf(
					"XP_Kelt_%s_%d_%d_%s",
					modeToken(cfg.mode),
					cfg.emaPeriod,
					cfg.atrPeriod,
					floatToken(cfg.mult),
				),
				cfg.mode,
				cfg.emaPeriod,
				cfg.atrPeriod,
				cfg.mult,
				18+float64(cfg.emaPeriod/10),
				44,
				72,
				0.18,
				0.44,
			),
			cfg.category,
			timeframe,
		)
	}

	if len(entries) != 301 {
		panic(fmt.Sprintf("strategy expansion pack expected 301 entries, got %d", len(entries)))
	}

	return entries
}

func modeToken(mode string) string {
	replacer := strings.NewReplacer("-", "_", " ", "_")
	return replacer.Replace(mode)
}

func floatToken(value float64) string {
	token := fmt.Sprintf("%.2f", value)
	token = strings.TrimRight(token, "0")
	token = strings.TrimRight(token, ".")
	return strings.ReplaceAll(token, ".", "p")
}
