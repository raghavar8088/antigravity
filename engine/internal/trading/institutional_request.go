package trading

import (
	"context"
	"fmt"
	"strings"

	"antigravity-engine/internal/delta"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/executiongateway"
	"antigravity-engine/internal/strategy"
)

// ProcessExecutionRequest is the single orchestrator entry for external execution intents.
func (o *Orchestrator) ProcessExecutionRequest(ctx context.Context, req executiongateway.Request) (executiongateway.Response, error) {
	if o.killSvc != nil && o.killSvc.IsActive() {
		return executiongateway.Response{
			OK: false, Status: "BLOCKED", RequestID: req.RequestID,
			Message: "kill switch active: " + o.killSvc.Reason(),
		}, nil
	}

	switch req.Venue {
	case "paper", "":
		return o.processPaperExecutionRequest(ctx, req)
	case "delta":
		return o.processDeltaExecutionRequest(ctx, req)
	default:
		return executiongateway.Response{
			OK: false, Status: "REJECTED", RequestID: req.RequestID,
			Message: fmt.Sprintf("unsupported venue: %s", req.Venue),
		}, nil
	}
}

func (o *Orchestrator) processPaperExecutionRequest(ctx context.Context, req executiongateway.Request) (executiongateway.Response, error) {
	o.mu.RLock()
	price := o.lastPrice
	o.mu.RUnlock()
	if price <= 0 {
		return executiongateway.Response{OK: false, Status: "REJECTED", RequestID: req.RequestID, Message: "no market price"}, nil
	}
	if req.Size <= 0 {
		return executiongateway.Response{OK: false, Status: "REJECTED", RequestID: req.RequestID, Message: "size must be > 0"}, nil
	}
	symbol := req.Symbol
	if symbol == "" {
		symbol = "BTC-USD"
	}
	action := strategy.ActionBuy
	if req.Side == "SELL" || req.Side == "SHORT" {
		action = strategy.ActionSell
	}
	conf := req.Confidence
	if conf <= 0 {
		conf = 0.75
	}
	sig := strategy.Signal{
		Symbol:        symbol,
		Action:        action,
		TargetSize:    req.Size,
		Confidence:    conf,
		StopLossPct:   defaultSignalStopLossPct,
		TakeProfitPct: minSignalTakeProfitPct,
	}
	fill, err := o.executeThroughInstitutionalPath(ctx, sig, req.StrategyName, price, execution.OrderModeIOC)
	if err != nil {
		return executiongateway.Response{OK: false, Status: "BLOCKED", RequestID: req.RequestID, Message: err.Error()}, nil
	}
	return executiongateway.Response{
		OK: true, Status: "ACCEPTED", RequestID: req.RequestID,
		ClientOrderID: fill.ClientOrderID, Message: "paper fill via institutional path",
	}, nil
}

