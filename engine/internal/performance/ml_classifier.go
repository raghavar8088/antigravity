package performance

type MLAction string

const (
	MLBuy  MLAction = "BUY"
	MLSell MLAction = "SELL"
	MLHold MLAction = "HOLD"
)

type MLFeatures struct {
	EMAFast      float64
	EMASlow      float64
	VWAPDistance float64
	RSI          float64
	ADX          float64
	VolumeZ      float64
	ATRPct       float64
	FundingRate  float64
	RegimeScore  float64
}

type MLDecision struct {
	Action          MLAction
	Confidence      float64
	Model           string
	LatencyTargetMs float64
}

type LinearClassifier struct {
	Weights [9]float64
	Bias    float64
}

func NewCouncilReplacementClassifier() LinearClassifier {
	return LinearClassifier{Weights: [9]float64{0.18, -0.18, -0.12, -0.05, 0.16, 0.08, -0.10, -0.07, 0.12}, Bias: 0}
}

func (c LinearClassifier) Infer(f MLFeatures) MLDecision {
	x := [9]float64{f.EMAFast, f.EMASlow, f.VWAPDistance, f.RSI / 100, f.ADX / 50, f.VolumeZ, f.ATRPct, f.FundingRate * 1000, f.RegimeScore}
	score := c.Bias
	for i, v := range x {
		score += c.Weights[i] * v
	}
	conf := clamp01(absFloat(score))
	action := MLHold
	if conf >= 0.55 {
		if score > 0 {
			action = MLBuy
		} else {
			action = MLSell
		}
	}
	return MLDecision{Action: action, Confidence: conf, Model: "linear-baseline-for-xgboost-shadow-validation", LatencyTargetMs: 5}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
