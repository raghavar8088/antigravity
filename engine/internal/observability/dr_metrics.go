package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────
// Disaster Recovery Metrics
// SLO: RPO < 5 minutes, RTO < 15 minutes
// ─────────────────────────────────────────────

var (
	// BackupAge measures the age of the most recent successful backup.
	// Alert when > 24h for cold backups, > 1h for hot backups.
	BackupAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "backup_age_seconds",
		Help:      "Age of the most recent successful backup in seconds. Alert if >3600.",
	}, []string{"db", "backup_type"})

	// BackupSuccess counts successful backup completions.
	BackupSuccess = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "backup_success_total",
		Help:      "Total successful backup operations per database and backup type.",
	}, []string{"db", "backup_type"})

	// BackupFailures counts backup failures.
	BackupFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "backup_failures_total",
		Help:      "Total backup failures per database and backup type.",
	}, []string{"db", "backup_type"})

	// BackupSizeBytes records the size of the latest backup.
	BackupSizeBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "backup_size_bytes",
		Help:      "Size of the most recent backup in bytes.",
	}, []string{"db", "backup_type"})

	// SnapshotAge measures the age of the most recent event store snapshot.
	// Alert when > RPO threshold (300 seconds).
	SnapshotAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "snapshot_age_seconds",
		Help:      "Age of the most recent event store snapshot in seconds. RPO target: <300s.",
	}, []string{"aggregate_type"})

	// SnapshotSuccess counts successful snapshot creations.
	SnapshotSuccess = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "snapshot_success_total",
		Help:      "Total successful event store snapshots per aggregate type.",
	}, []string{"aggregate_type"})

	// SnapshotFailures counts snapshot failures.
	SnapshotFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "snapshot_failures_total",
		Help:      "Total event store snapshot failures per aggregate type.",
	}, []string{"aggregate_type"})

	// ReplayRecoveryTime measures the time to replay events to restore state.
	// Target: RTO < 15 minutes (900 seconds).
	ReplayRecoveryTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "replay_recovery_seconds",
		Help:      "Event replay recovery duration in seconds. RTO target: <900s.",
		Buckets:   []float64{30, 60, 120, 180, 300, 450, 600, 750, 900, 1200},
	}, []string{"aggregate_type"})

	// RPOEstimate is the current estimated recovery point objective in seconds.
	RPOEstimate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "rpo_estimate_seconds",
		Help:      "Current estimated RPO in seconds (data loss window). Target: <300s.",
	}, []string{"component"})

	// RTOEstimate is the current estimated recovery time objective in seconds.
	RTOEstimate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "rto_estimate_seconds",
		Help:      "Current estimated RTO in seconds (downtime window). Target: <900s.",
	}, []string{"component"})

	// DRReadiness is 1 if all DR checks pass (backup fresh, snapshots healthy, replay tested).
	DRReadiness = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "readiness_score",
		Help:      "DR readiness score: 1.0=fully ready, 0.5=degraded, 0.0=not ready.",
	})

	// LastDRTest is the Unix timestamp of the last successful DR drill.
	LastDRTest = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "last_drill_unix",
		Help:      "Unix timestamp of the last successful DR drill. Alert if >7 days ago.",
	})

	// WALLag measures PostgreSQL WAL replication lag for the hot standby.
	WALLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "dr",
		Name:      "wal_lag_bytes",
		Help:      "PostgreSQL WAL replication lag in bytes for each standby.",
	}, []string{"standby"})
)

// ─────────────────────────────────────────────
// DR Reporter — helper for periodic DR status updates
// ─────────────────────────────────────────────

// DRStatus encapsulates the current DR health.
type DRStatus struct {
	BackupsHealthy   bool
	SnapshotsHealthy bool
	ReplayTested     bool
	RPOSeconds       float64
	RTOSeconds       float64
	LastDrillAt      time.Time
}

// RecordDRStatus updates all DR Prometheus metrics from a DRStatus snapshot.
func RecordDRStatus(ctx context.Context, status DRStatus) {
	// RPO/RTO estimates.
	RPOEstimate.WithLabelValues("event_store").Set(status.RPOSeconds)
	RTOEstimate.WithLabelValues("trading_engine").Set(status.RTOSeconds)

	// Readiness score.
	score := 0.0
	passing := 0
	total := 3
	if status.BackupsHealthy {
		passing++
	}
	if status.SnapshotsHealthy {
		passing++
	}
	if status.ReplayTested {
		passing++
	}
	switch passing {
	case 3:
		score = 1.0
	case 2:
		score = 0.5
	}
	DRReadiness.Set(score)

	if !status.LastDrillAt.IsZero() {
		LastDRTest.Set(float64(status.LastDrillAt.Unix()))
	}

	// Log DR status.
	Logger.Log(ctx, LevelInfo, "DR status updated",
		slog.String("event_type", "DR_STATUS"),
		slog.Float64("rpo_seconds", status.RPOSeconds),
		slog.Float64("rto_seconds", status.RTOSeconds),
		slog.Float64("readiness_score", score),
		slog.Int("checks_passing", passing),
		slog.Int("checks_total", total),
	)

	// Alert if readiness is degraded.
	if score < 1.0 {
		Logger.Log(ctx, LevelWarn, "DR readiness degraded",
			slog.String("event_type", "DR_READINESS_DEGRADED"),
			slog.Float64("score", score),
			slog.Bool("backups_healthy", status.BackupsHealthy),
			slog.Bool("snapshots_healthy", status.SnapshotsHealthy),
			slog.Bool("replay_tested", status.ReplayTested),
		)
	}
}

// RecordSnapshotCreated updates snapshot metrics after a successful snapshot.
func RecordSnapshotCreated(aggregateType string, sizeBytes int64) {
	SnapshotSuccess.WithLabelValues(aggregateType).Inc()
	SnapshotAge.WithLabelValues(aggregateType).Set(0) // just created
	BackupSizeBytes.WithLabelValues("event_store", aggregateType).Set(float64(sizeBytes))
}

// RecordBackupCompleted records a backup completion.
func RecordBackupCompleted(db, backupType string, sizeBytes int64, success bool) {
	if success {
		BackupSuccess.WithLabelValues(db, backupType).Inc()
		BackupAge.WithLabelValues(db, backupType).Set(0) // just completed
		BackupSizeBytes.WithLabelValues(db, backupType).Set(float64(sizeBytes))
	} else {
		BackupFailures.WithLabelValues(db, backupType).Inc()
	}
}
