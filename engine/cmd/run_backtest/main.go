package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"antigravity-engine/internal/backtest"
	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

type ResultRow struct {
	Rank           int     `json:"rank"`
	Strategy       string  `json:"strategy"`
	TotalTrades    int     `json:"total_trades"`
	WinRate        float64 `json:"win_rate_pct"`
	Sharpe         float64 `json:"sharpe"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	ProfitFactor   float64 `json:"profit_factor"`
	TotalReturnPct float64 `json:"total_return_pct"`
	AvgWinPct      float64 `json:"avg_win_pct"`
	AvgLossPct     float64 `json:"avg_loss_pct"`
	MeetsPromotion bool    `json:"meets_promotion"`
	DemoteReason   string  `json:"demote_reason,omitempty"`
}

func main() {
	symbol   := flag.String("symbol", "BTCUSDT", "trading symbol")
	fromStr  := flag.String("from", "2021-01-01", "backtest start YYYY-MM-DD")
	toStr    := flag.String("to", "2026-06-26", "backtest end YYYY-MM-DD")
	cacheDir := flag.String("cache-dir", "data/historical", "directory with cached JSON candles")
	outFile  := flag.String("out", "backtest_results.json", "output JSON file path")
	hwOnly   := flag.Bool("hw-only", false, "run only the 10 high-win-rate HW strategies")
	flag.Parse()

	from, err := time.Parse("2006-01-02", *fromStr)
	if err != nil {
		log.Fatalf("invalid --from: %v", err)
	}
	to, err := time.Parse("2006-01-02", *toStr)
	if err != nil {
		log.Fatalf("invalid --to: %v", err)
	}

	fmt.Printf("=== Antigravity Backtest ===\n")
	fmt.Printf("Symbol   : %s\n", *symbol)
	fmt.Printf("Period   : %s → %s\n", from.Format("2006-01-02"), to.Format("2006-01-02"))
	fmt.Printf("Cache dir: %s\n\n", *cacheDir)

	// Load all timeframes
	fmt.Println("Loading historical data...")
	loader := marketdata.NewHistoricalLoader(*cacheDir)
	ds, err := loader.BuildMTFDataset(*symbol, from, to)
	if err != nil {
		log.Fatalf("Failed to load dataset: %v\n\nMake sure data is downloaded first:\n  go run ./cmd/fetch_historical/main.go --symbol %s --from %s --to %s --cache-dir %s",
			err, *symbol, *fromStr, *toStr, *cacheDir)
	}

	fmt.Printf("Loaded candles — 1m:%d  5m:%d  15m:%d  1h:%d  4h:%d  1d:%d\n",
		len(ds.Candles1m), len(ds.Candles5m), len(ds.Candles15m),
		len(ds.Candles1h), len(ds.Candles4h), len(ds.Candles1d))
	fmt.Printf("Funding rates: %d  |  OI entries: %d\n\n", len(ds.FundingRates), len(ds.OIHistory))

	// Combine all strategies and filter to OHLCV-compatible only.
	// Strategies requiring live CVD, OI, ETH, DVOL, or options feeds are excluded.
	raw := append(scalers.BuildAllScalpers(), scalers.BuildPortedStrategies()...)
	var allEntries []scalers.RegistryEntry
	var skipped []string
	hwNames := map[string]bool{
		// HW1–HW10 (original)
		"HTF_Aligned_Pullback_Long": true, "HTF_Aligned_Pullback_Short": true,
		"BB_Squeeze_Breakout_Long": true, "BB_Squeeze_Breakout_Short": true,
		"Dual_RSI_Oversold_Long": true, "Dual_RSI_Overbought_Short": true,
		"MACD_Zero_Cross_Trend_Long": true, "MACD_Zero_Cross_Trend_Short": true,
		"Funding_Extreme_Contrarian_Long": true, "Funding_Extreme_Contrarian_Short": true,
		// HW11–HW35 (v2 research-backed)
		"Supertrend_Flip_Long": true, "Supertrend_Flip_Short": true,
		"Stoch_Bear_Cross_Short": true, "MACD_Hist_Expansion_Short": true,
		"StochRSI_4h_Oversold_Long": true, "StochRSI_4h_Overbought_Short": true,
		"Squeeze_Fired_Long": true, "Squeeze_Fired_Short": true,
		"WilliamsR_Oversold_Long": true, "WilliamsR_Overbought_Short": true,
		"BB_Lower_Break_Short": true, "Fisher_Extreme_Short": true,
		"CMF_Cross_Bullish_Long": true, "CMF_Cross_Bearish_Short": true,
		"Aroon_Crossover_Bull_Long": true, "Aroon_Crossover_Bear_Short": true,
		"Donchian_Breakout_Long": true, "Donchian_Breakdown_Short": true,
		"KST_Bullish_Cross_Long": true, "KST_Bearish_Cross_Short": true,
		"OBV_Breakout_Long": true, "OBV_Breakdown_Short": true,
		"MACD_Hist_Expansion_Long": true,
		"EFI_Bullish_Cross_Long": true, "EFI_Bearish_Cross_Short": true,
		// HW36–HW50 (v3 new strategies)
		"PSAR_4h_Flip_Long": true, "PSAR_4h_Flip_Short": true,
		"DEMA_4h_Golden_Cross_Long": true, "DEMA_4h_Death_Cross_Short": true,
		"RSI_4h_Momentum_Short": true, "Chandelier_Break_Short": true,
		"Coppock_Cross_Long": true,
		"EMA9_Cross_EMA21_Long": true, "EMA9_Cross_EMA21_Short": true,
		"Triple_EMA_Bull_Long": true, "Triple_EMA_Bear_Short": true,
		"ADX_Surge_Breakout_Long": true, "ADX_Surge_Breakout_Short": true,
		"MACD_Double_Bearish_Short": true, "HMA_4h_Direction_Short": true,
		// HW51–HW60 (v4 strategies)
		"MACD_Signal_Cross_Short": true, "MACD_Signal_Cross_Long": true,
		"RSI_Mid_Cross_Short": true, "RSI_Mid_Cross_Long": true,
		"Three_EMA_Cascade_Short": true, "EMA8_Cross_EMA50_Short": true,
		"Bearish_Momentum_Candle_Short": true, "Bearish_Engulfing_Short": true,
		"BB_Upper_Rejection_Short": true, "Price_EMA21_Breakdown_Short": true,
		"Bearish_Pin_Bar_Short": true, "Bullish_Hammer_Long": true,
		"Fisher_Zero_Cross_Short": true, "MACD_Hist_Sign_Flip_Short": true,
		// HW66–HW85 (v5 strategies)
		"WR_OB_Exit_Short": true, "StochRSI_KD_OB_Short": true,
		"CMF_Zero_Bear_Short": true, "KST_Signal_Cross_Bear_Short": true,
		"EFI_Zero_Cross_Bear_Short": true, "BB_Mid_Break_Short": true,
		"EMA21_EMA50_Death_Cross_Short": true, "Donchian_Mid_Break_Short": true,
		"Fisher_High_Exit_Short": true, "RSI_60_Break_Short": true,
		"HMA_Slope_Turn_Short": true, "Coppock_Zero_Bear_Short": true,
		"DEMA_Death_Cross_Short": true, "Keltner_Lower_Break_Short": true,
		"OBV_Slope_Bear_Short": true, "ZLEMA_Bear_Cross_Short": true,
		"WMA_Bear_Cross_Short": true, "RSI_65_Break_Short": true,
		"Chandelier_Bear_Break_Short": true, "BB_Width_Expand_Bear_Short": true,
		// HW86–HW105 (v6 strategies)
		"Three_Bear_Candles_Short": true, "Two_Bar_Bear_Reversal_Short": true,
		"Lower_High_Confirm_Short": true, "Big_Bear_Candle_Short": true,
		"MACD_Line_Zero_Cross_Short": true, "BB_Upper_Walk_Fail_Short": true,
		"ATR_Spike_Bear_Short": true, "Price_Below_All_EMAs_Short": true,
		"PSAR_ADX_Strong_Short": true, "StochRSI_50_Cross_Bear_Short": true,
		"WR_Mid_Cross_Bear_Short": true, "CMF_Deep_Bear_Short": true,
		"ADX_Strong_Trend_Bear_Short": true, "Multi_EMA_Full_Cascade_Short": true,
		"RSI_Bear_Slope_Short": true, "KST_Below_Zero_Bear_Short": true,
		"Aroon_Bear_Strong_Short": true, "EFI_Bear_Cross_Short": true,
		"Fisher_Deep_Bear_Short": true, "Close_Low_Range_Bear_Short": true,
		// HW106–HW125 (v7 strategies)
		"MTF_RSI_Dual_Bear_Short": true, "MTF_EMA_Confirm_Bear_Short": true,
		"1h_MACD_Cross_Bear_Short": true, "1h_RSI_60_Break_Bear_Short": true,
		"1h_EMA_Death_Cross_Bear_Short": true, "MTF_CMF_Both_Neg_Short": true,
		"1h_Fisher_Zero_Cross_Short": true, "1h_WR_OB_Cross_Bear_Short": true,
		"MTF_MACD_Both_Neg_Short": true, "1h_CMF_Zero_Cross_Bear_Short": true,
		"1h_StochRSI_OB_Cross_Bear_Short": true, "MTF_RSI_Cross_Confirm_Short": true,
		"1h_BB_Mid_Break_Bear_Short": true, "1h_Supertrend_Bear_Short": true,
		"1h_EFI_Zero_Cross_Bear_Short": true, "1h_RSI_Mid_Bear_Cross_Short": true,
		"1h_OBV_Bear_SMA_Cross_Short": true, "1h_Keltner_Break_Bear_Short": true,
		"1h_Donchian_Break_Bear_Short": true, "1d_MACD_Neg_4h_Entry_Short": true,
		// HW126–HW145 (v8 strategies)
		"RSI_MACD_KST_Bear_Short": true, "CMF_OBV_Confluence_Bear_Short": true,
		"Fisher_CMF_Confirm_Bear_Short": true, "StochRSI_CMF_Bear_Short": true,
		"WR_RSI_Bear_Confirm_Short": true, "Aroon_CMF_Bear_Dual_Short": true,
		"BB_RSI_Double_Bear_Short": true, "KST_EFI_Bear_Short": true,
		"HMA_CMF_Bear_Short": true, "DEMA_RSI_Bear_Short": true,
		"ZLEMA_EFI_Bear_Short": true, "Donchian_CMF_Bear_Short": true,
		"Keltner_CMF_Bear_Short": true, "StochRSI_ADX_Bear_Short": true,
		"Five_Indicator_Bear_Short": true, "Supertrend_CMF_Bear_Short": true,
		"WMA_CMF_Bear_Short": true, "ATR_CMF_Expand_Bear_Short": true,
		"RSI_Fisher_Bear_Short": true, "Coppock_KST_Bear_Short": true,
		"CMF_Extreme_Bear_Short": true,
		// HW146–HW165 (v9 strategies)
		"WMA5_WMA13_Bear_Cross_Short": true, "WMA13_WMA34_Bear_Cross_Short": true,
		"ATR_Expand_EFI_Bear_Short": true, "ATR_Expand_Fisher_Bear_Short": true,
		"ATR_Expand_WR_Bear_Short": true, "WMA9_CMF_Deep_Bear_Short": true,
		"WMA5_CMF_Bear_Short": true, "WMA13_CMF_Bear_Short": true,
		"WMA_EFI_Bear_Short": true, "WMA_Fisher_Bear_Short": true,
		"WMA_RSI_Bear_Short": true, "WMA_ADX_Strong_Bear_Short": true,
		"WMA_KST_Bear_Short": true, "ZLEMA_CMF_Bear_Short": true,
		"DEMA_CMF_Bear_Short": true, "Fisher_WR_Bear_Short": true,
		"Coppock_CMF_Bear_Short": true, "EMA8_EMA50_CMF_Bear_Short": true,
		"1h_WMA_Bear_CMF_Short": true, "1h_WMA_EFI_Bear_Short": true,
		// HW166–HW185 (v10 strategies)
		"WMA5_WMA13_ADX_Bear_Short": true, "WMA5_WMA13_EFI_Bear_Short": true,
		"WMA13_WMA34_ADX_Bear_Short": true, "WMA13_WMA34_EFI_Bear_Short": true,
		"WMA13_WMA34_CMF_Deep_Bear_Short": true, "ZLEMA_ADX_Bear_Short": true,
		"ZLEMA_Fisher_Bear_Short": true, "DEMA_ADX_Strong_Bear_Short": true,
		"DEMA_EFI_Bear_Short": true, "1h_WMA9_WMA21_ADX_Bear_Short": true,
		"1h_ZLEMA_CMF_Bear_Short": true, "1h_Fisher_CMF_Confirm_Short": true,
		"1h_WMA5_WMA13_Bear_Short": true, "1h_WR_CMF_Bear_Short": true,
		"1h_RSI_CMF_Bear_Short": true, "1h_EFI_CMF_Bear_Short": true,
		"1h_DEMA_CMF_Bear_Short": true, "1h_BB_Mid_CMF_Bear_Short": true,
		"1h_KST_CMF_Bear_Short": true, "1h_StochRSI_CMF_Bear_Short": true,
		// HW186–HW205 (v11 strategies)
		"WMA9_Aroon_Bear_Short": true, "WMA9_Donchian_Bear_Short": true,
		"EMA9_ADX_Strong_Bear_Short": true, "EMA9_EFI_Bear_Short": true,
		"BB_Mid_ADX_Bear_Short": true, "Keltner_ADX_Bear_Short": true,
		"Donchian_ADX_Bear_Short": true, "RSI55_Break_ADX_Short": true,
		"MACD_Hist_Flip_ADX_Short": true, "Supertrend_CMF_ADX_Bear_Short": true,
		"WMA9_Fisher_Aroon_Bear_Short": true, "WMA9_EFI_ADX_Bear_Short": true,
		"WMA9_CMF_ADX_Bear_Short": true, "1h_WMA9_ADX_EFI_Bear_Short": true,
		"1h_EMA9_CMF_Bear_Short": true, "1h_EMA9_EFI_Bear_Short": true,
		"Fisher_ADX_Bear_Short": true, "EFI_CMF_Bear_Dual_Short": true,
		"WR_CMF_Bear_Short": true, "OBV_Bear_CMF_Short": true,
		// v12 (HW206-HW230)
		"Aroon_CMF_Bear_V_Short": true, "Aroon_EFI_Bear_Short": true,
		"Donchian_EFI_Bear_Short": true, "BB_Mid_EFI_Bear_Short": true,
		"Keltner_Mid_EFI_Bear_Short": true, "KST_ADX_Bear_Short": true,
		"KST_EFI_Bear_V_Short": true, "Coppock_ADX_Bear_Short": true,
		"Coppock_EFI_Bear_Short": true, "Lower_High_CMF_Bear_Short": true,
		"Big_Bear_Candle_ADX_Short": true, "Three_Bear_Candles_ADX_Short": true,
		"1h_MACD_Zero_EFI_Short": true, "1h_RSI45_Break_CMF_Short": true,
		"1h_Fisher_EFI_Bear_Short": true, "1h_WMA13_CMF_Bear_Short": true,
		"1h_Donchian_CMF_Bear_Short": true, "RSI48_Break_CMF_Short": true,
		"RSI48_Break_EFI_Short": true, "Two_Bar_Bear_Reversal_ADX_Short": true,
		"Close_Low_Range_ADX_Short": true, "1h_MACD_Hist_Flip_CMF_Short": true,
		"1h_BB_Width_CMF_Bear_Short": true, "1h_Aroon_CMF_Bear_Short": true,
		"1h_KST_EFI_Bear_Short": true,
		// v13 (HW231-HW245)
		"MACD_Signal_EFI_ADX_Short": true, "Stoch_Bear_EFI_ADX_Short": true,
		"OBV_Slope_EFI_ADX_Short": true, "BB_Squeeze_EFI_ADX_Short": true,
		"EMA8_Cross_EFI_ADX_Short": true, "EMA8_Cross_CMF_ADX_Short": true,
		"Fisher_Zero_EFI_ADX_Short": true, "Donchian_Lower_EFI_ADX_Short": true,
		"WMA9_CMF_Deep_EFI_Short": true, "WR_Mid_EFI_ADX_Short": true,
		"CMF_Cross_EFI_ADX_Short": true, "Keltner_Lower_EFI_ADX_Short": true,
		"Bearish_Engulf_EFI_ADX_Short": true, "Bearish_Momentum_EFI_ADX_Short": true,
		"HMA9_Slope_EFI_ADX_Short": true,
		"EFI_Cross_Zero_ADX_CMF_Short": true, "WMA9_Fisher_ADX_Short": true,
	}
	for _, e := range raw {
		if e.OHLCVCompatible {
			if !*hwOnly || hwNames[e.Name] {
				allEntries = append(allEntries, e)
			}
		} else {
			skipped = append(skipped, e.Name)
		}
	}
	fmt.Printf("Strategies eligible (OHLCV-compatible): %d\n", len(allEntries))
	fmt.Printf("Skipped (require live feeds): %d — %v\n\n", len(skipped), skipped)

	cfg := backtest.DefaultRunConfig(*symbol)
	runner := backtest.NewBatchRunner(ds, cfg, allEntries)

	start := time.Now()
	results := runner.RunAll(func(done, total int, name string) {
		if name != "" {
			fmt.Printf("\r[%d/%d] %-50s", done, total, name)
		} else {
			fmt.Printf("\r[%d/%d] Done!%50s\n", done, total, "")
		}
	})
	elapsed := time.Since(start)

	fmt.Printf("\nBacktest completed in %s\n\n", elapsed.Round(time.Second))

	// Evaluate promotion eligibility (nil = skip stress-pass check)
	for i := range results {
		backtest.EvaluatePromotion(&results[i], nil)
	}

	// Sort by Sharpe descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Sharpe > results[j].Sharpe
	})

	// Build output rows
	rows := make([]ResultRow, len(results))
	for i, r := range results {
		rows[i] = ResultRow{
			Rank:           i + 1,
			Strategy:       r.StrategyName,
			TotalTrades:    r.TotalTrades,
			WinRate:        round2(r.WinRate * 100),
			Sharpe:         round2(r.Sharpe),
			MaxDrawdownPct: round2(r.MaxDrawdownPct),
			ProfitFactor:   round2(r.ProfitFactor),
			TotalReturnPct: round2(r.TotalReturnPct),
			AvgWinPct:      round2(r.AvgWinPct),
			AvgLossPct:     round2(r.AvgLossPct),
			MeetsPromotion: r.MeetsPromotion,
			DemoteReason:   r.DemoteReason,
		}
	}

	// Print leaderboard table to console
	fmt.Printf("%-4s %-40s %6s %7s %7s %8s %8s %9s\n",
		"Rank", "Strategy", "Trades", "WinRate", "Sharpe", "MaxDD%", "PF", "Return%")
	fmt.Println(fmt.Sprintf("%s", repeat("-", 100)))
	for _, r := range rows {
		promo := " "
		if r.MeetsPromotion {
			promo = "★"
		}
		fmt.Printf("%s%-3d %-40s %6d %6.1f%% %7.2f %7.2f%% %7.2f %8.2f%%\n",
			promo, r.Rank, truncate(r.Strategy, 40),
			r.TotalTrades, r.WinRate, r.Sharpe,
			r.MaxDrawdownPct, r.ProfitFactor, r.TotalReturnPct)
	}

	// Count promotions
	promoted := 0
	for _, r := range rows {
		if r.MeetsPromotion {
			promoted++
		}
	}
	fmt.Printf("\n★ = meets promotion criteria  |  %d/%d strategies qualify\n", promoted, len(rows))

	// Save to JSON
	out := map[string]interface{}{
		"symbol":     *symbol,
		"from":       from.Format("2006-01-02"),
		"to":         to.Format("2006-01-02"),
		"ran_at":     time.Now().UTC().Format(time.RFC3339),
		"elapsed_s":  elapsed.Seconds(),
		"strategies": rows,
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(*outFile, data, 0o644); err != nil {
		log.Printf("Warning: could not write %s: %v", *outFile, err)
	} else {
		fmt.Printf("\nFull results saved to: %s\n", *outFile)
	}
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
