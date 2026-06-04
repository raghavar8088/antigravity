// Package datalake implements the Phase 19K Research Data Lake.
// Stores raw market data, features, signals, backtests, and ML datasets
// with immutable versioned storage and full lineage tracking.
// Research-only: production systems never write to this store.
package datalake

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ─── Dataset Types ────────────────────────────────────────────────────────────

type DatasetType string

const (
	TypeRawMarketData  DatasetType = "RAW_MARKET_DATA"
	TypeFeatures       DatasetType = "FEATURES"
	TypeSignals        DatasetType = "SIGNALS"
	TypeTrades         DatasetType = "TRADES"
	TypeBacktestResult DatasetType = "BACKTEST_RESULT"
	TypeMLDataset      DatasetType = "ML_DATASET"
	TypeResearchOutput DatasetType = "RESEARCH_OUTPUT"
)

// ─── Dataset Descriptor ───────────────────────────────────────────────────────

// Dataset is the metadata descriptor for one versioned dataset in the lake.
type Dataset struct {
	ID            string
	Name          string
	Type          DatasetType
	Version       int
	Description   string
	Symbol        string
	FromTime      time.Time
	ToTime        time.Time
	RecordCount   int64
	SizeBytes     int64
	Tags          []string
	ParentIDs     []string // datasets this was derived from
	FeatureIDs    []string // features used to produce this dataset
	ResearcherID  string
	ExperimentID  string
	SchemaVersion string
	Checksum      string // SHA-256 of serialised content
	Immutable     bool   // once true, cannot be overwritten
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DatasetVersion captures the version history of a dataset.
type DatasetVersion struct {
	DatasetID  string
	Version    int
	Descriptor Dataset
	ChangedBy  string
	ChangeNote string
	CreatedAt  time.Time
}

// ─── Data Lake ────────────────────────────────────────────────────────────────

// Lake is the research data lake: immutable, versioned, append-only storage.
type Lake struct {
	mu       sync.RWMutex
	datasets map[string][]DatasetVersion // datasetID → version history
	byType   map[DatasetType][]string    // type → []datasetIDs
	bySymbol map[string][]string         // symbol → []datasetIDs

	// In-memory payload store (production: replace with S3/GCS/blob storage).
	payloads map[string][]byte // datasetID:version → serialised payload
}

// NewLake creates an empty research data lake.
func NewLake() *Lake {
	return &Lake{
		datasets: make(map[string][]DatasetVersion),
		byType:   make(map[DatasetType][]string),
		bySymbol: make(map[string][]string),
		payloads: make(map[string][]byte),
	}
}

// Register adds a new dataset descriptor to the lake.
// Datasets are immutable once registered unless explicitly versioned.
func (l *Lake) Register(ctx context.Context, ds Dataset) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if ds.Name == "" {
		return "", errors.New("datalake: dataset name required")
	}
	if ds.Type == "" {
		return "", errors.New("datalake: dataset type required")
	}
	if ds.ID == "" {
		ds.ID = fmt.Sprintf("ds_%d", time.Now().UnixNano())
	}
	ds.Version = 1
	ds.Immutable = true
	ds.CreatedAt = time.Now().UTC()
	ds.UpdatedAt = ds.CreatedAt

	l.mu.Lock()
	defer l.mu.Unlock()

	if existing, ok := l.datasets[ds.ID]; ok && len(existing) > 0 {
		return "", fmt.Errorf("datalake: dataset %s already exists — use Version() to create a new version", ds.ID)
	}

	version := DatasetVersion{
		DatasetID: ds.ID, Version: 1, Descriptor: ds,
		CreatedAt: ds.CreatedAt,
	}
	l.datasets[ds.ID] = []DatasetVersion{version}
	l.byType[ds.Type] = appendUnique(l.byType[ds.Type], ds.ID)
	if ds.Symbol != "" {
		l.bySymbol[ds.Symbol] = appendUnique(l.bySymbol[ds.Symbol], ds.ID)
	}
	return ds.ID, nil
}

