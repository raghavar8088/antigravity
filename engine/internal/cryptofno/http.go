package cryptofno

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// HTTP surface for the crypto F&O paper desk.
//
// Paper only: this book never reaches a broker. Every mutation is POST so a
// stray GET cannot alter an account, and the balance gate is enforced inside
// ExecuteBasket rather than here — a route that could bypass it would make the
// gate advisory.

// ChainSource supplies live contracts and marks. Implemented over the shared
// Delta option-chain cache, so this desk and the options desks read the SAME
// snapshot and add no upstream requests.
type ChainSource interface {
	// Contracts returns the live chain for an underlying.
	Contracts(underlying string) []ChainRow
	// Spot returns the underlying's current price, 0 when unknown.
	Spot(underlying string) float64
}

// ChainRow is one tradeable contract as the UI needs it.
type ChainRow struct {
	Symbol        string     `json:"symbol"`
	ProductID     int        `json:"productId"`
	Type          OptionType `json:"type"`
	Strike        float64    `json:"strike"`
	Expiry        time.Time  `json:"expiry"`
	MarkPerBTC    float64    `json:"markPerBtc"`
	Bid           float64    `json:"bid"`
	Ask           float64    `json:"ask"`
	IV            float64    `json:"iv"`
	ContractValue float64    `json:"contractValue"`
}

// Service wires the book to HTTP.
type Service struct {
	book    *Book
	chain   ChainSource
	feeRate float64
}

// NewService creates the desk service. feeRate is charged on notional per side.
func NewService(book *Book, chain ChainSource, feeRate float64) *Service {
	if feeRate <= 0 {
		feeRate = 0.0005 // Delta taker, per side
	}
	return &Service{book: book, chain: chain, feeRate: feeRate}
}

func (s *Service) spots() map[string]float64 {
	out := map[string]float64{}
	if s.chain == nil {
		return out
	}
	for _, u := range []string{"BTC", "ETH"} {
		if v := s.chain.Spot(u); v > 0 {
			out[u] = v
		}
	}
	return out
}

// Handler returns the desk's routes.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/crypto-fno/accounts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, s.book.Accounts(s.spots()))
		case http.MethodPost:
			var body struct {
				Name       string  `json:"name"`
				CapitalUSD float64 `json:"capitalUsd"`
			}
			if err := decode(r, &body); err != nil {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
			a, err := s.book.CreateAccount(body.Name, body.CapitalUSD)
			if err != nil {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, a)
		default:
			writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("GET or POST"))
		}
	})

	// Edit name and/or capital. Capital 0 means "leave unchanged", so a rename
	// cannot accidentally re-fund the book.
	mux.HandleFunc("/api/crypto-fno/accounts/edit", s.postOnly(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AccountID  string  `json:"accountId"`
			Name       string  `json:"name"`
			CapitalUSD float64 `json:"capitalUsd"`
		}
		if err := decode(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		a, err := s.book.EditAccount(body.AccountID, body.Name, body.CapitalUSD)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, a)
	}))

	mux.HandleFunc("/api/crypto-fno/accounts/reset", s.postOnly(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AccountID  string  `json:"accountId"`
			CapitalUSD float64 `json:"capitalUsd"`
		}
		if err := decode(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		a, err := s.book.ResetAccount(body.AccountID, body.CapitalUSD)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, a)
	}))

	mux.HandleFunc("/api/crypto-fno/accounts/delete", s.postOnly(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AccountID string `json:"accountId"`
		}
		if err := decode(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if err := s.book.DeleteAccount(body.AccountID); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}))

	// The live chain the basket builder selects from.
	mux.HandleFunc("/api/crypto-fno/chain", func(w http.ResponseWriter, r *http.Request) {
		u := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("underlying")))
		if u == "" {
			u = "BTC"
		}
		if s.chain == nil {
			writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("chain not wired"))
			return
		}
		rows := s.chain.Contracts(u)
		writeJSON(w, http.StatusOK, map[string]any{
			"underlying": u,
			"spot":       s.chain.Spot(u),
			"contracts":  rows,
			"count":      len(rows),
		})
	})

	// Preview runs the SAME margin path as execution, so the ticket cannot
	// disagree with the fill.
	mux.HandleFunc("/api/crypto-fno/preview", s.postOnly(func(w http.ResponseWriter, r *http.Request) {
		req, err := s.decodeBasket(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		spot := s.chain.Spot(req.Underlying)
		margin, view, err := s.book.PreviewBasket(req.AccountID, req.Underlying, req.Legs, spot, s.feeRate)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		// Tell the UI up front whether this will be refused, and by how much,
		// so the button can be disabled with a reason instead of failing later.
		affordable := margin.RequiredUSD <= view.AvailableUSD
		writeJSON(w, http.StatusOK, map[string]any{
			"margin":     margin,
			"account":    view,
			"label":      LabelFor(req.Legs),
			"spot":       spot,
			"affordable": affordable,
			"shortfallUsd": func() float64 {
				if affordable {
					return 0
				}
				return margin.RequiredUSD - view.AvailableUSD
			}(),
		})
	}))

	mux.HandleFunc("/api/crypto-fno/execute", s.postOnly(func(w http.ResponseWriter, r *http.Request) {
		req, err := s.decodeBasket(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		spot := s.chain.Spot(req.Underlying)
		pos, err := s.book.ExecuteBasket(req.AccountID, req.Underlying, req.Legs, spot, s.feeRate)
		if err != nil {
			// A capital rejection is a business outcome the UI must render, not
			// a server fault: 422 rather than 400 or 500.
			var insuff ErrInsufficientCapital
			if asInsufficient(err, &insuff) {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
					"error":        err.Error(),
					"requiredUsd":  insuff.RequiredUSD,
					"availableUsd": insuff.AvailableUSD,
					"shortfallUsd": insuff.Shortfall,
				})
				return
			}
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, pos)
	}))

	mux.HandleFunc("/api/crypto-fno/positions", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.URL.Query().Get("accountId"))
		if id == "" {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("accountId is required"))
			return
		}
		openOnly := r.URL.Query().Get("open") == "1"
		positions := s.book.Positions(id, openOnly)

		spots := s.spots()
		type row struct {
			Position
			UnrealisedUSD float64 `json:"unrealisedUsd"`
		}
		out := make([]row, 0, len(positions))
		for _, p := range positions {
			cp := p
			out = append(out, row{Position: cp, UnrealisedUSD: cp.UnrealisedUSD(spots[cp.Underlying])})
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("/api/crypto-fno/close", s.postOnly(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AccountID  string `json:"accountId"`
			PositionID string `json:"positionId"`
			Reason     string `json:"reason"`
		}
		if err := decode(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		positions := s.book.Positions(body.AccountID, true)
		underlying := "BTC"
		for _, p := range positions {
			if p.ID == body.PositionID {
				underlying = p.Underlying
			}
		}
		reason := body.Reason
		if reason == "" {
			reason = "manual"
		}
		pos, err := s.book.CloseBasket(body.AccountID, body.PositionID, s.chain.Spot(underlying), s.feeRate, reason)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, pos)
	}))

	return mux
}

