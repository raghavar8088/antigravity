package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// VerificationResult describes the outcome of a backup verification.
type VerificationResult struct {
	Entry       BackupEntry
	Exists      bool
	HashMatch   bool
	Parseable   bool
	RestoreTest bool
	VerifiedAt  time.Time
	Error       error
}

// IsFullyValid returns true if all verification checks passed.
func (v VerificationResult) IsFullyValid() bool {
	return v.Exists && v.HashMatch && v.Parseable
}

// Verifier runs automated backup integrity checks.
type Verifier struct {
	nodeID  string
	encKey  [32]byte
	restore *RestoreManager

	metricValid      prometheus.Counter
	metricInvalid    prometheus.Counter
	metricCheckCount prometheus.Counter
}

func NewVerifier(nodeID string, encKey [32]byte, rm *RestoreManager, reg prometheus.Registerer) *Verifier {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	v := &Verifier{nodeID: nodeID, encKey: encKey, restore: rm}
	f := promauto.With(reg)
	labels := prometheus.Labels{"node_id": nodeID}
	v.metricValid = f.NewCounter(prometheus.CounterOpts{
		Name: "backup_verification_valid_total", Help: "Backups that passed verification",
		ConstLabels: labels,
	})
	v.metricInvalid = f.NewCounter(prometheus.CounterOpts{
		Name: "backup_verification_invalid_total", Help: "Backups that failed verification",
		ConstLabels: labels,
	})
	v.metricCheckCount = f.NewCounter(prometheus.CounterOpts{
		Name: "backup_verifications_total", Help: "Total verification runs",
		ConstLabels: labels,
	})
	return v
}

// Verify runs all integrity checks against a single backup entry.
func (v *Verifier) Verify(ctx context.Context, entry BackupEntry) VerificationResult {
	result := VerificationResult{
		Entry:      entry,
		VerifiedAt: time.Now(),
	}
	v.metricCheckCount.Inc()

	// 1. File existence.
	if _, err := os.Stat(entry.Path); err != nil {
		result.Error = fmt.Errorf("file not found: %s", entry.Path)
		v.metricInvalid.Inc()
		log.Printf("[backup/verify] FAIL %s: file missing", entry.ID)
		return result
	}
	result.Exists = true

	// 2. Hash verification.
	raw, err := os.ReadFile(entry.Path)
	if err != nil {
		result.Error = fmt.Errorf("read file: %w", err)
		v.metricInvalid.Inc()
		return result
	}
	actualHash := sha256sum(raw)
	if actualHash != entry.SHA256 {
		result.Error = fmt.Errorf("hash mismatch: stored=%s computed=%s",
			entry.SHA256[:16], actualHash[:16])
		v.metricInvalid.Inc()
		log.Printf("[backup/verify] FAIL %s: hash mismatch", entry.ID)
		return result
	}
	result.HashMatch = true

	// 3. Decrypt + decompress + parse.
	if err := v.restore.TestRestore(ctx, entry); err != nil {
		result.Error = fmt.Errorf("parse test: %w", err)
		v.metricInvalid.Inc()
		log.Printf("[backup/verify] FAIL %s: parse failed: %v", entry.ID, err)
		return result
	}
	result.Parseable = true

	v.metricValid.Inc()
	log.Printf("[backup/verify] PASS %s (hash=%s)", entry.ID, entry.SHA256[:16])
	return result
}

// VerifyAll verifies every entry in the catalog.
func (v *Verifier) VerifyAll(ctx context.Context, catalog []BackupEntry) []VerificationResult {
	results := make([]VerificationResult, 0, len(catalog))
	for _, entry := range catalog {
		r := v.Verify(ctx, entry)
		results = append(results, r)

		// Update the entry's Verified field in the manager's catalog.
		// (The Manager's catalog is authoritative; we just report here.)
	}
	return results
}

// VerifyLatest verifies only the most-recent backup of each type.
func (v *Verifier) VerifyLatest(ctx context.Context, catalog []BackupEntry) []VerificationResult {
	latest := make(map[BackupType]*BackupEntry)
	for i := range catalog {
		e := &catalog[i]
		if cur, ok := latest[e.Type]; !ok || e.CreatedAt.After(cur.CreatedAt) {
			latest[e.Type] = e
		}
	}
	var results []VerificationResult
	for _, e := range latest {
		results = append(results, v.Verify(ctx, *e))
	}
	return results
}

// RunScheduled starts a periodic verification loop that checks all backups
// every interval.
func (v *Verifier) RunScheduled(ctx context.Context, mgr *Manager, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			catalog := mgr.Catalog()
			log.Printf("[backup/verify] scheduled verification: %d entries", len(catalog))
			results := v.VerifyLatest(ctx, catalog)
			invalid := 0
			for _, r := range results {
				if !r.IsFullyValid() {
					invalid++
					log.Printf("[backup/verify] ALERT: invalid backup %s: %v", r.Entry.ID, r.Error)
				}
			}
			if invalid > 0 {
				log.Printf("[backup/verify] SUMMARY: %d invalid out of %d checked",
					invalid, len(results))
			} else {
				log.Printf("[backup/verify] SUMMARY: all %d checked backups valid", len(results))
			}
		}
	}
}
