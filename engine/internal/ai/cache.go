package ai

import (
	"fmt"
	"hash/fnv"
	"math"
	"sync"
	"time"
)

type cachedDecision struct {
	decision  AIDecision
	expiresAt time.Time
}

type cachedAudit struct {
	approved   bool
	reason     string
	confidence float64
	provider   string
	expiresAt  time.Time
}

type DecisionCache struct {
	mu        sync.RWMutex
	decisions map[string]cachedDecision
	audits    map[string]cachedAudit
	shortTTL  time.Duration
	longTTL   time.Duration
	maxSize   int
	hits      uint64
	misses    uint64
}

type CacheStats struct {
	DecisionEntries int
	AuditEntries    int
	Hits            uint64
	Misses          uint64
	HitRate         float64
}

func NewDecisionCache(shortTTL, longTTL time.Duration, maxSize int) *DecisionCache {
	if shortTTL <= 0 {
		shortTTL = 5 * time.Second
	}
	if longTTL <= 0 {
		longTTL = 10 * time.Second
	}
	if maxSize <= 0 {
		maxSize = 4096
	}
	return &DecisionCache{
		decisions: make(map[string]cachedDecision),
		audits:    make(map[string]cachedAudit),
		shortTTL:  shortTTL,
		longTTL:   longTTL,
		maxSize:   maxSize,
	}
}

func (c *DecisionCache) MarketKey(market MarketContext) string {
	h := fnv.New64a()
	volume := 0.0
	if len(market.RecentCandles) > 0 {
		volume = market.RecentCandles[len(market.RecentCandles)-1].Volume
	}
	// Round noisy floats so materially equivalent states reuse decisions while
	// avoiding stale decisions after a real regime or indicator change.
	_, _ = fmt.Fprintf(h, "%s|p=%.1f|ema=%.1f:%.1f|vwap=%.1f|rsi=%.1f|adx=%.1f|atr=%.1f|vol=%.2f|regime=%s|pos=%d:%d:%d|loss=%d",
		market.Symbol,
		roundTo(market.Price, 0.1),
		roundTo(market.EMAFast, 0.1),
		roundTo(market.EMASlow, 0.1),
		roundTo(market.VWAP, 0.1),
		roundTo(market.RSI, 0.5),
		roundTo(market.ADX, 0.5),
		roundTo(market.ATR, 0.5),
		roundTo(volume, 0.01),
		market.Regime,
		market.OpenPositions,
		market.LongPositions,
		market.ShortPositions,
		market.ConsecutiveLosses,
	)
	return fmt.Sprintf("%x", h.Sum64())
}

func (c *DecisionCache) AuditKey(market MarketContext, strategyName, action, userNote string) string {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s|%s|%s|%s", c.MarketKey(market), strategyName, action, userNote)
	return fmt.Sprintf("%x", h.Sum64())
}

func (c *DecisionCache) GetDecision(key string) (AIDecision, bool) {
	c.mu.RLock()
	item, ok := c.decisions[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(item.expiresAt) {
		c.recordMiss()
		return AIDecision{}, false
	}
	c.recordHit()
	return item.decision, true
}

func (c *DecisionCache) SetDecision(key string, market MarketContext, decision AIDecision) {
	ttl := c.ttlForMarket(market)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictIfNeededLocked()
	c.decisions[key] = cachedDecision{decision: decision, expiresAt: time.Now().Add(ttl)}
}

func (c *DecisionCache) GetAudit(key string) (bool, string, float64, string, bool) {
	c.mu.RLock()
	item, ok := c.audits[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(item.expiresAt) {
		c.recordMiss()
		return false, "", 0, "", false
	}
	c.recordHit()
	return item.approved, item.reason, item.confidence, item.provider, true
}

func (c *DecisionCache) SetAudit(key string, market MarketContext, approved bool, reason string, confidence float64, provider string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictIfNeededLocked()
	c.audits[key] = cachedAudit{approved: approved, reason: reason, confidence: confidence, provider: provider, expiresAt: time.Now().Add(c.ttlForMarket(market))}
}

func (c *DecisionCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}
	return CacheStats{DecisionEntries: len(c.decisions), AuditEntries: len(c.audits), Hits: c.hits, Misses: c.misses, HitRate: hitRate}
}

func (c *DecisionCache) ttlForMarket(market MarketContext) time.Duration {
	if market.ATR > 0 && market.Price > 0 {
		atrPct := market.ATR / market.Price * 100
		if atrPct > 0.45 || market.ADX > 35 {
			return c.shortTTL
		}
	}
	if market.Regime == "VOLATILE" || market.Regime == "HIGH_VOLATILITY" {
		return c.shortTTL
	}
	return c.longTTL
}

func (c *DecisionCache) evictIfNeededLocked() {
	if len(c.decisions)+len(c.audits) < c.maxSize {
		return
	}
	now := time.Now()
	for k, v := range c.decisions {
		if now.After(v.expiresAt) {
			delete(c.decisions, k)
		}
	}
	for k, v := range c.audits {
		if now.After(v.expiresAt) {
			delete(c.audits, k)
		}
	}
	for len(c.decisions)+len(c.audits) >= c.maxSize {
		for k := range c.decisions {
			delete(c.decisions, k)
			return
		}
		for k := range c.audits {
			delete(c.audits, k)
			return
		}
	}
}

func (c *DecisionCache) recordHit() {
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
}

func (c *DecisionCache) recordMiss() {
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
}

func roundTo(v, step float64) float64 {
	if step <= 0 {
		return v
	}
	return math.Round(v/step) * step
}
