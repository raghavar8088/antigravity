package optionchain

import (
	"strings"
	"time"

	"antigravity-engine/internal/cryptofno"
)

// FnoChain adapts the shared chain cache to the crypto F&O desk.
//
// It reads the SAME snapshot the options desks use, so adding this desk costs
// zero extra upstream requests no matter how many users browse the chain.
type FnoChain struct {
	cache *Cache
	// spotFn supplies the underlying price. The chain alone does not carry spot,
	// and pricing a basket without it would be guessing.
	spotFn func(underlying string) float64
}

// ForCryptoFno binds the cache to the F&O desk.
func ForCryptoFno(c *Cache, spotFn func(string) float64) *FnoChain {
	return &FnoChain{cache: c, spotFn: spotFn}
}

// Contracts returns every live, QUOTED contract for an underlying. Unquoted
// contracts are omitted: a basket cannot be priced honestly against a strike
// with no market, so offering it in the builder would only produce a rejection
// at execution.
func (f *FnoChain) Contracts(underlying string) []cryptofno.ChainRow {
	if f == nil || f.cache == nil {
		return nil
	}
	f.cache.mu.RLock()
	snap := f.cache.snap
	f.cache.mu.RUnlock()

	want := strings.ToUpper(underlying)
	out := make([]cryptofno.ChainRow, 0, len(snap.contracts))
	for _, ct := range snap.contracts {
		if !strings.Contains(strings.ToUpper(ct.Symbol), "-"+want+"-") {
			continue
		}
		m, ok := snap.marks[ct.Symbol]
		if !ok || m.MarkPerBTC <= 0 {
			continue
		}
		typ := cryptofno.TypeCall
		if ct.OptionType == "PUT" {
			typ = cryptofno.TypePut
		}
		out = append(out, cryptofno.ChainRow{
			Symbol: ct.Symbol, ProductID: ct.ProductID, Type: typ,
			Strike: ct.Strike, Expiry: ct.Expiry,
			MarkPerBTC: m.MarkPerBTC, Bid: m.Bid, Ask: m.Ask,
			// Delta publishes IV per contract on the ticker; when it is missing
			// the margin engine falls back to intrinsic value, which is
			// conservative for a short.
			IV:            m.IV,
			ContractValue: deltaOptionContractBTC,
		})
	}
	return out
}

// Spot returns the underlying price.
func (f *FnoChain) Spot(underlying string) float64 {
	if f == nil || f.spotFn == nil {
		return 0
	}
	return f.spotFn(strings.ToUpper(underlying))
}

// deltaOptionContractBTC is Delta's contract_value for BTC/ETH options.
const deltaOptionContractBTC = 0.001

var _ = time.Time{}
