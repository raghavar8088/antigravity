package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OptionsBuyPaperPersistence is implemented by *Store (Postgres) and *FileSnapshotStore (JSON files).
type OptionsBuyPaperPersistence interface {
	SaveOptionsState(ctx context.Context, state *OptionsState) error
	LoadOptionsState(ctx context.Context) (*OptionsState, error)
}

// OptionsSellPaperPersistence is implemented by *Store and *FileSnapshotStore.
type OptionsSellPaperPersistence interface {
	SaveOptionsSellingState(ctx context.Context, state *OptionsSellingState) error
	LoadOptionsSellingState(ctx context.Context) (*OptionsSellingState, error)
}

var (
	_ OptionsBuyPaperPersistence  = (*Store)(nil)
	_ OptionsBuyPaperPersistence  = (*FileSnapshotStore)(nil)
	_ OptionsSellPaperPersistence = (*Store)(nil)
	_ OptionsSellPaperPersistence = (*FileSnapshotStore)(nil)
)

// FileSnapshotStore persists BTC options buy/sell snapshots as JSON files when
// DATABASE_URL is not configured. Set ENGINE_DATA_DIR to a mounted volume on
// Render/Fly so history survives redeploys.
type FileSnapshotStore struct {
	Dir string
}

// NewFileSnapshotStore creates the store and ensures the directory exists.
func NewFileSnapshotStore() (*FileSnapshotStore, error) {
	dir := strings.TrimSpace(os.Getenv("ENGINE_DATA_DIR"))
	if dir == "" {
		dir = ".engine-data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create engine data dir %q: %w", dir, err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return &FileSnapshotStore{Dir: abs}, nil
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func emptyOptionsStateFile() *OptionsState {
	return &OptionsState{
		Balance:    0,
		LastPrice:  0,
		LastMinute: 0,
		TradeSeq:   0,
		PriceHist:  json.RawMessage("[]"),
		MinuteBars: json.RawMessage("[]"),
		Trades:     json.RawMessage("[]"),
		Strategies: json.RawMessage("[]"),
		SavedAt:    time.Time{},
	}
}

func emptyOptionsSellingStateFile() *OptionsSellingState {
	return &OptionsSellingState{
		Balance:         0,
		DayStartBalance: 0,
		DayStartDate:    0,
		LastPrice:       0,
		LastMinute:      0,
		TradeSeq:        0,
		PriceHist:       json.RawMessage("[]"),
		MinuteBars:      json.RawMessage("[]"),
		Trades:          json.RawMessage("[]"),
		Strategies:      json.RawMessage("[]"),
		SavedAt:         time.Time{},
	}
}

// SaveOptionsState writes btc_options_buy.json.
func (f *FileSnapshotStore) SaveOptionsState(ctx context.Context, state *OptionsState) error {
	_ = ctx
	if f == nil {
		return fmt.Errorf("nil FileSnapshotStore")
	}
	return writeJSONAtomic(filepath.Join(f.Dir, "btc_options_buy.json"), state)
}

// LoadOptionsState reads btc_options_buy.json. Missing file means fresh account.
func (f *FileSnapshotStore) LoadOptionsState(ctx context.Context) (*OptionsState, error) {
	_ = ctx
	if f == nil {
		return nil, fmt.Errorf("nil FileSnapshotStore")
	}
	path := filepath.Join(f.Dir, "btc_options_buy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyOptionsStateFile(), nil
		}
		return nil, err
	}
	var st OptionsState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// SaveOptionsSellingState writes btc_options_sell.json.
func (f *FileSnapshotStore) SaveOptionsSellingState(ctx context.Context, state *OptionsSellingState) error {
	_ = ctx
	if f == nil {
		return fmt.Errorf("nil FileSnapshotStore")
	}
	return writeJSONAtomic(filepath.Join(f.Dir, "btc_options_sell.json"), state)
}

// LoadOptionsSellingState reads btc_options_sell.json.
func (f *FileSnapshotStore) LoadOptionsSellingState(ctx context.Context) (*OptionsSellingState, error) {
	_ = ctx
	if f == nil {
		return nil, fmt.Errorf("nil FileSnapshotStore")
	}
	path := filepath.Join(f.Dir, "btc_options_sell.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyOptionsSellingStateFile(), nil
		}
		return nil, err
	}
	var st OptionsSellingState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}
