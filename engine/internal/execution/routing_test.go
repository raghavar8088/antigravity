package execution

import "testing"

func TestRouteModeForCategory(t *testing.T) {
	if got := RouteModeForCategory("Mean Rev Elite", "RANGE"); got != OrderModePostOnly {
		t.Fatalf("expected mean reversion to use post-only, got %s", got)
	}
	if got := RouteModeForCategory("Breakout Elite", "TREND"); got != OrderModeIOC {
		t.Fatalf("expected breakout to use IOC, got %s", got)
	}
	if got := RouteModeForCategory("Momentum Elite", "TREND"); got != OrderModeMarket {
		t.Fatalf("expected momentum to use market routing, got %s", got)
	}
	if got := RouteModeForCategory("Trend", "TREND"); got != OrderModePostOnly {
		t.Fatalf("expected trend pullback routing to use post-only, got %s", got)
	}
}
