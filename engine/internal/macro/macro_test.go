package macro

import "testing"

func TestComputeMacroScore_CoupledSPYUp(t *testing.T) {
	data := MacroData{MacroCoupled: true, SPY_Dir_1h: "UP", DXY_Trend: "FLAT", VIX: 18}
	s := ComputeMacroScore(data)
	if s != 2.0 {
		t.Errorf("expected 2.0, got %v", s)
	}
}

func TestComputeMacroScore_CoupledSPYDown_HighVIX(t *testing.T) {
	data := MacroData{MacroCoupled: true, SPY_Dir_1h: "DOWN", DXY_Trend: "RISING", VIX: 40}
	s := ComputeMacroScore(data)
	// -2 (spy down) - 1 (dxy rising) - 1.5 (vix>35) = -4.5 → clamped -3
	if s != -3.0 {
		t.Errorf("expected -3.0, got %v", s)
	}
}

func TestComputeMacroScore_Uncoupled(t *testing.T) {
	data := MacroData{MacroCoupled: false, SPY_Dir_1h: "UP", DXY_Trend: "FALLING", VIX: 12}
	s := ComputeMacroScore(data)
	// 0 (not coupled) + 0.5 (dxy falling) + 0.5 (vix<15) = 1.0
	if s != 1.0 {
		t.Errorf("expected 1.0, got %v", s)
	}
}

func TestGetLatest_NilBeforeFetch(t *testing.T) {
	f := NewMacroFetcher("python3", "macro_fetcher.py")
	if f.GetLatest() != nil {
		t.Error("expected nil before first fetch")
	}
}
