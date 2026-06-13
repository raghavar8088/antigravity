package calibration

import "testing"

func TestApply_NilResult(t *testing.T) {
	if got := Apply(75.0, nil); got != 75.0 {
		t.Errorf("Apply with nil should return unchanged, got %v", got)
	}
}

func TestApply_NotOverconfident(t *testing.T) {
	result := &CalibrationResult{IsOverconfident: false, CalibrationFactor: 0.85}
	if got := Apply(80.0, result); got != 80.0 {
		t.Errorf("Apply when not overconfident should return unchanged, got %v", got)
	}
}

func TestApply_Overconfident(t *testing.T) {
	result := &CalibrationResult{IsOverconfident: true, CalibrationFactor: 0.85}
	got := Apply(80.0, result)
	expected := 80.0 * 0.85
	if got != expected {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestApply_ClampsToZero(t *testing.T) {
	result := &CalibrationResult{IsOverconfident: true, CalibrationFactor: -1.0}
	if got := Apply(50.0, result); got != 0 {
		t.Errorf("expected 0, got %v", got)
	}
}

func TestApply_ClampsToHundred(t *testing.T) {
	result := &CalibrationResult{IsOverconfident: true, CalibrationFactor: 2.0}
	if got := Apply(80.0, result); got != 100 {
		t.Errorf("expected 100, got %v", got)
	}
}
