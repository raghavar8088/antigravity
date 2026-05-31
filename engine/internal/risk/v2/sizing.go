package v2

type SizingScale string

const (
	Size100     SizingScale = "100%"
	Size75      SizingScale = "75%"
	Size50      SizingScale = "50%"
	Size25      SizingScale = "25%"
	Size10      SizingScale = "10%"
	SizeDisable SizingScale = "DISABLE"
)

type SizingDecisionLog struct {
	Layer      string
	BeforeBTC  float64
	AfterBTC   float64
	Multiplier float64
	Reason     string
}

func scaleFromMultiplier(mult float64) SizingScale {
	switch {
	case mult <= 0:
		return SizeDisable
	case mult <= 0.10:
		return Size10
	case mult <= 0.25:
		return Size25
	case mult <= 0.50:
		return Size50
	case mult <= 0.75:
		return Size75
	default:
		return Size100
	}
}
