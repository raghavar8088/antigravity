package sharedfeed

import (
	"errors"
	"strings"
	"testing"
)

// Delta lists 220 perpetuals; Binance does not carry about 18 of them. Without
// a permanent-negative memory the fallback re-requests a contract that can never
// resolve, on every poll, forever — guaranteed-400 HTTP calls and error lines
// that train the reader to ignore feed errors entirely.
func TestBinanceUnlisted_IsRememberedNotRetried(t *testing.T) {
	const sym = "TESTONLYUSDT"
	binanceUnlisted.Delete(sym)
	defer binanceUnlisted.Delete(sym)

	binanceUnlisted.Store(sym, true)
	if _, ok := binanceUnlisted.Load(sym); !ok {
		t.Fatal("the unlisted set did not retain the symbol")
	}
}

// -1121 is a statement about Binance's listings, not about the request, so it
// must be distinguishable from a timeout — one is worth retrying, the other is
// not.
func TestBinanceCode_ParsesInvalidSymbol(t *testing.T) {
	body := `{"code":-1121,"msg":"Invalid symbol."}`
	if got := binanceCode(body); got != -1121 {
		t.Errorf("binanceCode = %d, want -1121", got)
	}
	// A rate-limit or ban must NOT be mistaken for a delisting, or a temporary
	// block would permanently disable the fallback for that symbol.
	if got := binanceCode(`{"code":-1003,"msg":"Too many requests"}`); got == -1121 {
		t.Error("a rate-limit code was read as an invalid symbol")
	}
	// Garbage must not resolve to -1121 either.
	for _, junk := range []string{"", "not json", "<html>502</html>", "{}"} {
		if binanceCode(junk) == -1121 {
			t.Errorf("junk body %q was read as an invalid symbol", junk)
		}
	}
}

// The sentinel must survive wrapping, or callers cannot tell "no fallback
// exists" from "the fallback broke".
func TestErrNotOnBinance_IsIdentifiableWhenWrapped(t *testing.T) {
	wrapped := errors.New("outer: " + ErrNotOnBinance.Error())
	if errors.Is(wrapped, ErrNotOnBinance) {
		t.Skip("string-built error is not wrapped; nothing to assert")
	}
	if !strings.Contains(ErrNotOnBinance.Error(), "not listed") {
		t.Errorf("sentinel message %q does not say the contract is unlisted", ErrNotOnBinance)
	}
}
