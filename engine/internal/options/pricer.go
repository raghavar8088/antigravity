package options

import (
	"math"
	"time"
)

const riskFreeRate = 0.05 // 5% annual

// normCDF is the standard normal cumulative distribution function
func normCDF(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

// normPDF is the standard normal probability density function
func normPDF(x float64) float64 {
	return math.Exp(-0.5*x*x) / math.Sqrt(2*math.Pi)
}

// PriceResult holds the option price and Greeks
type PriceResult struct {
	Premium float64
	Delta   float64
	Gamma   float64
	Theta   float64
	Vega    float64
}

// PriceOption calculates the Black-Scholes price and Greeks for a European option.
func PriceOption(spot, strike float64, expiry time.Time, iv float64, optType OptionType) PriceResult {
	T := time.Until(expiry).Hours() / 8760.0 // Fraction of a year
	if T <= 0 {
		var intrinsic float64
		if optType == Call {
			intrinsic = math.Max(spot-strike, 0)
		} else {
			intrinsic = math.Max(strike-spot, 0)
		}
		return PriceResult{Premium: intrinsic}
	}

	sqrtT := math.Sqrt(T)
	d1 := (math.Log(spot/strike) + (riskFreeRate+iv*iv/2)*T) / (iv * sqrtT)
	d2 := d1 - iv*sqrtT

	var premium, delta float64
	if optType == Call {
		premium = spot*normCDF(d1) - strike*math.Exp(-riskFreeRate*T)*normCDF(d2)
		delta = normCDF(d1)
	} else {
		premium = strike*math.Exp(-riskFreeRate*T)*normCDF(-d2) - spot*normCDF(-d1)
		delta = normCDF(d1) - 1
	}

	gamma := normPDF(d1) / (spot * iv * sqrtT)
	vega := spot * normPDF(d1) * sqrtT / 100
	theta := -(spot*normPDF(d1)*iv/(2*sqrtT) + riskFreeRate*strike*math.Exp(-riskFreeRate*T)*normCDF(d2)) / 365

	if premium < 0.01 {
		premium = 0.01
	}

	return PriceResult{
		Premium: premium,
		Delta:   delta,
		Gamma:   gamma,
		Theta:   theta,
		Vega:    vega,
	}
}

// EstimateIV derives implied volatility from recent 1-minute price closes.
func EstimateIV(prices []float64) float64 {
	n := len(prices)
	if n < 10 {
		return 0.80
	}
	if n > 60 {
		prices = prices[n-60:]
	}

	var returns []float64
	for i := 1; i < len(prices); i++ {
		if prices[i-1] > 0 {
			returns = append(returns, math.Log(prices[i]/prices[i-1]))
		}
	}
	if len(returns) < 2 {
		return 0.80
	}

	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(returns))

	annVol := math.Sqrt(variance * 525600) // 525,600 minutes per year
	if annVol < 0.40 {
		annVol = 0.40
	}
	if annVol > 2.50 {
		annVol = 2.50
	}
	return annVol
}
