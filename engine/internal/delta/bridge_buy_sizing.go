package delta

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

// DeltaContractSizeBTC is the underlying per Delta BTC option/perp contract.
const DeltaContractSizeBTC = 0.001

// fallbackPremiumUSD is used only when a real per-contract quote is unavailable.
// Grounded in first-principles ATM pricing (0.4·S·σ·√T ≈ $1.78 for a weekly ATM
// BTC contract), not the old $35 guess which implied a deep-ITM/long-dated
// option. Override via DELTA_BUY_EST_PREMIUM_USD; its use is always logged.
const fallbackPremiumUSD = 1.78

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

	// Boundaries are inclusive of the ceiling: a $100 wallet — the configured
	// account size — must land in the 10% / 3-contract tier, not fall through to
	// the 12% / 5-contract default meant for larger accounts.
	if v := parseEnvFloat("DELTA_BUY_RISK_PCT", 0); v > 0 && v <= 0.5 {
		riskPct = v
	} else {
		switch {
		case walletUSD < 25:
			riskPct = 0.07
		case walletUSD < 50:
			riskPct = 0.09
		case walletUSD <= 100:
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
		case walletUSD <= 100:
			maxContracts = 3
		default:
			maxContracts = 5
		}
	}
	return riskPct, maxContracts, minWallet
}

// BuyingContractsFromWallet sizes a long-option buy from the wallet balance and
// the REAL per-contract premium in USD (option mark price × DeltaContractSizeBTC).
// It returns the number of contracts and, when that is zero, a human-readable
// skip reason for the audit log.
//
// Three invariants the old implementation violated:
//   - it never forces a position: if the risk budget cannot afford one contract
//     the result is 0 (+reason), never a forced single contract;
//   - it uses floor, so contracts × premium can never exceed the budget;
//   - the premium is a real quote. Pass premiumPerContractUSD ≤ 0 only when a
//     live quote is genuinely unavailable; the fallback estimate is then used
//     and the fallback is logged loudly every time.
func BuyingContractsFromWallet(walletUSD, premiumPerContractUSD float64) (int, string) {
	if walletUSD <= 0 {
		return 0, "wallet balance is zero"
	}
	riskPct, maxC, minW := TieredBuySizing(walletUSD)
	if walletUSD < minW {
		return 0, fmt.Sprintf("wallet $%.2f below minimum $%.2f", walletUSD, minW)
	}

	premium := premiumPerContractUSD
	if premium <= 0 {
		premium = parseEnvFloat("DELTA_BUY_EST_PREMIUM_USD", fallbackPremiumUSD)
		log.Printf("[DELTA SIZING] WARNING: no live premium quote — using fallback $%.2f/contract; sizing may be wrong until a real quote is available", premium)
	}
	if premium <= 0 {
		return 0, "no valid premium quote"
	}

	budget := walletUSD * riskPct
	n := int(math.Floor(budget / premium))
	if n < 1 {
		return 0, fmt.Sprintf("risk budget $%.2f (%.0f%% of $%.2f) cannot afford one contract at $%.2f premium", budget, riskPct*100, walletUSD, premium)
	}
	if n > maxC {
		n = maxC
	}
	return n, ""
}

// AssertBuyWithinBudget is the server-side backstop the spec requires: it rejects
// any order whose real cost exceeds the risk budget, independently of how the
// contract count was derived. Even if a future sizing path is wrong, this fails
// the order before it reaches the broker.
func AssertBuyWithinBudget(walletUSD, premiumPerContractUSD float64, contracts int) error {
	if contracts <= 0 {
		return fmt.Errorf("non-positive contract count %d", contracts)
	}
	if premiumPerContractUSD <= 0 {
		return fmt.Errorf("cannot verify budget without a real premium quote")
	}
	riskPct, maxC, _ := TieredBuySizing(walletUSD)
	if contracts > maxC {
		return fmt.Errorf("contracts %d exceed max %d for wallet $%.2f", contracts, maxC, walletUSD)
	}
	budget := walletUSD * riskPct
	cost := float64(contracts) * premiumPerContractUSD
	if cost > budget {
		return fmt.Errorf("order cost $%.2f exceeds risk budget $%.2f (%.0f%% of $%.2f)", cost, budget, riskPct*100, walletUSD)
	}
	return nil
}
