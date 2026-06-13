package mocks

import (
	"fmt"
	"sync"
	"time"
)

// MockMongoDB is an in-memory MongoDB substitute for integration tests.
// It stores documents as map[string]interface{} keyed by a string ID.
type MockMongoDB struct {
	mu       sync.RWMutex
	trades   map[string]interface{}
	lessons  map[string]interface{}
	sessions map[string]interface{}
	ForceErr error // set to inject errors
}

// NewMockMongoDB creates a ready-to-use MockMongoDB.
func NewMockMongoDB() *MockMongoDB {
	return &MockMongoDB{
		trades:   make(map[string]interface{}),
		lessons:  make(map[string]interface{}),
		sessions: make(map[string]interface{}),
	}
}

// InsertTrade inserts a trade document keyed by id.
func (m *MockMongoDB) InsertTrade(id string, doc interface{}) error {
	if m.ForceErr != nil {
		return m.ForceErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trades[id] = doc
	return nil
}

// FindTradeByID returns the trade document for the given id.
func (m *MockMongoDB) FindTradeByID(id string) (interface{}, error) {
	if m.ForceErr != nil {
		return nil, m.ForceErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	doc, ok := m.trades[id]
	if !ok {
		return nil, fmt.Errorf("trade %s not found", id)
	}
	return doc, nil
}

// FindTradesByStatus returns all trades with the given status field value.
func (m *MockMongoDB) FindTradesByStatus(status string) ([]interface{}, error) {
	if m.ForceErr != nil {
		return nil, m.ForceErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []interface{}
	for _, doc := range m.trades {
		if d, ok := doc.(map[string]interface{}); ok {
			if d["status"] == status {
				results = append(results, doc)
			}
		}
	}
	return results, nil
}

// UpdateTradeStatus updates the status field of a trade and applies extra fields.
func (m *MockMongoDB) UpdateTradeStatus(id, status string, extraFields map[string]interface{}) error {
	if m.ForceErr != nil {
		return m.ForceErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.trades[id]
	if !ok {
		return fmt.Errorf("trade %s not found", id)
	}
	if d, ok := doc.(map[string]interface{}); ok {
		d["status"] = status
		d["updated_at"] = time.Now()
		for k, v := range extraFields {
			d[k] = v
		}
	}
	return nil
}

// CountTrades returns the number of trade documents in the store.
func (m *MockMongoDB) CountTrades() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.trades)
}

// Reset clears all collections and resets the ForceErr field.
func (m *MockMongoDB) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trades = make(map[string]interface{})
	m.lessons = make(map[string]interface{})
	m.sessions = make(map[string]interface{})
	m.ForceErr = nil
}
