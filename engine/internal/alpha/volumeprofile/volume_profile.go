package volumeprofile

import (
	"sort"
	"time"

	"antigravity-engine/internal/alpha"
)

type Node struct {
	Price  float64
	Volume float64
}

type Snapshot struct {
	Symbol    string
	POC       float64
	HVN       []float64
	LVN       []float64
	VAH       float64
	VAL       float64
	StartedAt time.Time
	EndedAt   time.Time
}

func Build(candles []alpha.Candle, bins int) Snapshot {
	if len(candles) == 0 {
		return Snapshot{}
	}
	if bins < 10 {
		bins = 48
	}
	low, high := candles[0].Low, candles[0].High
	totalVol := 0.0
	for _, c := range candles {
		if c.Low < low {
			low = c.Low
		}
		if c.High > high {
			high = c.High
		}
		totalVol += c.Volume
	}
	if high <= low {
		return Snapshot{Symbol: candles[len(candles)-1].Symbol, POC: candles[len(candles)-1].Close}
	}
	width := (high - low) / float64(bins)
	vols := make([]Node, bins)
	for i := range vols {
		vols[i].Price = low + (float64(i)+0.5)*width
	}
	for _, c := range candles {
		idx := int((c.Close - low) / width)
		if idx < 0 {
			idx = 0
		}
		if idx >= bins {
			idx = bins - 1
		}
		vols[idx].Volume += c.Volume
	}
	poc := vols[0]
	for _, n := range vols {
		if n.Volume > poc.Volume {
			poc = n
		}
	}
	sorted := append([]Node(nil), vols...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Volume > sorted[j].Volume })
	valueVol, vah, val := 0.0, poc.Price, poc.Price
	for _, n := range sorted {
		valueVol += n.Volume
		if n.Price > vah {
			vah = n.Price
		}
		if n.Price < val {
			val = n.Price
		}
		if totalVol > 0 && valueVol/totalVol >= 0.70 {
			break
		}
	}
	hvn, lvn := []float64{}, []float64{}
	for i, n := range sorted {
		if i < 3 {
			hvn = append(hvn, n.Price)
		}
		if i >= len(sorted)-3 {
			lvn = append(lvn, n.Price)
		}
	}
	return Snapshot{Symbol: candles[len(candles)-1].Symbol, POC: poc.Price, HVN: hvn, LVN: lvn, VAH: vah, VAL: val, StartedAt: candles[0].Timestamp, EndedAt: candles[len(candles)-1].Timestamp}
}
