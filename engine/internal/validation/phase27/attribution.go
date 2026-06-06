package phase27

import "antigravity-engine/internal/validation/phase25"

// BuildAttributionRecords maps each LiveTrade directly to an AlphaAttributionRecord.
// The AlphaFamily is taken verbatim from the trade — no inference.
func BuildAttributionRecords(trades []phase25.LiveTrade) []AlphaAttributionRecord {
	records := make([]AlphaAttributionRecord, 0, len(trades))
	for _, t := range trades {
		records = append(records, AlphaAttributionRecord{
			TradeID:      t.TradeID,
			StrategyName: t.StrategyName,
			AlphaFamily:  t.AlphaFamily,
			NetPnLUSD:    t.NetPnLUSD,
			GrossPnLUSD:  t.GrossPnLUSD,
			FeeUSD:       t.FeeUSD,
			SlippageUSD:  t.SlippageUSD,
			FundingUSD:   t.FundingUSD,
			IsWin:        t.IsWin,
			EntryTime:    t.EntryTime,
		})
	}
	return records
}
