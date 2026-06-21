package config

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// AdminSecretHeader matches the header name already used by the Vercel proxy
// (client/src/app/api/engine/[...path]/route.ts forwards X-Engine-Admin-Secret
// for admin-tier paths) and by mock-trading/reset/route.ts. Reusing the exact
// same header keeps the operator auth story for this module consistent with
// every other admin endpoint in the system.
const AdminSecretHeader = "X-Engine-Admin-Secret"

// ColConfigChanges is the MongoDB collection name for the audit trail of
// every threshold edit made through this module.
const ColConfigChanges = "config_changes"

// ConfigChangeEntry is one row of the audit trail, persisted to MongoDB
// (collection config_changes) and returned by GET /api/engine/config/history.
type ConfigChangeEntry struct {
	ID              string    `json:"id" bson:"_id,omitempty"`
	Key             string    `json:"key" bson:"key"`
	OldValue        float64   `json:"oldValue" bson:"old_value"`
	NewValue        float64   `json:"newValue" bson:"new_value"`
	ChangedAt       time.Time `json:"changedAt" bson:"changed_at"`
	ChangedBy       string    `json:"changedBy" bson:"changed_by"`
	RequiresRestart bool      `json:"requiresRestart" bson:"requires_restart"`
	Reason          string    `json:"reason,omitempty" bson:"reason,omitempty"`
}

// auditStore is the minimal interface the HTTP handlers need for persisting
// and reading the audit trail. mongoAuditStore is used when MONGODB_URI is
// configured; memoryAuditStore is an in-process fallback so the module still
// works (without surviving a restart) when Mongo is unavailable — matching
// the engine's existing "degrade gracefully" persistence pattern.
type auditStore interface {
	Append(ctx context.Context, entry ConfigChangeEntry) error
	List(ctx context.Context, key string, limit int) ([]ConfigChangeEntry, error)
}

type memoryAuditStore struct {
	mu      sync.Mutex
	entries []ConfigChangeEntry
}

func (m *memoryAuditStore) Append(_ context.Context, entry ConfigChangeEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
	return nil
}

func (m *memoryAuditStore) List(_ context.Context, key string, limit int) ([]ConfigChangeEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ConfigChangeEntry, 0, len(m.entries))
	for _, e := range m.entries {
		if key != "" && e.Key != key {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChangedAt.After(out[j].ChangedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type mongoAuditStore struct {
	col *mongo.Collection
}

func (m *mongoAuditStore) Append(ctx context.Context, entry ConfigChangeEntry) error {
	entry.ID = ""
	_, err := m.col.InsertOne(ctx, entry)
	return err
}

func (m *mongoAuditStore) List(ctx context.Context, key string, limit int) ([]ConfigChangeEntry, error) {
	filter := bson.M{}
	if key != "" {
		filter["key"] = key
	}
	if limit <= 0 {
		limit = 50
	}
	opts := options.Find().SetSort(bson.D{{Key: "changed_at", Value: -1}}).SetLimit(int64(limit))
	cur, err := m.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []ConfigChangeEntry
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

var (
	auditStoreOnce sync.Once
	auditStoreInst auditStore
)

// getAuditStore lazily connects to MongoDB (MONGODB_URI) for the
// config_changes audit collection, falling back to an in-memory store (with
// a one-time warning) if Mongo is not configured or unreachable. This
// mirrors the degrade-gracefully pattern already used for paper-trading
// persistence in cmd/antigravity/main.go.
func getAuditStore() auditStore {
	auditStoreOnce.Do(func() {
		uri := os.Getenv("MONGODB_URI")
		if uri == "" {
			log.Printf("[CONFIG] MONGODB_URI not set — config change audit log is in-memory only (lost on restart)")
			auditStoreInst = &memoryAuditStore{}
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		dbName := os.Getenv("MONGODB_DB")
		if dbName == "" {
			dbName = "loop_trades"
		}
		clientOpts := options.Client().ApplyURI(uri).SetServerSelectionTimeout(5 * time.Second)
		mc, err := mongo.Connect(clientOpts)
		if err != nil {
			log.Printf("[CONFIG] WARNING: mongo connect failed for config audit log: %v — falling back to in-memory", err)
			auditStoreInst = &memoryAuditStore{}
			return
		}
		if err := mc.Ping(ctx, nil); err != nil {
			log.Printf("[CONFIG] WARNING: mongo ping failed for config audit log: %v — falling back to in-memory", err)
			auditStoreInst = &memoryAuditStore{}
			return
		}
		col := mc.Database(dbName).Collection(ColConfigChanges)
		_, _ = col.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "key", Value: 1}, {Key: "changed_at", Value: -1}}})
		_, _ = col.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "changed_at", Value: -1}}})
		log.Printf("[CONFIG] connected config_changes audit collection (db=%s)", dbName)
		auditStoreInst = &mongoAuditStore{col: col}
	})
	return auditStoreInst
}

