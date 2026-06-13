package sentiment

import (
	"math"
	"testing"
)

func approxEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestComputeSentimentScore_Bullish(t *testing.T) {
	data := SentimentData{Score: 0.8, Velocity: 5}
	s := ComputeSentimentScore(data)
	if !approxEqual(s, 2.4, 1e-9) {
		t.Errorf("expected ~2.4, got %v", s)
	}
}

func TestComputeSentimentScore_Bearish(t *testing.T) {
	data := SentimentData{Score: -0.9, Velocity: 3}
	s := ComputeSentimentScore(data)
	if s != -2.7 {
		t.Errorf("expected -2.7, got %v", s)
	}
}

func TestComputeSentimentScore_HighVelocityReduced(t *testing.T) {
	data := SentimentData{Score: 1.0, Velocity: 20}
	s := ComputeSentimentScore(data)
	// 3.0 * 0.7 = 2.1
	if !approxEqual(s, 2.1, 1e-9) {
		t.Errorf("expected ~2.1, got %v", s)
	}
}

func TestComputeSentimentScore_CrisisKeyword(t *testing.T) {
	data := SentimentData{Score: 0.5, HotKeywords: []string{"hack"}, Velocity: 5}
	s := ComputeSentimentScore(data)
	if s != -1.5 {
		t.Errorf("crisis keyword should floor at -1.5, got %v", s)
	}
}

func TestComputeSentimentScore_CrisisOverridesBullish(t *testing.T) {
	data := SentimentData{Score: 1.0, HotKeywords: []string{"ban"}, Velocity: 5}
	s := ComputeSentimentScore(data)
	if s != -1.5 {
		t.Errorf("ban keyword must cap at -1.5, got %v", s)
	}
}

func TestGetLatest_NilBeforeFetch(t *testing.T) {
	f := NewSentimentFetcher(nil, "http://localhost:8001")
	if f.GetLatest() != nil {
		t.Error("expected nil before first fetch")
	}
}
