package execution

import "antigravity-engine/internal/strategy"

// Engine represents standard REST calls mapped against ANY exchange (Paper or Live).
type Engine interface {
	PlaceMarketOrder(sig strategy.Signal) error
	GetPosition(symbol string) float64
	GetBalanceUSD() float64
	ResetAccount() error
}
