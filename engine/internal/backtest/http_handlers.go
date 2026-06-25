package backtest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// BacktestAPI wires 7 HTTP endpoints for the backtest system.
// Wire it via: api := backtest.NewBacktestAPI(...); api.Register(mux)
type BacktestAPI struct {
	jobs     *JobStore
	store    *ResultsStore
	cacheDir string

	// promotionFn, if non-nil, is called with a strategy name when a promotion
	// request arrives — allows the caller to write to ScalerBundle.promotionCh.
	promotionFn func(strategyName string)
}

// NewBacktestAPI creates the API with a pre-opened ResultsStore.
func NewBacktestAPI(store *ResultsStore, cacheDir string, promotionFn func(string)) *BacktestAPI {
	return &BacktestAPI{
		jobs:        NewJobStore(),
		store:       store,
		cacheDir:    cacheDir,
		promotionFn: promotionFn,
	}
}

// Register wires all 7 routes on the default http.ServeMux.
func (a *BacktestAPI) Register() {
	http.HandleFunc("/api/backtest/run", a.handleRun)
	http.HandleFunc("/api/backtest/status/", a.handleStatus)
	http.HandleFunc("/api/backtest/jobs", a.handleListJobs)
	http.HandleFunc("/api/backtest/results/", a.handleResults)
	http.HandleFunc("/api/backtest/leaderboard", a.handleLeaderboard)
	http.HandleFunc("/api/backtest/strategies", a.handleStrategies)
	http.HandleFunc("/api/backtest/promote/", a.handlePromote)
}

// ── POST /api/backtest/run ────────────────────────────────────────────────────

type runRequest struct {
	Symbol     string   `json:"symbol"`
	From       string   `json:"from"` // YYYY-MM-DD
	To         string   `json:"to"`
	Strategies []string `json:"strategies"` // empty = all ported
}

func (a *BacktestAPI) handleRun(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Symbol == "" {
		req.Symbol = "BTCUSDT"
	}
	from, err := time.Parse("2006-01-02", req.From)
	if err != nil {
		jsonError(w, "invalid from date: "+err.Error(), http.StatusBadRequest)
		return
	}
	to := time.Now().UTC().Truncate(24 * time.Hour)
	if req.To != "" {
		if to, err = time.Parse("2006-01-02", req.To); err != nil {
			jsonError(w, "invalid to date: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	job := a.jobs.Create(req.Symbol, from, to, req.Strategies)
	go a.runJob(job)

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(job)
}

// ── GET /api/backtest/status/{jobID} ─────────────────────────────────────────

func (a *BacktestAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	id := strings.TrimPrefix(r.URL.Path, "/api/backtest/status/")
	job, ok := a.jobs.Get(id)
	if !ok {
		jsonError(w, "job not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(job)
}

// ── GET /api/backtest/jobs ────────────────────────────────────────────────────

func (a *BacktestAPI) handleListJobs(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	_ = json.NewEncoder(w).Encode(a.jobs.List())
}

// ── GET /api/backtest/results/{runID} ────────────────────────────────────────

func (a *BacktestAPI) handleResults(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	runID := strings.TrimPrefix(r.URL.Path, "/api/backtest/results/")
	if runID == "" {
		jsonError(w, "runID required", http.StatusBadRequest)
		return
	}
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	results, err := a.store.Leaderboard(runID, symbol, 200)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(results)
}

// ── GET /api/backtest/leaderboard ────────────────────────────────────────────

func (a *BacktestAPI) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		// pick latest
		runs, err := a.store.AllRuns()
		if err != nil || len(runs) == 0 {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		runID = runs[0]
	}
	results, err := a.store.Leaderboard(runID, symbol, 50)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(results)
}

// ── GET /api/backtest/strategies ─────────────────────────────────────────────

func (a *BacktestAPI) handleStrategies(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	entries := scalers.BuildPortedStrategies()
	type stratInfo struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Regimes     []string `json:"regimes"`
		Timeframes  []string `json:"timeframes"`
	}
	out := make([]stratInfo, len(entries))
	for i, e := range entries {
		regimes := make([]string, len(e.Regimes))
		for j, rg := range e.Regimes {
			regimes[j] = string(rg)
		}
		out[i] = stratInfo{Name: e.Name, Description: e.Description, Regimes: regimes, Timeframes: e.Timeframes}
	}
	_ = json.NewEncoder(w).Encode(out)
}

// ── POST /api/backtest/promote/{strategyName} ─────────────────────────────────

func (a *BacktestAPI) handlePromote(w http.ResponseWriter, r *http.Request) {
	setCORSJSON(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/backtest/promote/")
	if name == "" {
		jsonError(w, "strategy name required", http.StatusBadRequest)
		return
	}
	p, exists := scalers.GetPerformance(name)
	if !exists {
		p = scalers.Performance{StrategyName: name}
	}
	p.Active = true
	scalers.UpdatePerformance(p)

	if a.promotionFn != nil {
		a.promotionFn(name)
	}

	log.Printf("[API] Promoted strategy %s to live via HTTP", name)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "strategy": name, "promoted": true})
}

// ── background job runner ─────────────────────────────────────────────────────

func (a *BacktestAPI) runJob(job *Job) {
	a.jobs.UpdateStatus(job.ID, JobStatusRunning, "")

	loader := marketdata.NewHistoricalLoader(a.cacheDir)
	ds, err := loader.BuildMTFDataset(job.Symbol, job.FromDate, job.ToDate)
	if err != nil {
		a.jobs.UpdateStatus(job.ID, JobStatusError, fmt.Sprintf("load dataset: %v", err))
		return
	}

	entries := scalers.BuildPortedStrategies()
	if len(job.Strategies) > 0 {
		filter := make(map[string]bool, len(job.Strategies))
		for _, n := range job.Strategies {
			filter[n] = true
		}
		filtered := entries[:0]
		for _, e := range entries {
			if filter[e.Name] {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	cfg := DefaultRunConfig(job.Symbol)
	batchRunner := NewBatchRunner(ds, cfg, entries)
	results := batchRunner.RunAll(func(done, total int, name string) {
		if total > 0 {
			a.jobs.UpdateProgress(job.ID, done*100/total)
		}
	})

	if err := a.store.SaveBatch(job.RunID, results); err != nil {
		a.jobs.UpdateStatus(job.ID, JobStatusError, fmt.Sprintf("save results: %v", err))
		return
	}

	// Update job with the runID for client to fetch results
	a.jobs.mu.Lock()
	if j, ok := a.jobs.jobs[job.ID]; ok {
		j.RunID = job.RunID
	}
	a.jobs.mu.Unlock()

	a.jobs.UpdateStatus(job.ID, JobStatusDone, "")
	log.Printf("[BACKTEST] job %s done: %d strategies on %s", job.ID, len(results), job.Symbol)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func setCORSJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// DBPath returns the default SQLite path for backtest results.
func DBPath(dataDir string) string {
	return filepath.Join(dataDir, "backtest_results.db")
}