// ── Wire response types ─────────────────────────────────────────────────────

// ThresholdConfigDTO is the full payload returned by GET /api/engine/config
// for one threshold — static metadata merged with the live registry value.
type ThresholdConfigDTO struct {
	Key             string  `json:"key"`
	Category        string  `json:"category"`
	Label           string  `json:"label"`
	Description     string  `json:"description"`
	DataType        string  `json:"dataType"`
	Unit            string  `json:"unit"`
	CurrentValue    float64 `json:"currentValue"`
	DefaultValue    float64 `json:"defaultValue"`
	Min             float64 `json:"min"`
	Max             float64 `json:"max"`
	RecommendedMin  float64 `json:"recommendedMin"`
	RecommendedMax  float64 `json:"recommendedMax"`
	RiskLevel       string  `json:"riskLevel"`
	WhatIfIncreased string  `json:"whatHappensIfIncreased"`
	WhatIfDecreased string  `json:"whatHappensIfDecreased"`
	RequiresRestart bool    `json:"requiresRestart"`
	Source          string  `json:"source"`
	UpdatedAt       string  `json:"lastChangedAt,omitempty"`
}

func dtoFor(m Meta, v ThresholdValue) ThresholdConfigDTO {
	dto := ThresholdConfigDTO{
		Key: m.Key, Category: m.Category, Label: m.Label, Description: m.Description,
		DataType: m.DataType, Unit: m.Unit,
		CurrentValue: v.CurrentValue, DefaultValue: v.DefaultValue,
		Min: m.Min, Max: m.Max, RecommendedMin: m.RecommendedMin, RecommendedMax: m.RecommendedMax,
		RiskLevel: m.RiskLevel, WhatIfIncreased: m.WhatIfIncreased, WhatIfDecreased: m.WhatIfDecreased,
		RequiresRestart: m.RequiresRestart, Source: v.Source,
	}
	if !v.UpdatedAt.IsZero() {
		dto.UpdatedAt = v.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return dto
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func checkAdminSecret(r *http.Request) bool {
	expected := os.Getenv("ENGINE_ADMIN_SECRET")
	if expected == "" {
		// Defense-in-depth default: if not configured, deny mutating calls
		// rather than silently allow (the read-only GET path does not call
		// this check at all).
		return false
	}
	provided := r.Header.Get(AdminSecretHeader)
	if len(provided) != len(expected) {
		return false
	}
	diff := byte(0)
	for i := 0; i < len(provided); i++ {
		diff |= provided[i] ^ expected[i]
	}
	return diff == 0
}

// HandleGetConfig serves GET /api/engine/config — full ThresholdConfigDTO list.
func HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	reg := Default()
	values := reg.All()
	metas := reg.AllMeta()

	out := make([]ThresholdConfigDTO, 0, len(metas))
	for key, m := range metas {
		v, ok := values[key]
		if !ok {
			v = ThresholdValue{Key: key, CurrentValue: defaultValueOf(m), DefaultValue: defaultValueOf(m), Source: "default"}
		}
		out = append(out, dtoFor(m, v))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Key < out[j].Key
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "thresholds": out})
}