// Version creates a new version of an existing dataset.
// The previous version remains immutable and accessible.
func (l *Lake) Version(ctx context.Context, datasetID string, updated Dataset, changedBy, note string) (DatasetVersion, error) {
	if ctx.Err() != nil {
		return DatasetVersion{}, ctx.Err()
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	history, ok := l.datasets[datasetID]
	if !ok || len(history) == 0 {
		return DatasetVersion{}, fmt.Errorf("datalake: dataset %s not found", datasetID)
	}
	latest := history[len(history)-1]
	updated.ID = datasetID
	updated.Version = latest.Version + 1
	updated.CreatedAt = latest.Descriptor.CreatedAt
	updated.UpdatedAt = time.Now().UTC()
	updated.Immutable = true

	dv := DatasetVersion{
		DatasetID: datasetID, Version: updated.Version,
		Descriptor: updated, ChangedBy: changedBy,
		ChangeNote: note, CreatedAt: time.Now().UTC(),
	}
	l.datasets[datasetID] = append(history, dv)
	return dv, nil
}

// Store persists the raw payload for a dataset version.
// In production this writes to object storage (S3 / GCS).
func (l *Lake) Store(ctx context.Context, datasetID string, version int, payload []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := fmt.Sprintf("%s:%d", datasetID, version)
	l.payloads[key] = make([]byte, len(payload))
	copy(l.payloads[key], payload)
	return nil
}

// Load retrieves the raw payload for a dataset version.
func (l *Lake) Load(ctx context.Context, datasetID string, version int) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	key := fmt.Sprintf("%s:%d", datasetID, version)
	data, ok := l.payloads[key]
	if !ok {
		return nil, fmt.Errorf("datalake: payload not found for %s v%d", datasetID, version)
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

// GetLatest returns the latest version descriptor of a dataset.
func (l *Lake) GetLatest(datasetID string) (Dataset, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	history, ok := l.datasets[datasetID]
	if !ok || len(history) == 0 {
		return Dataset{}, fmt.Errorf("datalake: dataset %s not found", datasetID)
	}
	return history[len(history)-1].Descriptor, nil
}

// GetVersion returns a specific version of a dataset.
func (l *Lake) GetVersion(datasetID string, version int) (Dataset, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, dv := range l.datasets[datasetID] {
		if dv.Version == version {
			return dv.Descriptor, nil
		}
	}
	return Dataset{}, fmt.Errorf("datalake: version %d not found for dataset %s", version, datasetID)
}

// ListVersions returns all versions of a dataset.
func (l *Lake) ListVersions(datasetID string) []DatasetVersion {
	l.mu.RLock()
	defer l.mu.RUnlock()
	history := l.datasets[datasetID]
	out := make([]DatasetVersion, len(history))
	copy(out, history)
	return out
}

// ListByType returns the latest version of all datasets of a given type.
func (l *Lake) ListByType(t DatasetType) []Dataset {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []Dataset
	for _, id := range l.byType[t] {
		history := l.datasets[id]
		if len(history) > 0 {
			out = append(out, history[len(history)-1].Descriptor)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// ListBySymbol returns all datasets associated with a given trading symbol.
func (l *Lake) ListBySymbol(symbol string) []Dataset {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []Dataset
	for _, id := range l.bySymbol[symbol] {
		history := l.datasets[id]
		if len(history) > 0 {
			out = append(out, history[len(history)-1].Descriptor)
		}
	}
	return out
}

// Search finds datasets matching name, tag, or symbol substrings.
func (l *Lake) Search(query string) []Dataset {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []Dataset
	for _, history := range l.datasets {
		if len(history) == 0 {
			continue
		}
		ds := history[len(history)-1].Descriptor
		if containsSubstr(ds.Name, query) || containsSubstr(ds.Symbol, query) || tagsContain(ds.Tags, query) {
			out = append(out, ds)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// TotalDatasets returns the number of distinct datasets in the lake.
func (l *Lake) TotalDatasets() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.datasets)
}

// TotalVersions returns the total number of dataset versions stored.
func (l *Lake) TotalVersions() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	total := 0
	for _, h := range l.datasets {
		total += len(h)
	}
	return total
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func appendUnique(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}

func containsSubstr(s, sub string) bool {
	if sub == "" {
		return true
	}
	return len(s) >= len(sub) && containsRune(s, sub)
}

func containsRune(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func tagsContain(tags []string, query string) bool {
	for _, t := range tags {
		if containsRune(t, query) {
			return true
		}
	}
	return false
}
