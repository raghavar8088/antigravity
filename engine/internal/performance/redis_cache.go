package performance

import (
	"context"
	"time"
)

type CacheClient interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type CacheKeyspace string

const (
	CacheStrategyRankings CacheKeyspace = "strategy_rankings"
	CacheResearchResults  CacheKeyspace = "research_results"
	CachePortfolioMetrics CacheKeyspace = "portfolio_metrics"
	CacheMarketState      CacheKeyspace = "market_state"
	CacheAIDecisions      CacheKeyspace = "ai_decisions"
	CacheRiskMetrics      CacheKeyspace = "risk_metrics"
	CacheDashboardViews   CacheKeyspace = "dashboard_views"
)

type CachePolicy struct {
	Keyspace CacheKeyspace
	TTL      time.Duration
	Warm     bool
}

func DefaultCachePolicies() []CachePolicy {
	return []CachePolicy{
		{Keyspace: CacheStrategyRankings, TTL: 30 * time.Second, Warm: true},
		{Keyspace: CacheResearchResults, TTL: 10 * time.Minute, Warm: true},
		{Keyspace: CachePortfolioMetrics, TTL: 2 * time.Second, Warm: true},
		{Keyspace: CacheMarketState, TTL: 1 * time.Second, Warm: true},
		{Keyspace: CacheAIDecisions, TTL: 10 * time.Second, Warm: false},
		{Keyspace: CacheRiskMetrics, TTL: 2 * time.Second, Warm: true},
		{Keyspace: CacheDashboardViews, TTL: 5 * time.Second, Warm: true},
	}
}