// setRequestBody is the POST /api/engine/config request payload.
type setRequestBody struct {
	Key    string  `json:"key"`
	Value  float64 `json:"value"`
	Reason string  `json:"reason,omitempty"`
}

// HandleSetConfig serves POST /api/engine/config — validates [min,max],
// applies the change immediately via the registry, appends an audit log
// entry, and returns the updated value plus restart-needed flag. Out-of-
// range values are rejected with a clear error and never silently clamped.
func HandleSetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if !checkAdminSecret(r) {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"ok": false, "error": "admin secret required"})
		return
	}

	var body setRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "invalid JSON body"})
		return
	}
	if body.Key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "key is required"})
		return
	}

	reg := Default()
	oldValue := reg.Get(body.Key)
	m, hasMeta := reg.GetMeta(body.Key)

	updated, err := reg.Set(body.Key, body.Value, "runtime_override")
	if err != nil {
		switch e := err.(type) {
		case ErrUnknownKey:
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"ok": false, "error": e.Error()})
		case ErrOutOfRange:
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"ok": false, "error": e.Error(), "min": e.Min, "max": e.Max,
			})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"ok": false, "error": err.Error()})
		}
		return
	}

	changedBy := r.Header.Get("X-Service-Name")
	if changedBy == "" {
		changedBy = "operator"
	}

	entry := ConfigChangeEntry{
		Key: body.Key, OldValue: oldValue, NewValue: body.Value,
		ChangedAt: time.Now().UTC(), ChangedBy: changedBy,
		RequiresRestart: hasMeta && m.RequiresRestart, Reason: body.Reason,
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := getAuditStore().Append(ctx, entry); err != nil {
		log.Printf("[CONFIG] WARNING: failed to write audit log entry for %s: %v", body.Key, err)
	}

	// Hot-reload integration points: refresh the in-process package-level
	// vars that read through the registry so the change is effective on the
	// next evaluation cycle for non-restart-tier keys. These calls are no-ops
	// for keys outside their respective package's hot set.
	RefreshHotReloadHooks()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":              true,
		"threshold":       dtoFor(m, updated),
		"requiresRestart": hasMeta && m.RequiresRestart,
	})
}

// HandleConfigHistory serves GET /api/engine/config/history?key=X&limit=N —
// the audit trail for one threshold (or all thresholds if key is omitted),
// last 50 by default, reverse chronological.
func HandleConfigHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	key := r.URL.Query().Get("key")
	limit := 50

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	entries, err := getAuditStore().List(ctx, key, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	if entries == nil {
		entries = []ConfigChangeEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "history": entries})
}

// hotReloadHooks are registered by other packages (internal/trading,
// internal/strategy/scalpers) at init time via RegisterHotReloadHook so this
// package does not need an import cycle back to them. Called after every
// successful Set().
var (
	hotReloadHooksMu sync.Mutex
	hotReloadHooks   []func()
)

// RegisterHotReloadHook adds a callback invoked after every successful
// registry Set(). Used by internal/trading.RefreshThresholdsFromRegistry and
// internal/strategy/scalpers.RefreshWalkForwardThresholdsFromRegistry so an
// operator edit takes effect immediately without this package importing
// theirs (which would create an import cycle, since both already import
// internal/config).
func RegisterHotReloadHook(fn func()) {
	hotReloadHooksMu.Lock()
	defer hotReloadHooksMu.Unlock()
	hotReloadHooks = append(hotReloadHooks, fn)
}

// RefreshHotReloadHooks invokes every registered hot-reload hook.
func RefreshHotReloadHooks() {
	hotReloadHooksMu.Lock()
	hooks := make([]func(), len(hotReloadHooks))
	copy(hooks, hotReloadHooks)
	hotReloadHooksMu.Unlock()
	for _, fn := range hooks {
		fn()
	}
}
