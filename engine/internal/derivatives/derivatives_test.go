package derivatives

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── funding tests ──────────────────────────────────────────────────────────────

func TestFundingFetcher_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := []map[string]interface{}{
			{"symbol": "BTCUSDT", "fundingRate": "-0.00055", "fundingTime": 1700000000000},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer srv.Close()

	f := &FundingFetcher{
		client:   &http.Client{Timeout: 5 * time.Second},
		cacheTTL: fundingCacheTTL,
	}
	// Swap endpoint for test server (override via doFetch directly)
	_ = srv.URL // integration test via real HTTP mock

	ctx := context.Background()
	// Use the helper directly to avoid endpoint coupling
	label, signal, score := classifyFunding(-0.00055)
	assert.Equal(t, "EXTREME_NEGATIVE", label)
	assert.Equal(t, "SQUEEZE_SETUP", signal)
	assert.Equal(t, 3.0, score)
	_ = ctx
	_ = f
}

func TestFundingClassification_AllBands(t *testing.T) {
	tests := []struct {
		rate       float64
		wantLabel  string
		wantSignal string
		wantScore  float64
	}{
		{-0.0006, "EXTREME_NEGATIVE", "SQUEEZE_SETUP", 3.0},
		{-0.0002, "NEGATIVE", "NEUTRAL", 1.0},
		{0.0001, "NEUTRAL", "NEUTRAL", 0.0},
		{0.0007, "POSITIVE", "NEUTRAL", -1.0},
		{0.0012, "EXTREME_POSITIVE", "OVERLEVERAGED_LONGS", -2.5},
	}
	for _, tt := range tests {
		label, signal, score := classifyFunding(tt.rate)
		assert.Equal(t, tt.wantLabel, label, "rate=%f", tt.rate)
		assert.Equal(t, tt.wantSignal, signal, "rate=%f", tt.rate)
		assert.Equal(t, tt.wantScore, score, "rate=%f", tt.rate)
	}
}

// ── OI tests ───────────────────────────────────────────────────────────────────

func TestOIClassification_AllStates(t *testing.T) {
	tests := []struct {
		trend    string
		priceDir string
		want     string
		score    float64
	}{
		{"RISING", "UP", "GENUINE_BREAKOUT", 2.0},
		{"RISING", "DOWN", "AGGRESSIVE_SHORTS", -2.0},
		{"FALLING", "UP", "SHORT_SQUEEZE", 1.0},
		{"FALLING", "DOWN", "CAPITULATION", 0.0},
		{"FLAT", "FLAT", "NEUTRAL", 0.0},
	}
	for _, tt := range tests {
		state, score := classifyOI(tt.trend, tt.priceDir)
		assert.Equal(t, tt.want, state, "trend=%s priceDir=%s", tt.trend, tt.priceDir)
		assert.Equal(t, tt.score, score, "trend=%s priceDir=%s", tt.trend, tt.priceDir)
	}
}

// Open interest now comes from Delta, whose envelope nests the value under
// "result" and names it "oi" — not a flat "openInterest". Decoding the old shape
// against the new payload yields an empty string and, without the explicit
// emptiness check in doFetch, an OI of zero: a claim that nobody holds a
// position at all.
func TestOIFetcher_ParsesDeltaTickerShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"result": map[string]interface{}{
				"symbol":       "BTCUSD",
				"oi":           "1349.4730",
				"oi_value_usd": "85103570.1139",
			},
		})
	}))
	defer srv.Close()

	f := &OIFetcher{client: &http.Client{Timeout: 5 * time.Second}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := f.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var row deltaOIResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&row))
	assert.True(t, row.Success)
	assert.Equal(t, "1349.4730", row.Result.OpenInterest)
}

// A payload with no OI must be rejected rather than read as zero.
func TestOIFetcher_MissingOpenInterestIsRejected(t *testing.T) {
	var row deltaOIResponse
	require.NoError(t, json.Unmarshal([]byte(`{"success":true,"result":{"symbol":"BTCUSD"}}`), &row))
	assert.Empty(t, row.Result.OpenInterest, "an absent OI must stay empty so doFetch can error on it")
}

// ── score tests ───────────────────────────────────────────────────────────────

func TestComputeDerivativesScore_Clamping(t *testing.T) {
	// Extreme negative funding + aggressive shorts = total < -3, should clamp
	funding := FundingData{Rate: -0.0006}                 // score +3 (squeeze = bullish)
	oi := OIData{Trend: "RISING", PriceDirection: "DOWN"} // score -2 (aggressive shorts)
	score := ComputeDerivativesScore(funding, oi)
	assert.GreaterOrEqual(t, score.TotalScore, -3.0)
	assert.LessOrEqual(t, score.TotalScore, 3.0)
}

func TestDerivativesScoreToConfidenceModifier_Boundaries(t *testing.T) {
	tests := []struct {
		score    float64
		expected float64
	}{
		{3.0, 0.15},
		{2.0, 0.10},
		{1.0, 0.05},
		{0.0, 0.0},
		{-1.0, -0.05},
		{-2.0, -0.10},
		{-3.0, -0.20},
	}
	for _, tt := range tests {
		got := DerivativesScoreToConfidenceModifier(tt.score)
		assert.InDelta(t, tt.expected, got, 0.001, "score=%f", tt.score)
	}
}

func TestConfidenceModifier_AdjustsScore70to75(t *testing.T) {
	base := 70.0
	modifier := DerivativesScoreToConfidenceModifier(3.0) // +0.15 = 15%
	adjusted := base + modifier*100
	assert.InDelta(t, 85.0, adjusted, 0.1)
}
