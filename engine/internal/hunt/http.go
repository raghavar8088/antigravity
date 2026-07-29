package hunt

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// HTTP surface for the strategy hunt.
//
// Read-only. The hunt reports which strategies grew their $1,000 and which
// cleared the pre-registered gate; it never moves capital. Promotion remains an
// authenticated action on the Live Engine, so there is no route from this
// endpoint to real money.

// DeskSource supplies one desk's closed trades. Implemented by adapters over the
// buying, selling and scalp desks.
type DeskSource interface {
	// DeskName is the label shown in the UI ("buying", "selling", "scalp").
	DeskName() string
	// HuntTrades returns every closed paper trade for the desk.
	HuntTrades() []Trade
	// StartingCapital is the per-strategy stake for this desk.
	StartingCapital() float64
}

// Service assembles the hunt view across desks.
type Service struct {
	mu      sync.RWMutex
	sources []DeskSource
	gate    Gate
}

// NewService creates the hunt service with the pre-registered gate.
func NewService(gate Gate, sources ...DeskSource) *Service {
	if gate.MinTrades == 0 && gate.MinDays == 0 {
		gate = DefaultGate
	}
	return &Service{sources: sources, gate: gate}
}

// AddSource registers another desk.
func (s *Service) AddSource(src DeskSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources = append(s.sources, src)
}

// Accounts derives every account across every desk.
func (s *Service) Accounts() []Account {
	s.mu.RLock()
	srcs := append([]DeskSource(nil), s.sources...)
	s.mu.RUnlock()

	var all []Account
	for _, src := range srcs {
		all = append(all, BuildAccounts(src.DeskName(), src.HuntTrades(), src.StartingCapital())...)
	}
	return all
}

type leaderboardResponse struct {
	Summary  HuntSummary `json:"summary"`
	Gate     Gate        `json:"gate"`
	GateDesc string      `json:"gateDescription"`
	Rows     []row       `json:"rows"`
}

type row struct {
	Account
	Verdict Verdict `json:"verdict"`
}

// GateDescription is shown next to the leaderboard so the distinction between
// ranking and authorisation is visible in the product, not just the code.
const GateDescription = "Growth ranks this table; it does not authorise capital. " +
	"With ~900 concurrent accounts roughly half end profitable by chance and the best few look excellent, " +
	"so promotion requires the PRE-REGISTERED gate: >=200 trades, >=30 days, PF>=1.2 net of fees, " +
	"maxDD<=25%, positive expectancy, fee drag <=30% of gross, and net-positive in BOTH halves of its window. " +
	"Passer counts are reported against how many chance alone would produce."

// Handler serves the hunt endpoints.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/hunt/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		accounts := s.Accounts()
		lb := Leaderboard(accounts)

		// Optional filters, so the UI can ask for gate survivors only.
		if r.URL.Query().Get("gate") == "pass" {
			lb = Candidates(accounts, s.gate)
		}
		if desk := strings.TrimSpace(r.URL.Query().Get("desk")); desk != "" {
			filtered := lb[:0:0]
			for _, a := range lb {
				if strings.EqualFold(a.Desk, desk) {
					filtered = append(filtered, a)
				}
			}
			lb = filtered
		}
		limit := 500
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 5000 {
			limit = n
		}
		if len(lb) > limit {
			lb = lb[:limit]
		}

		rows := make([]row, 0, len(lb))
		for _, a := range lb {
			rows = append(rows, row{Account: a, Verdict: s.gate.Evaluate(a)})
		}

		writeJSON(w, leaderboardResponse{
			Summary:  Summarise(accounts, s.gate),
			Gate:     s.gate,
			GateDesc: GateDescription,
			Rows:     rows,
		})
	})

	mux.HandleFunc("/api/hunt/summary", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, Summarise(s.Accounts(), s.gate))
	})

	// Candidates are gate survivors re-validated at LIVE size. A strategy proven
	// on $1,000 can be untradeable at the $100 ceiling, so this endpoint answers
	// "can it actually trade live?" rather than "did it do well?".
	mux.HandleFunc("/api/hunt/candidates", func(w http.ResponseWriter, r *http.Request) {
		accounts := s.Accounts()
		cands := Candidates(accounts, s.gate)

		contractCost := TypicalContractCostUSD
		if v, err := strconv.ParseFloat(r.URL.Query().Get("contractCostUsd"), 64); err == nil && v > 0 {
			contractCost = v
		}

		type candidate struct {
			Account
			Promotion PromotionCheck `json:"promotion"`
		}
		out := make([]candidate, 0, len(cands))
		for _, a := range cands {
			c := candidate{Account: a}
			if strings.EqualFold(a.Desk, "selling") {
				// Payoff shape decides this, not the record: a naked short has
				// unbounded loss against a $100 account.
				c.Promotion = SellingPromotionPolicy{AllowDefinedRiskSpreads: false}.
					CheckSellingPromotable(a, false)
			} else {
				c.Promotion = CheckPromotable(a, s.gate, contractCost)
			}
			out = append(out, c)
		}
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Promotion.Ready != out[j].Promotion.Ready {
				return out[i].Promotion.Ready
			}
			return out[i].GrowthPct > out[j].GrowthPct
		})

		writeJSON(w, map[string]any{
			"note": "Gate survivors re-validated at live size. Promotion remains a human action " +
				"on the Live Engine; nothing here moves capital.",
			"contractCostUsd": contractCost,
			"liveCeilingUsd":  LiveCeilingUSD,
			"candidates":      out,
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