func (o *Orchestrator) processDeltaExecutionRequest(ctx context.Context, req executiongateway.Request) (executiongateway.Response, error) {
	if o.deltaBroker == nil {
		return executiongateway.Response{OK: false, Status: "REJECTED", RequestID: req.RequestID, Message: "delta broker not configured"}, nil
	}
	contracts := req.Contracts
	if contracts < 1 {
		contracts = int(req.Size)
	}
	if contracts < 1 {
		contracts = 1
	}
	symbol := strings.TrimSpace(req.Symbol)
	if symbol == "" {
		return executiongateway.Response{OK: false, Status: "REJECTED", RequestID: req.RequestID, Message: "symbol required"}, nil
	}
	side := delta.SideSell
	action := strategy.ActionSell
	if req.Side == "BUY" || req.Side == "LONG" {
		side = delta.SideBuy
		action = strategy.ActionBuy
	}
	o.mu.RLock()
	price := o.lastPrice
	if price <= 0 {
		price = 90000
	}
	o.mu.RUnlock()

	sig := strategy.Signal{
		Symbol:        "DELTA:" + symbol,
		Action:        action,
		TargetSize:    float64(contracts) * 0.001,
		Confidence:    0.75,
		StopLossPct:   defaultSignalStopLossPct,
		TakeProfitPct: minSignalTakeProfitPct,
	}
	fillFn := func(c context.Context, _ strategy.Signal, clientOrderID string) (execution.FillResult, error) {
		if o.deltaBroker == nil || o.deltaBroker.Client() == nil {
			return execution.FillResult{}, fmt.Errorf("delta broker not configured")
		}
		info, err := o.deltaBroker.Client().FindProductBySymbol(c, symbol)
		if err != nil {
			return execution.FillResult{}, err
		}
		result, err := o.deltaBroker.SubmitOrder(c, info.ProductID, side, contracts)
		if err != nil {
			return execution.FillResult{}, err
		}
		return execution.FillResult{
			ExecPrice: result.Price, OrderMode: execution.OrderModeMarket, ClientOrderID: clientOrderID,
		}, nil
	}
	fill, err := o.executeThroughInstitutionalPathWithFill(ctx, sig, req.StrategyName, price, execution.OrderModeMarket, fillFn)
	if err != nil {
		return executiongateway.Response{OK: false, Status: "BLOCKED", RequestID: req.RequestID, Message: err.Error()}, nil
	}
	return executiongateway.Response{
		OK: true, Status: "ACCEPTED", RequestID: req.RequestID,
		ClientOrderID: fill.ClientOrderID, Message: "delta via institutional path",
	}, nil
}

