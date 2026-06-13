package learning

import "testing"

func TestTradeOutcomeConstants(t *testing.T) {
	if OutcomeWin != "WIN" {
		t.Errorf("expected WIN, got %s", OutcomeWin)
	}
	if OutcomeLoss != "LOSS" {
		t.Errorf("expected LOSS, got %s", OutcomeLoss)
	}
	if OutcomeBreakeven != "BREAKEVEN" {
		t.Errorf("expected BREAKEVEN, got %s", OutcomeBreakeven)
	}
}

func TestGenerateLessonText_Win(t *testing.T) {
	worked, failed := generateLessonText("EMA_CROSS", OutcomeWin, "TRENDING_BULL", "TP2")
	if worked == "" {
		t.Error("win lesson should have worked text")
	}
	if failed != "" {
		t.Error("win lesson should have empty failed text")
	}
}

func TestGenerateLessonText_Loss(t *testing.T) {
	worked, failed := generateLessonText("RSI_THRESHOLD", OutcomeLoss, "RANGING", "SL")
	if worked != "" {
		t.Error("loss lesson should have empty worked text")
	}
	if failed == "" {
		t.Error("loss lesson should have failed text")
	}
}

func TestGetLatestSummary_NilBeforeGenerate(t *testing.T) {
	g := &LessonGenerator{}
	if g.GetLatestSummary() != nil {
		t.Error("expected nil before first generation")
	}
}
