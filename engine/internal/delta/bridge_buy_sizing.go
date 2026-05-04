package delta

import (
	"math"
	"os"
	"strconv"
	"strings"
)

func parseEnvFloat(key string, def float64) float64 {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return def
	}
	return v
}

func parseEnvInt(key string, def int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	return v
}

// TieredBuySizing returns risk fraction, max contracts, and minimum wallet (USD)
// used when mirroring paper signals as long-option BUY orders on Delta.
//
// Override with env: DELTA_MIN_WALLET_USD, DELTA_BUY_RISK_PCT (0–1), DELTA_BUY_MAX_CONTRACTS.
func TieredBuySizing(walletUSD float64) (riskPct float64, maxContracts int, minWallet float64) {
	minWallet = parseEnvFloat("DELTA_MIN_WALLET_USD", 5)

	if v := parseEnvFloat("DELTA_BUY_RISK_PCT", 0); v > 0 && v <= 0.5 {
		riskPct = v
	} else {
		switch {
		case walletUSD < 25:
			riskPct = 0.07
		case walletUSD < 50:
			riskPct = 0.09
		case walletUSD < 100:
			riskPct = 0.10
		default:
			riskPct = 0.12
		}
	}

	if v := parseEnvInt("DELTA_BUY_MAX_CONTRACTS", 0); v > 0 {
		maxContracts = v
	} else {
		switch {
		case walletUSD < 25:
			maxContracts = 1
		case walletUSD < 50:
			maxContracts = 2
		case walletUSD < 100:
			maxContracts = 3
		default:
			maxContracts = 5
		}
	}
	return riskPct, maxContracts, minWallet
}

// BuyingContractsFromWallet sizes long-option mirror orders from **wallet USD** only.
// Paper `PremiumUSD` is not a reliable per-contract quote on Delta; we use a conservative
// premium estimate (override with DELTA_BUY_EST_PREMIUM_USD, default 35).
// Returns 0 if wallet is below DELTA_MIN_WALLET_USD (default 5).
func BuyingContractsFromWallet(walletUSD float64) int {
	if walletUSD <= 0 {
		return 0
	}
	riskPct, maxC, minW := TieredBuySizing(walletUSD)
	if walletUSD < minW {
		return 0
	}
	est := parseEnvFloat("DELTA_BUY_EST_PREMIUM_USD", 35)
	if est < 5 {
		est = 5
	}
	spend := walletUSD * riskPct
	n := int(math.Round(spend / est))
	if n < 1 {
		n = 1
	}
	if n > maxC {
		n = maxC
	}
	return n
}
