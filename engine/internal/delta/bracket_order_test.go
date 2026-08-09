package delta

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every bracket leg must carry a LIMIT PRICE.
//
// Its absence is what Delta rejected on every attempt for three hours:
//
//	"Limit price required for limit orders"          param: limit_price
//	"invalid value"    param: bracket_take_profit_limit_price
//
// The failure logged a warning and fell back to the 15-second monitor, so the
// desk kept trading and looked like it was working. Measured cost: stop-outs
// exiting at 0.830% against a 0.580% stop — a 1.43x overshoot on every loss.
//
// This is the assertion that would have caught it before a single trade.
func TestBracketRequest_EveryLegNeedsAStopAndALimit(t *testing.T) {
	good := BracketRequest{
		ProductID:  27,
		StopLoss:   &BracketLeg{OrderType: TypeLimit, StopPrice: "0.19", LimitPrice: "0.19"},
		TakeProfit: &BracketLeg{OrderType: TypeLimit, StopPrice: "0.21", LimitPrice: "0.21"},
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("a well-formed bracket was refused: %v", err)
	}

	// The exact shape that was being sent.
	missingLimit := good
	missingLimit.TakeProfit = &BracketLeg{OrderType: TypeLimit, StopPrice: "0.21"}
	err := missingLimit.Validate()
	if err == nil {
		t.Fatal("a leg with no limit_price was accepted; this is the payload Delta rejected 12 times")
	}
	if !strings.Contains(err.Error(), "limit_price") {
		t.Errorf("error %q does not name the missing field", err)
	}

	// And the stop leg.
	missingStopLimit := good
	missingStopLimit.StopLoss = &BracketLeg{OrderType: TypeLimit, StopPrice: "0.19"}
	if missingStopLimit.Validate() == nil {
		t.Error("a stop leg with no limit_price was accepted")
	}
}

// A bracket with neither leg protects nothing. Refusing beats sending a request
// that succeeds and leaves the position naked.
func TestBracketRequest_RefusesAnEmptyBracket(t *testing.T) {
	if (BracketRequest{ProductID: 27}).Validate() == nil {
		t.Error("a bracket with no legs was accepted; it would protect nothing")
	}
	if (BracketRequest{StopLoss: &BracketLeg{OrderType: TypeLimit, StopPrice: "1", LimitPrice: "1"}}).Validate() == nil {
		t.Error("a bracket with no product id was accepted")
	}
	// Missing order_type is the third field Delta validates.
	noType := BracketRequest{ProductID: 27, StopLoss: &BracketLeg{StopPrice: "1", LimitPrice: "1"}}
	if noType.Validate() == nil {
		t.Error("a leg with no order_type was accepted")
	}
}

// The JSON must use the field names Delta's schema errors named, or the request
// is rejected for a different reason and the fix is illusory.
func TestBracketRequest_SerialisesToDeltasSchema(t *testing.T) {
	blob, err := json.Marshal(BracketRequest{
		ProductID:     27,
		ProductSymbol: "ADAUSD",
		StopLoss:      &BracketLeg{OrderType: TypeLimit, StopPrice: "0.19", LimitPrice: "0.19"},
		TakeProfit:    &BracketLeg{OrderType: TypeLimit, StopPrice: "0.21", LimitPrice: "0.21"},
		TriggerMethod: "last_traded_price",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := string(blob)
	for _, want := range []string{
		`"product_id":27`,
		`"stop_loss_order"`,
		`"take_profit_order"`,
		`"stop_price":"0.19"`,
		`"limit_price":"0.19"`,
		`"bracket_stop_trigger_method":"last_traded_price"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("payload missing %s\ngot: %s", want, body)
		}
	}
	// The parameters must NOT be on the entry order any more — that is where
	// they were, and where Delta refused them.
	if strings.Contains(body, "bracket_take_profit_limit_price") {
		t.Error("the old flat bracket_* parameters are back on the request")
	}
}
