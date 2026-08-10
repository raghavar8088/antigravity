package delta

// PerpStreamView is one roster entry as the UI sees it.
//
// Enabled is per STREAM, not per strategy. Switching
// ANTI_Recurrence_Quantification_Signal off on COOKIEUSD must not also switch
// it off on MUBARAKUSD: they are separate positions with separate records, and
// the reason to stop one is usually the instrument rather than the logic.
type PerpStreamView struct {
	Strategy string `json:"strategy"`
	Symbol   string `json:"symbol"`
	Enabled  bool   `json:"enabled"`

	// GridRefusals counts signals the pre-trade grid gate turned away, and
	// LastStopTicks is how wide the stop was the last time it did.
	//
	// Reported because a stream can be switched ON and still be structurally
	// unable to trade: its symbol's tick grid is too coarse for the stop the
	// strategy wants. Without this it shows as "on" with no fills, which reads
	// as a strategy that is not signalling rather than one being refused every
	// time it does.
	GridRefusals  int     `json:"gridRefusals,omitempty"`
	LastStopTicks float64 `json:"lastStopTicks,omitempty"`
}

// gridBlock is the per-stream refusal record behind those fields.
type gridBlock struct {
	Refusals      int
	LastStopTicks float64
}
