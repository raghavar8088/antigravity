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
}
