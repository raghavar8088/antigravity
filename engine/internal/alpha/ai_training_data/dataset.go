package ai_training_data

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"antigravity-engine/internal/alpha"
)

type Row struct {
	Timestamp     time.Time
	Symbol        string
	RSI           float64
	ADX           float64
	Funding       float64
	CVD           float64
	Delta         float64
	MSS           float64
	VolumeProfile float64
	Session       float64
	Volatility    float64
	MarketRegime  string
	Output        alpha.Action
}

type Writer struct {
	path string
}

func NewWriter(path string) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		w := csv.NewWriter(f)
		_ = w.Write([]string{"timestamp", "symbol", "rsi", "adx", "funding", "cvd", "delta", "mss", "volume_profile", "session", "volatility", "market_regime", "output"})
		w.Flush()
		_ = f.Close()
	}
	return &Writer{path: path}, nil
}

func (w *Writer) Append(row Row) error {
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	cw := csv.NewWriter(f)
	err = cw.Write([]string{
		row.Timestamp.UTC().Format(time.RFC3339Nano), row.Symbol,
		ff(row.RSI), ff(row.ADX), ff(row.Funding), ff(row.CVD), ff(row.Delta),
		ff(row.MSS), ff(row.VolumeProfile), ff(row.Session), ff(row.Volatility),
		row.MarketRegime, string(row.Output),
	})
	cw.Flush()
	if err != nil {
		return err
	}
	return cw.Error()
}

func ff(v float64) string {
	return strconv.FormatFloat(v, 'f', 8, 64)
}

func LabelFromForwardReturn(ret float64) alpha.Action {
	switch {
	case ret > 0.0015:
		return alpha.ActionBuy
	case ret < -0.0015:
		return alpha.ActionSell
	default:
		return alpha.ActionHold
	}
}

func DatasetPath(baseDir, symbol string) string {
	return filepath.Join(baseDir, fmt.Sprintf("%s_alpha_training.csv", symbol))
}
