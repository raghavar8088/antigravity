package funding

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FundingSnapshot struct {
	Exchange         string    `json:"exchange"`
	Symbol           string    `json:"symbol"`
	FundingRate      float64   `json:"fundingRate"`
	PredictedFunding float64   `json:"predictedFunding"`
	NextFundingTime  time.Time `json:"nextFundingTime"`
	Timestamp        time.Time `json:"timestamp"`
}

type Cache struct {
	mu      sync.RWMutex
	path    string
	history map[string][]FundingSnapshot
}

func NewCache(path string) (*Cache, error) {
	c := &Cache{path: path, history: make(map[string][]FundingSnapshot)}
	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		_ = c.Load()
	}
	return c, nil
}

func (c *Cache) Add(snapshot FundingSnapshot) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now().UTC()
	}
	key := snapshot.Exchange + ":" + snapshot.Symbol
	rows := append(c.history[key], snapshot)
	cutoff := snapshot.Timestamp.Add(-30 * 24 * time.Hour)
	start := 0
	for start < len(rows) && rows[start].Timestamp.Before(cutoff) {
		start++
	}
	c.history[key] = rows[start:]
	if c.path == "" {
		return nil
	}
	f, err := os.OpenFile(c.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(snapshot)
}

func (c *Cache) History(exchange, symbol string, window time.Duration) []FundingSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := exchange + ":" + symbol
	rows := c.history[key]
	if window <= 0 {
		return append([]FundingSnapshot(nil), rows...)
	}
	cutoff := time.Now().UTC().Add(-window)
	out := make([]FundingSnapshot, 0, len(rows))
	for _, row := range rows {
		if !row.Timestamp.Before(cutoff) {
			out = append(out, row)
		}
	}
	return out
}

func (c *Cache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, err := os.Open(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var row FundingSnapshot
		if err := json.Unmarshal(scanner.Bytes(), &row); err == nil {
			key := row.Exchange + ":" + row.Symbol
			c.history[key] = append(c.history[key], row)
		}
	}
	return scanner.Err()
}
