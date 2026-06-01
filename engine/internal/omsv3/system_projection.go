package omsv3

import (
	"encoding/json"
	"sort"
	"time"

	"antigravity-engine/internal/ledger"
)

// EngineBootRecord captures one boot cycle (start → stop) in the engine's
// operational history. Rebuilt from ENGINE_STARTED / ENGINE_STOPPED pairs.
type EngineBootRecord struct {
	StartedAt  time.Time     `json:"started_at"`
	StoppedAt  time.Time     `json:"stopped_at,omitempty"`
	Uptime     time.Duration `json:"uptime_ns,omitempty"`
	Version    string        `json:"version,omitempty"`
	StopReason string        `json:"stop_reason,omitempty"`
}

// SystemHistoryProjection is the complete operational timeline for the engine,
// rebuilt deterministically from SYSTEM ledger events.
type SystemHistoryProjection struct {
	CurrentState string `json:"current_state"`
	Version      string `json:"version"`

	TotalBoots        int    `json:"total_boots"`
	TotalReplays      int    `json:"total_replays"`
	TotalSnapshots    int    `json:"total_snapshots"`
	LastSnapshotID    string `json:"last_snapshot_id,omitempty"`
	KillSwitchActive  bool   `json:"kill_switch_active"`

	// Exchange connectivity per exchange
	Exchanges map[string]*ExchangeHealthProjection `json:"exchanges"`

	// Boot history (most recent 10)
	Boots []EngineBootRecord `json:"boots"`
}

// ExchangeHealthProjection is per-exchange connectivity state.
type ExchangeHealthProjection struct {
	Exchange        string `json:"exchange"`
	Connected       bool   `json:"connected"`
	DisconnectCount int    `json:"disconnect_count"`
	RateLimitHits   int    `json:"rate_limit_hits"`
	DataGaps        int    `json:"data_gaps"`
	DataStale       bool   `json:"data_stale"`
}

// BuildSystemHistoryProjection performs a single-pass scan of all account events
// and reconstructs the full engine operational history.
func BuildSystemHistoryProjection(events []ledger.Event) SystemHistoryProjection {
	proj := SystemHistoryProjection{
		CurrentState: "UNKNOWN",
		Exchanges:    make(map[string]*ExchangeHealthProjection),
	}

	var currentBoot *EngineBootRecord

	for _, e := range events {
		switch e.AggregateType {
		case ledger.AggregateSystem:
			var payload ledger.SystemLifecyclePayload
			if len(e.Payload) > 0 {
				_ = json.Unmarshal(e.Payload, &payload)
			}

			switch e.EventType {
			case ledger.EventEngineStarting:
				proj.CurrentState = "STARTING"
				currentBoot = &EngineBootRecord{StartedAt: e.CreatedAt}
				if payload.Version != "" {
					proj.Version = payload.Version
					currentBoot.Version = payload.Version
				}

			case ledger.EventEngineStarted:
				proj.CurrentState = "RUNNING"
				proj.TotalBoots++
				if currentBoot == nil {
					currentBoot = &EngineBootRecord{StartedAt: e.CreatedAt}
				}
				if payload.Version != "" {
					proj.Version = payload.Version
				}

			case ledger.EventEngineStopping:
				proj.CurrentState = "STOPPING"

			case ledger.EventEngineStopped:
				proj.CurrentState = "STOPPED"
				if currentBoot != nil {
					currentBoot.StoppedAt = e.CreatedAt
					currentBoot.Uptime = e.CreatedAt.Sub(currentBoot.StartedAt)
					currentBoot.StopReason = payload.Reason
					proj.Boots = appendBoot(proj.Boots, *currentBoot, 10)
					currentBoot = nil
				}

			case ledger.EventReplayStarted:
				proj.CurrentState = "REPLAYING"
				proj.TotalReplays++

			case ledger.EventReplayCompleted:
				proj.CurrentState = "RUNNING"

			case ledger.EventSnapshotCreated:
				proj.TotalSnapshots++
				if payload.SnapshotID != "" {
					proj.LastSnapshotID = payload.SnapshotID
				}

			case ledger.EventSnapshotRestored:
				if payload.SnapshotID != "" {
					proj.LastSnapshotID = payload.SnapshotID
				}
			}

		case ledger.AggregateExchange:
			ex := e.AggregateID
			health, ok := proj.Exchanges[ex]
			if !ok {
				health = &ExchangeHealthProjection{Exchange: ex}
				proj.Exchanges[ex] = health
			}
			switch e.EventType {
			case ledger.EventExchangeConnected, ledger.EventExchangeReconnected:
				health.Connected = true
				health.DataStale = false
			case ledger.EventExchangeDisconnected, ledger.EventExchangeOutage:
				health.Connected = false
				health.DisconnectCount++
			case ledger.EventExchangeRateLimitHit:
				health.RateLimitHits++
			case ledger.EventExchangeDataGapDetected:
				health.DataGaps++
			case ledger.EventMarketDataStale:
				health.DataStale = true
			case ledger.EventMarketDataRecovered:
				health.DataStale = false
			}

		case ledger.AggregateRisk:
			switch e.EventType {
			case ledger.EventKillSwitchTriggered:
				proj.KillSwitchActive = true
			case ledger.EventKillSwitchReleased:
				proj.KillSwitchActive = false
			}
		}
	}

	return proj
}

func appendBoot(boots []EngineBootRecord, boot EngineBootRecord, maxLen int) []EngineBootRecord {
	boots = append(boots, boot)
	if len(boots) > maxLen {
		boots = boots[len(boots)-maxLen:]
	}
	// Sort descending so most recent is first.
	sort.Slice(boots, func(i, j int) bool {
		return boots[i].StartedAt.After(boots[j].StartedAt)
	})
	return boots
}

// ConnectedExchangeNames returns the list of exchanges currently showing as connected.
func (p *SystemHistoryProjection) ConnectedExchangeNames() []string {
	var names []string
	for name, h := range p.Exchanges {
		if h.Connected {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
