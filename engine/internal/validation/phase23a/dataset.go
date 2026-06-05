package phase23a

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"antigravity-engine/internal/marketdata"
)

// AuditDataset validates the integrity of a candle dataset.
func AuditDataset(candles []OHLCVCandle, symbol string) DatasetStats {
	ds := DatasetStats{Symbol: symbol}
	if len(candles) == 0 {
		ds.Issues = append(ds.Issues, "CRITICAL: no candle data")
		return ds
	}

	ds.From = candles[0].OpenTime
	ds.To = candles[len(candles)-1].CloseTime
	ds.CandleCount = len(candles)

	// Expected 1-minute candles
	duration := ds.To.Sub(ds.From)
	ds.ExpectedCandles = int(duration.Minutes())

	// Missing candles
	missing := ds.ExpectedCandles - ds.CandleCount
	if missing < 0 {
		missing = 0
	}
	if ds.ExpectedCandles > 0 {
		ds.MissingPct = float64(missing) / float64(ds.ExpectedCandles) * 100
	}

	// Outlier detection: candles where High-Low spread > 5%
	outliers := 0
	for _, c := range candles {
		if c.Close > 0 {
			spread := (c.High - c.Low) / c.Close * 100
			if spread > 5.0 {
				outliers++
			}
		}
		if c.FundingRate != 0 {
			ds.HasFunding = true
		}
		if c.OpenInterest > 0 {
			ds.HasOI = true
		}
		if c.LiquidationUSD > 0 {
			ds.HasLiquidations = true
		}
	}
	ds.OutlierCount = outliers

	// Quality score: 100 - penalties
	score := 100.0
	score -= ds.MissingPct * 2  // each 1% missing costs 2 quality points
	score -= math.Min(20, float64(outliers)/float64(len(candles))*100*5)
	if !ds.HasFunding {
		score -= 5
		ds.Issues = append(ds.Issues, "WARNING: no funding rate data — funding cost model will use defaults")
	}
	if !ds.HasOI {
		score -= 3
	}
	if ds.MissingPct > 5 {
		ds.Issues = append(ds.Issues, fmt.Sprintf("WARNING: %.1f%% missing candles may affect indicator accuracy", ds.MissingPct))
	}
	if ds.MissingPct > 20 {
		ds.Issues = append(ds.Issues, "CRITICAL: >20%% missing candles — dataset quality is insufficient")
	}
	ds.QualityScore = math.Max(0, math.Min(100, score))

	// Dataset duration check
	months := duration.Hours() / 24 / 30
	if months < 12 {
		ds.Issues = append(ds.Issues, fmt.Sprintf("WARNING: dataset covers only %.1f months (12+ preferred)", months))
	}

	return ds
}

// GenerateSyntheticCandles generates a realistic BTC futures 1-minute candle
// series for use in demo/test mode.  Uses geometric Brownian motion with
// realistic volatility, trend regimes, and funding rate cycles.
func GenerateSyntheticCandles(rng *rand.Rand, from time.Time, months int) []OHLCVCandle {
	// ~252 trading days × 1440 min/day ≈ 363,000 candles/year; we use 1m candles.
	totalMinutes := months * 30 * 24 * 60
	candles := make([]OHLCVCandle, 0, totalMinutes)

	price := 65000.0 // starting BTC price
	// drift and vol per minute
	annualVol := 0.80 // 80% annual vol for BTC
	dt := 1.0 / (252 * 1440)
	drift := 0.15 * dt           // 15% annual drift
	vol := annualVol * math.Sqrt(dt)

	// Regime parameters: cycle through regimes every ~7 days
	regimeCycleMins := 7 * 24 * 60
	fundingBase := 0.0001 // 0.01% per 8h

	t := from
	for i := 0; i < totalMinutes; i++ {
		// regime-adjusted parameters
		regimePhase := (i / regimeCycleMins) % 4
		var regimeDrift float64
		var regimeVol float64
		switch regimePhase {
		case 0: // bull
			regimeDrift = drift * 3
			regimeVol = vol * 1.2
		case 1: // bear
			regimeDrift = -drift * 2
			regimeVol = vol * 1.4
		case 2: // range
			regimeDrift = 0
			regimeVol = vol * 0.7
		case 3: // volatile
			regimeDrift = drift * 0.5
			regimeVol = vol * 2.0
		}

		// GBM price step
		z := rng.NormFloat64()
		ret := regimeDrift + regimeVol*z
		price *= math.Exp(ret)
		if price < 1000 {
			price = 1000 // floor
		}

		// OHLC construction: add intra-candle noise
		noise := price * regimeVol * 2
		high := price + math.Abs(rng.NormFloat64())*noise
		low := price - math.Abs(rng.NormFloat64())*noise
		open := price * (1 + (rng.Float64()-0.5)*regimeVol)
		if low > open {
			low = open * 0.999
		}
		if high < open {
			high = open * 1.001
		}
		if low > price {
			low = price * 0.999
		}
		if high < price {
			high = price * 1.001
		}

		// Volume: higher in volatile regimes
		volume := (500 + rng.Float64()*1500) * (1 + regimeVol/vol)

		// Funding rate: updated every 8h (480 min)
		fundingRate := 0.0
		if i%480 == 0 {
			// funding oscillates around fundingBase
			fundingRate = fundingBase + (rng.Float64()-0.5)*0.0002
		}

		// OI: slowly varying
		oi := 500_000_000 + rng.Float64()*200_000_000

		// Liquidations: rare spikes
		liq := 0.0
		if regimeVol > vol*1.5 && rng.Float64() < 0.01 {
			liq = 10_000_000 + rng.Float64()*50_000_000
		}

		openTime := t
		closeTime := t.Add(time.Minute - time.Millisecond)

		candles = append(candles, OHLCVCandle{
			Candle: marketdata.Candle{
				Symbol:    "BTC-USDT",
				Open:      open,
				High:      high,
				Low:       low,
				Close:     price,
				Volume:    volume,
				Trades:    int(volume / 0.1),
				OpenTime:  openTime,
				CloseTime: closeTime,
			},
			FundingRate:    fundingRate,
			OpenInterest:   oi,
			LiquidationUSD: liq,
		})
		t = t.Add(time.Minute)
	}
	return candles
}

// CandleToTick converts an OHLCVCandle to a marketdata.Tick (close price).
func CandleToTick(c OHLCVCandle) marketdata.Tick {
	return marketdata.Tick{
		Symbol:   c.Symbol,
		Price:    c.Close,
		Quantity: c.Volume,
		Side:     "BUY",
		TradeID:  c.OpenTime.UnixMilli(),
		TimeMs:   c.OpenTime.UnixMilli(),
	}
}

// FilterCandles filters a candle series to [from, to).
func FilterCandles(candles []OHLCVCandle, from, to time.Time) []OHLCVCandle {
	var out []OHLCVCandle
	for _, c := range candles {
		if !c.OpenTime.Before(from) && c.OpenTime.Before(to) {
			out = append(out, c)
		}
	}
	return out
}