type basketRequest struct {
	AccountID  string `json:"accountId"`
	Underlying string `json:"underlying"`
	Legs       []Leg  `json:"legs"`
}

// decodeBasket resolves each requested contract against the LIVE chain rather
// than trusting client-supplied prices. A caller could otherwise post a
// favourable premium and book a fill that never existed.
func (s *Service) decodeBasket(r *http.Request) (basketRequest, error) {
	var req basketRequest
	if err := decode(r, &req); err != nil {
		return req, err
	}
	if req.AccountID == "" {
		return req, fmt.Errorf("accountId is required")
	}
	if req.Underlying == "" {
		req.Underlying = "BTC"
	}
	req.Underlying = strings.ToUpper(req.Underlying)
	if len(req.Legs) == 0 {
		return req, fmt.Errorf("basket has no legs")
	}
	if s.chain == nil {
		return req, fmt.Errorf("chain not wired")
	}

	bySymbol := map[string]ChainRow{}
	for _, c := range s.chain.Contracts(req.Underlying) {
		bySymbol[c.Symbol] = c
	}

	for i := range req.Legs {
		l := &req.Legs[i]
		c, ok := bySymbol[l.Symbol]
		if !ok {
			return req, fmt.Errorf("leg %d: %q is not a live contract on %s", i, l.Symbol, req.Underlying)
		}
		// Server-side truth wins for everything price-bearing.
		l.ProductID = c.ProductID
		l.Type = c.Type
		l.Strike = c.Strike
		l.Expiry = c.Expiry
		l.IV = c.IV
		l.ContractValue = c.ContractValue
		// A seller receives the bid and a buyer pays the ask; using the mark for
		// both would credit half the spread that never arrives.
		switch {
		case l.Side == SideSell && c.Bid > 0:
			l.PremiumPerBTC = c.Bid
		case l.Side == SideBuy && c.Ask > 0:
			l.PremiumPerBTC = c.Ask
		default:
			l.PremiumPerBTC = c.MarkPerBTC
		}
		if l.Lots <= 0 {
			return req, fmt.Errorf("leg %d (%s): lots must be positive", i, l.Symbol)
		}
	}
	return req, nil
}

func (s *Service) postOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
			return
		}
		h(w, r)
	}
}

func decode(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}
	return json.NewDecoder(r.Body).Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func asInsufficient(err error, target *ErrInsufficientCapital) bool {
	if e, ok := err.(ErrInsufficientCapital); ok {
		*target = e
		return true
	}
	return false
}