// WireDeltaBridge connects the delta live bridge to institutional execution gates.
func (o *Orchestrator) WireDeltaBridge(bridge *delta.Bridge) {
	if bridge == nil {
		return
	}
	o.SetDeltaBroker(bridge)
	bridge.SetKillCheck(func(ctx context.Context) error {
		if o.killSvc != nil && o.killSvc.IsActive() {
			return fmt.Errorf("kill switch active: %s", o.killSvc.Reason())
		}
		return nil
	})
	bridge.SetInstitutionalOpenHandler(func(ctx context.Context, open delta.OpenSignal, tradeID string) error {
		o.mu.RLock()
		price := open.BTCPrice
		if price <= 0 {
			price = o.lastPrice
		}
		o.mu.RUnlock()
		if price <= 0 {
			return fmt.Errorf("no reference price for institutional delta open")
		}
		action := strategy.ActionSell
		side := delta.SideSell
		if bridge.IsBuyingMode() {
			action = strategy.ActionBuy
			side = delta.SideBuy
		}
		sig := strategy.Signal{
			Symbol:        fmt.Sprintf("DELTA-OPT:%s:%.0f", open.OptionType, open.Strike),
			Action:        action,
			TargetSize:    open.PremiumUSD / 100000,
			Confidence:    0.72,
			StopLossPct:   defaultSignalStopLossPct,
			TakeProfitPct: minSignalTakeProfitPct,
		}
		if sig.TargetSize < 0.01 {
			sig.TargetSize = 0.01
		}
		var captured delta.PlaceOrderResult
		var productID int
		var contracts int
		var premiumPerContractUSD float64

		// Resolve the option product and its real per-contract premium up front so
		// the post-gate budget assertion can price the order. The premium unit is
		// confirmed from Delta's live spec: USD/contract = mark(USD/BTC) × 0.001.
		if info, rerr := bridge.Client().FindOptionProduct(ctx, open.Strike, open.ExpiryTime, open.OptionType); rerr == nil {
			productID = info.ProductID
			if p, perr := bridge.Client().OptionPremiumPerContractUSD(ctx, info.Symbol); perr == nil {
				premiumPerContractUSD = p
			}
		}

		fillFn := func(c context.Context, _ strategy.Signal, _ string) (execution.FillResult, error) {
			if productID == 0 {
				info, err := bridge.Client().FindOptionProduct(c, open.Strike, open.ExpiryTime, open.OptionType)
				if err != nil {
					return execution.FillResult{}, err
				}
				productID = info.ProductID
			}
			contracts = int(open.PremiumUSD / 100)
			if contracts < 1 {
				contracts = 1
			}
			if bridge.IsBuyingMode() {
				bal, err2 := bridge.Client().GetWallet(c)
				if err2 != nil {
					return execution.FillResult{}, err2
				}
				if bal < 1 {
					return execution.FillResult{}, fmt.Errorf("wallet below minimum for delta buy")
				}
				contracts = 1
			}
			result, err := bridge.SubmitOrder(c, productID, side, contracts)
			if err != nil {
				return execution.FillResult{}, err
			}
			captured = result
			return execution.FillResult{ExecPrice: result.Price, OrderMode: execution.OrderModeMarket}, nil
		}
		var pathOpts []InstitutionalPathOpts
		if bridge.IsBuyingMode() {
			// Live-money buys carry the budget backstop at the post-gate choke
			// point, priced with the real per-contract premium. If the quote was
			// unavailable (premiumPerContractUSD == 0), AssertBuyWithinBudget fails
			// closed — the buy is rejected rather than sized blind.
			premium := premiumPerContractUSD
			pathOpts = append(pathOpts, InstitutionalPathOpts{
				PreSubmitAssert: func() error {
					bal, werr := bridge.Client().GetWallet(ctx)
					if werr != nil {
						return werr
					}
					return delta.AssertBuyWithinBudget(bal, premium, 1)
				},
			})
		}
		if _, err := o.executeThroughInstitutionalPathWithFill(ctx, sig, open.StrategyName, price, execution.OrderModeMarket, fillFn, pathOpts...); err != nil {
			return err
		}
		bridge.UpdateTradeAfterFill(tradeID, captured, productID, contracts, open.Strike, open.ExpiryTime)
		bridge.RegisterOpenMapping(open.PaperTradeID, tradeID)
		return nil
	})
	bridge.SetInstitutionalCloseHandler(func(ctx context.Context, close delta.CloseSignal, trade delta.LiveTrade) error {
		o.mu.RLock()
		price := close.ExitBTCPrice
		if price <= 0 {
			price = o.lastPrice
		}
		o.mu.RUnlock()
		if price <= 0 {
			return fmt.Errorf("no reference price for institutional delta close")
		}
		buying := bridge.IsBuyingMode()
		closeSide := delta.SideBuy
		action := strategy.ActionBuy
		if buying {
			closeSide = delta.SideSell
			action = strategy.ActionSell
		}
		sig := strategy.Signal{
			Symbol:        fmt.Sprintf("DELTA-CLOSE:%s:%.0f", trade.OptionType, trade.Strike),
			Action:        action,
			TargetSize:    float64(trade.Contracts) * 0.001,
			Confidence:    0.95,
			StopLossPct:   defaultSignalStopLossPct,
			TakeProfitPct: minSignalTakeProfitPct,
		}
		if sig.TargetSize < 0.01 {
			sig.TargetSize = 0.01
		}
		strategyName := trade.StrategyName
		if strategyName == "" {
			strategyName = "DELTA_BRIDGE_CLOSE"
		}
		var captured delta.PlaceOrderResult
		fillFn := func(c context.Context, _ strategy.Signal, _ string) (execution.FillResult, error) {
			result, err := bridge.SubmitReduceOnlyOrder(c, trade.ProductID, closeSide, trade.Contracts)
			if err != nil {
				return execution.FillResult{}, err
			}
			captured = result
			return execution.FillResult{ExecPrice: result.Price, OrderMode: execution.OrderModeMarket}, nil
		}
		if _, err := o.executeThroughInstitutionalPathWithFill(ctx, sig, strategyName+"_CLOSE", price, execution.OrderModeMarket, fillFn); err != nil {
			return err
		}
		bridge.UpdateTradeAfterClose(trade.ID, captured, buying)
		return nil
	})
}
