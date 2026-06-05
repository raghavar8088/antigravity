package phase22f

import (
	"fmt"
	"sort"
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

// AuditDataIntegrity performs Phase 1 data integrity certification.
// It audits the provided trade records and returns a full certification report.
// Only records that survive deduplication and sanity checks count as certified.
func AuditDataIntegrity(trades []phase22e.TradeRecord) DataIntegrityCertification {
	cert := DataIntegrityCertification{
		GeneratedAt: time.Now().UTC(),
	}

	if len(trades) == 0 {
		cert.Issues = append(cert.Issues, "CRITICAL: zero trade records supplied — no data to certify")
		return cert
	}

	// ── Paper trade audit ────────────────────────────────────────────────────
	paperAudit := auditSource(DataSourcePaperTrades, trades, false)
	cert.Sources = append(cert.Sources, paperAudit)

	// ── Backtest V2 / V3 (subset of non-live) ───────────────────────────────
	btV2 := auditSource(DataSourceBacktestV2, filterNonLive(trades), false)
	btV3 := auditSource(DataSourceBacktestV3, filterNonLive(trades), false)
	cert.Sources = append(cert.Sources, btV2, btV3)

	// ── OMS audit ────────────────────────────────────────────────────────────
	omsAudit := DataSourceAudit{
		Source:       DataSourceOMS,
		Available:    true,
		TotalRecords: len(trades),
		ValidRecords: countValidOMS(trades),
	}
	cert.Sources = append(cert.Sources, omsAudit)

	// ── Cross-source certification ────────────────────────────────────────────
	certified, issues := certifyTrades(trades)
	cert.Issues = append(cert.Issues, issues...)

	cert.CertifiedTrades = len(certified)
	cert.CertifiedFills = countFills(certified)
	cert.CertifiedStrategies = countUniqueStrategies(certified)

	// check survivorship bias: if only strategies with net positive PnL present
	if allPositivePnL(certified) && len(certified) > 20 {
		cert.Issues = append(cert.Issues, "WARNING: survivorship bias detected — all strategies show positive total PnL")
		for i := range cert.Sources {
			cert.Sources[i].SurvivshipBias = true
		}
	}

	// check look-ahead bias
	if hasLookAheadBias(certified) {
		cert.Issues = append(cert.Issues, "WARNING: potential look-ahead bias — some exit prices precede entry times")
		for i := range cert.Sources {
			cert.Sources[i].LookAheadBias = true
		}
	}

	cert.Passed = len(cert.Issues) == 0
	if !cert.Passed {
		// non-fatal issues are warnings; fatal ones start with CRITICAL
		fatal := 0
		for _, iss := range cert.Issues {
			if len(iss) >= 8 && iss[:8] == "CRITICAL" {
				fatal++
			}
		}
		cert.Passed = fatal == 0
	}
	return cert
}

// ── helpers ──────────────────────────────────────────────────────────────────

func auditSource(src DataSourceType, trades []phase22e.TradeRecord, liveOnly bool) DataSourceAudit {
	a := DataSourceAudit{
		Source:       src,
		Available:    len(trades) > 0,
		TotalRecords: len(trades),
	}
	seen := make(map[string]bool, len(trades))
	valid := 0
	dups := 0
	corrupt := 0
	for _, t := range trades {
		if liveOnly && !t.IsLive {
			continue
		}
		if t.TradeID == "" || t.StrategyID == "" {
			corrupt++
			continue
		}
		if seen[t.TradeID] {
			dups++
			continue
		}
		seen[t.TradeID] = true
		if t.ExitTime.Before(t.EntryTime) {
			corrupt++
			continue
		}
		valid++
	}
	a.ValidRecords = valid
	a.Duplicates = dups
	a.Corrupted = corrupt
	a.Missing = a.TotalRecords - valid - dups - corrupt
	return a
}

func filterNonLive(trades []phase22e.TradeRecord) []phase22e.TradeRecord {
	out := make([]phase22e.TradeRecord, 0, len(trades))
	for _, t := range trades {
		if !t.IsLive {
			out = append(out, t)
		}
	}
	return out
}

func countValidOMS(trades []phase22e.TradeRecord) int {
	n := 0
	for _, t := range trades {
		if t.TradeID != "" && t.Quantity > 0 && t.EntryPrice > 0 {
			n++
		}
	}
	return n
}

// certifyTrades deduplicates and sanity-checks the full trade list.
// Returns (certified records, issues).
func certifyTrades(trades []phase22e.TradeRecord) ([]phase22e.TradeRecord, []string) {
	var issues []string
	seen := make(map[string]bool, len(trades))
	certified := make([]phase22e.TradeRecord, 0, len(trades))

	for _, t := range trades {
		if t.TradeID == "" {
			issues = append(issues, fmt.Sprintf("WARN: trade with no ID (strategy=%s) skipped", t.StrategyID))
			continue
		}
		if seen[t.TradeID] {
			continue // silently dedup
		}
		seen[t.TradeID] = true
		if t.ExitTime.Before(t.EntryTime) {
			issues = append(issues, fmt.Sprintf("WARN: trade %s has exit before entry — skipped", t.TradeID))
			continue
		}
		if t.EntryPrice <= 0 || t.Quantity <= 0 {
			issues = append(issues, fmt.Sprintf("WARN: trade %s has invalid entry price or quantity — skipped", t.TradeID))
			continue
		}
		certified = append(certified, t)
	}

	// sort by exit time for consistent downstream processing
	sort.Slice(certified, func(i, j int) bool {
		return certified[i].ExitTime.Before(certified[j].ExitTime)
	})
	return certified, issues
}

func countFills(trades []phase22e.TradeRecord) int {
	// Each closed trade represents at least 2 fills (entry + exit).
	return len(trades) * 2
}

func countUniqueStrategies(trades []phase22e.TradeRecord) int {
	m := make(map[string]bool)
	for _, t := range trades {
		m[t.StrategyID] = true
	}
	return len(m)
}

func allPositivePnL(trades []phase22e.TradeRecord) bool {
	byStrat := make(map[string]float64)
	for _, t := range trades {
		byStrat[t.StrategyID] += t.NetPnLUSD
	}
	for _, pnl := range byStrat {
		if pnl <= 0 {
			return false
		}
	}
	return true
}

func hasLookAheadBias(trades []phase22e.TradeRecord) bool {
	for _, t := range trades {
		if t.ExitTime.Before(t.EntryTime) || t.ExitTime.Equal(t.EntryTime) {
			if t.NetPnLUSD != 0 {
				return true
			}
		}
	}
	return false
}
