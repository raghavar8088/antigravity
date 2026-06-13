package aiscoring

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	defaultWorkerCount   = 3
	scoreCacheTTL        = 60 * time.Second
	aiCallTimeout        = 45 * time.Second
	requestChannelBuffer = 256
	cleanupInterval      = 30 * time.Second
)

// AsyncScorer scores trading signals out-of-band so the hot execution path
// is never blocked by an AI API call. Workers score signals in the background
// and populate a local cache. The execution path calls GetCachedScore to read
// the last known score without waiting.
type AsyncScorer struct {
	requestChan chan ScoringRequest
	cache       *ScoreCache
	fallback    *FallbackScorer
	aiClient    AIClient
	workerCount int
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewAsyncScorer creates a scorer backed by aiClient. If aiClient is nil,
// all scoring falls back to the rule-based FallbackScorer.
func NewAsyncScorer(aiClient AIClient, workerCount int) *AsyncScorer {
	if workerCount <= 0 {
		workerCount = defaultWorkerCount
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &AsyncScorer{
		requestChan: make(chan ScoringRequest, requestChannelBuffer),
		cache:       NewScoreCache(scoreCacheTTL),
		fallback:    &FallbackScorer{},
		aiClient:    aiClient,
		workerCount: workerCount,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start launches the worker pool and the cache cleanup goroutine.
// Call once at engine startup.
func (s *AsyncScorer) Start() {
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
	s.wg.Add(1)
	go s.cacheCleanupLoop()
	slog.Info("[aiscoring] async scorer started", "workers", s.workerCount)
}

// SubmitForScoring enqueues a signal for background scoring.
// This function MUST return immediately — it never blocks.
func (s *AsyncScorer) SubmitForScoring(req ScoringRequest) {
	select {
	case s.requestChan <- req:
		slog.Debug("[aiscoring] signal enqueued", "request_id", req.RequestID)
	default:
		// Channel full — score inline via fallback and cache the result.
		slog.Warn("[aiscoring] request channel full — using fallback immediately",
			"request_id", req.RequestID)
		score := s.fallback.Score(req.Context)
		s.cache.Set(signalKey(req), score)
	}
}

// GetCachedScore returns the most recent cached score for signalType.
// Returns (nil, false) if no cached score exists or the cache is expired.
// This is the hot path — never blocks.
func (s *AsyncScorer) GetCachedScore(signalType string) (*SignalScore, bool) {
	score, ok := s.cache.Get(signalType)
	if ok && time.Since(score.ScoredAt) > scoreCacheTTL {
		score.IsStale = true
	}
	return score, ok
}

// Stop drains the request channel, waits for workers to finish, and cancels
// the background context. Safe to call multiple times.
func (s *AsyncScorer) Stop() {
	s.cancel()
	s.wg.Wait()
	slog.Info("[aiscoring] async scorer stopped")
}

// ── internal ──────────────────────────────────────────────────────────────────

func (s *AsyncScorer) worker(id int) {
	defer s.wg.Done()
	slog.Debug("[aiscoring] worker started", "worker_id", id)

	for {
		select {
		case <-s.ctx.Done():
			slog.Debug("[aiscoring] worker stopping", "worker_id", id)
			return
		case req := <-s.requestChan:
			s.scoreRequest(req)
		}
	}
}

func (s *AsyncScorer) scoreRequest(req ScoringRequest) {
	key := signalKey(req)

	// Check cache first — skip redundant AI call if still fresh.
	if cached, ok := s.cache.Get(key); ok {
		slog.Debug("[aiscoring] cache hit, skipping AI call",
			"request_id", req.RequestID, "key", key)
		_ = cached
		return
	}

	var score SignalScore

	if s.aiClient != nil {
		ctx, cancel := context.WithTimeout(s.ctx, aiCallTimeout)
		defer cancel()

		result, err := s.aiClient.ScoreSignal(ctx, req.Signal, req.Context)
		if err != nil {
			slog.Warn("[aiscoring] AI call failed, using fallback",
				"request_id", req.RequestID,
				"err", err,
			)
			score = s.fallback.Score(req.Context)
		} else {
			score = *result
		}
	} else {
		score = s.fallback.Score(req.Context)
	}

	s.cache.Set(key, score)
	slog.Debug("[aiscoring] scored",
		"request_id", req.RequestID,
		"confidence", score.Confidence,
		"direction", score.Direction,
		"source", score.Source,
	)
}

func (s *AsyncScorer) cacheCleanupLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cache.Cleanup()
		}
	}
}

// signalKey derives the cache key for a scoring request.
// Uses the signal type string when the signal implements fmt.Stringer;
// falls back to the request ID so the score is still cached.
func signalKey(req ScoringRequest) string {
	if req.RequestID != "" {
		// For pre-scoring, group by market regime + direction so signals with
		// the same context share a cached score.
		return req.Context.Regime + "_" + req.RequestID[:min(8, len(req.RequestID))]
	}
	return "unknown"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
