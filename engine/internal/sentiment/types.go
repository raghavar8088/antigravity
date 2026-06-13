// Package sentiment polls a local FinBERT sentiment server and converts
// news sentiment into a directional signal in range [-3, +3].
package sentiment

import "time"

// SentimentData holds one polling cycle of aggregated news sentiment.
type SentimentData struct {
	Score       float64   // -1.0 to +1.0 weighted average
	Label       string    // "BULLISH" | "BEARISH" | "NEUTRAL"
	HotKeywords []string  // impactful terms found
	Velocity    int       // articles per hour
	Headlines   []string  // sanitized top 3 headlines
	FetchedAt   time.Time
}
