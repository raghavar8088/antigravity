package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────
// SYSTEM PROMPTS — each agent has a distinct identity and role
// ─────────────────────────────────────────────────────────────────

const bullSystemPrompt = `You are the BULL AGENT for AntiGravity, an autonomous BTC scalping engine.
Your role: Analyze market data and make the case for LONG (buy) positions.
Be intellectually honest — if conditions are poor, say so. Quality over quantity.

CRITICAL: Respond ONLY with a valid JSON object. No markdown, no explanation outside the JSON.
JSON schema:
{
  "should_trade": boolean,
  "confidence": number (0.0 to 1.0),
  "thesis": "string — 2-3 sentences explaining the bull case",
  "size_btc": number (0.001 to 0.05),
  "stop_loss_pct": number (0.10 to 0.80),
  "take_profit_pct": number (0.30 to 2.00)
}`

const bearSystemPrompt = `You are the BEAR AGENT for AntiGravity, an autonomous BTC scalping engine.
Your role: Analyze market data and make the case for SHORT (sell) positions.
Be intellectually honest — if conditions are poor, say so. Quality over quantity.

CRITICAL: Respond ONLY with a valid JSON object. No markdown, no explanation outside the JSON.
JSON schema:
{
  "should_trade": boolean,
  "confidence": number (0.0 to 1.0),
  "thesis": "string — 2-3 sentences explaining the bear case",
  "size_btc": number (0.001 to 0.05),
  "stop_loss_pct": number (0.10 to 0.80),
  "take_profit_pct": number (0.30 to 2.00)
}`

const riskSystemPrompt = `You are the RISK AGENT for AntiGravity, an autonomous BTC scalping engine.
Your role: Review proposed trades against the Trading Constitution. Protect capital above all else.
You have final veto power. When in doubt, choose HOLD.

CRITICAL: Respond ONLY with a valid JSON object. No markdown, no explanation outside the JSON.
JSON schema:
{
  "approved": boolean,
  "approved_action": "BUY" or "SELL" or "HOLD",
  "veto_reason": "string or null",
  "reasoning": "1-2 sentence risk assessment",
  "adjusted_size": number (may reduce the proposed size for safety)
}`

// ─────────────────────────────────────────────────────────────────
// PROMPT BUILDER — formats MarketContext into Claude-readable text
// ─────────────────────────────────────────────────────────────────

func buildMarketPrompt(ctx MarketContext) string {
	candleLines := make([]string, 0, len(ctx.RecentCandles))
	for i, c := range ctx.RecentCandles {
		age := len(ctx.RecentCandles) - i
		candleLines = append(candleLines, fmt.Sprintf(
			"  [-%dm] O:%.0f H:%.0f L:%.0f C:%.0f V:%.3f",
			age*5, c.Open, c.High, c.Low, c.Close, c.Volume,
		))
	}

	directionStr := "FLAT"
	if ctx.EMAFast > ctx.EMASlow {
		directionStr = "BULLISH (fast EMA above slow)"
	} else if ctx.EMAFast < ctx.EMASlow {
		directionStr = "BEARISH (fast EMA below slow)"
	}

	priceVsVWAP := 0.0
	if ctx.VWAP > 0 {
		priceVsVWAP = ((ctx.Price - ctx.VWAP) / ctx.VWAP) * 100
	}

	return fmt.Sprintf(`MARKET SNAPSHOT — BTC/USDT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Price:        $%.2f
VWAP:         $%.2f (price is %+.3f%% vs VWAP)
Regime:       %s
Direction:    %s
ADX:          %.1f (trend strength: %s)
RSI(14):      %.1f (%s)
ATR(14):      $%.2f (%.3f%% of price)

Last %d × 5-minute candles (oldest → newest):
%s

ACCOUNT STATE
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Balance:           $%.2f
Daily PnL:         $%+.2f
Total PnL:         $%+.2f
Open Positions:    %d (Long: %d, Short: %d)
Consecutive Losses: %d`,
		ctx.Price,
		ctx.VWAP, priceVsVWAP,
		ctx.Regime,
		directionStr,
		ctx.ADX, adxLabel(ctx.ADX),
		ctx.RSI, rsiLabel(ctx.RSI),
		ctx.ATR, (ctx.ATR/ctx.Price)*100,
		len(ctx.RecentCandles),
		strings.Join(candleLines, "\n"),
		ctx.Balance,
		ctx.DailyPnL,
		ctx.TotalPnL,
		ctx.OpenPositions, ctx.LongPositions, ctx.ShortPositions,
		ctx.ConsecutiveLosses,
	)
}

func adxLabel(adx float64) string {
	switch {
	case adx >= 35:
		return "STRONG TREND"
	case adx >= 25:
		return "trending"
	case adx >= 20:
		return "weak trend"
	default:
		return "ranging/choppy"
	}
}

func rsiLabel(rsi float64) string {
	switch {
	case rsi >= 70:
		return "OVERBOUGHT"
	case rsi >= 60:
		return "bullish"
	case rsi <= 30:
		return "OVERSOLD"
	case rsi <= 40:
		return "bearish"
	default:
		return "neutral"
	}
}

// ─────────────────────────────────────────────────────────────────
// AGENT IMPLEMENTATIONS
// ─────────────────────────────────────────────────────────────────

type bullAgentResponse struct {
	ShouldTrade   bool    `json:"should_trade"`
	Confidence    float64 `json:"confidence"`
	Thesis        string  `json:"thesis"`
	SizeBTC       float64 `json:"size_btc"`
	StopLossPct   float64 `json:"stop_loss_pct"`
	TakeProfitPct float64 `json:"take_profit_pct"`
}

type bearAgentResponse struct {
	ShouldTrade   bool    `json:"should_trade"`
	Confidence    float64 `json:"confidence"`
	Thesis        string  `json:"thesis"`
	SizeBTC       float64 `json:"size_btc"`
	StopLossPct   float64 `json:"stop_loss_pct"`
	TakeProfitPct float64 `json:"take_profit_pct"`
}

type riskAgentResponse struct {
	Approved       bool    `json:"approved"`
	ApprovedAction string  `json:"approved_action"`
	VetoReason     string  `json:"veto_reason"`
	Reasoning      string  `json:"reasoning"`
	AdjustedSize   float64 `json:"adjusted_size"`
}

func runBullAgent(ctx context.Context, client *ClaudeClient, market MarketContext) AgentSignal {
	prompt := buildMarketPrompt(market) + "\n\nShould we open a LONG (BUY) position right now? Be rigorous."
	raw, err := client.ChatForSignal(ctx, bullSystemPrompt, prompt)
	if err != nil {
		return AgentSignal{Role: RoleBull, Error: err.Error()}
	}

	raw = extractJSON(raw)
	var resp bullAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return AgentSignal{Role: RoleBull, Error: fmt.Sprintf("parse error: %v | raw: %s", err, raw)}
	}

	return AgentSignal{
		Role:          RoleBull,
		ShouldTrade:   resp.ShouldTrade,
		Confidence:    resp.Confidence,
		Thesis:        resp.Thesis,
		SizeBTC:       resp.SizeBTC,
		StopLossPct:   resp.StopLossPct,
		TakeProfitPct: resp.TakeProfitPct,
	}
}

func runBearAgent(ctx context.Context, client *ClaudeClient, market MarketContext) AgentSignal {
	prompt := buildMarketPrompt(market) + "\n\nShould we open a SHORT (SELL) position right now? Be rigorous."
	raw, err := client.ChatForSignal(ctx, bearSystemPrompt, prompt)
	if err != nil {
		return AgentSignal{Role: RoleBear, Error: err.Error()}
	}

	raw = extractJSON(raw)
	var resp bearAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return AgentSignal{Role: RoleBear, Error: fmt.Sprintf("parse error: %v | raw: %s", err, raw)}
	}

	return AgentSignal{
		Role:          RoleBear,
		ShouldTrade:   resp.ShouldTrade,
		Confidence:    resp.Confidence,
		Thesis:        resp.Thesis,
		SizeBTC:       resp.SizeBTC,
		StopLossPct:   resp.StopLossPct,
		TakeProfitPct: resp.TakeProfitPct,
	}
}

func runRiskAgent(ctx context.Context, client *ClaudeClient, bull, bear AgentSignal, market MarketContext) RiskVerdict {
	bullJSON, _ := json.MarshalIndent(bull, "", "  ")
	bearJSON, _ := json.MarshalIndent(bear, "", "  ")

	prompt := fmt.Sprintf(`%s

TRADING CONSTITUTION:
%s

BULL AGENT PROPOSAL:
%s

BEAR AGENT PROPOSAL:
%s

Review both proposals. Approve the stronger one if it passes the constitution. Veto both if neither qualifies.`,
		buildMarketPrompt(market),
		ConstitutionRules(),
		string(bullJSON),
		string(bearJSON),
	)

	raw, err := client.ChatForRisk(ctx, riskSystemPrompt, prompt)
	if err != nil {
		return RiskVerdict{
			Approved:       false,
			ApprovedAction: "HOLD",
			Reasoning:      "Risk agent unavailable: " + err.Error(),
			Error:          err.Error(),
		}
	}

	raw = extractJSON(raw)
	var resp riskAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return RiskVerdict{
			Approved:       false,
			ApprovedAction: "HOLD",
			Reasoning:      fmt.Sprintf("Risk agent parse error: %v", err),
			Error:          err.Error(),
		}
	}

	return RiskVerdict{
		Approved:       resp.Approved,
		ApprovedAction: resp.ApprovedAction,
		VetoReason:     resp.VetoReason,
		Reasoning:      resp.Reasoning,
		AdjustedSize:   resp.AdjustedSize,
	}
}

// ─────────────────────────────────────────────────────────────────
// MULTI-AGENT ORCHESTRATOR
// ─────────────────────────────────────────────────────────────────

// MultiAgentOrchestrator runs the Bull → Bear → Risk agent debate
// and produces a final AIDecision on every 5m candle close.
type MultiAgentOrchestrator struct {
	client   *ClaudeClient
	insights *InsightStore
	mu       sync.Mutex
	idSeq    int
}

func NewMultiAgentOrchestrator(client *ClaudeClient) *MultiAgentOrchestrator {
	return &MultiAgentOrchestrator{
		client:   client,
		insights: NewInsightStore(50),
	}
}

func (o *MultiAgentOrchestrator) IsAvailable() bool {
	return o != nil && o.client != nil && o.client.IsAvailable()
}

func (o *MultiAgentOrchestrator) GetInsights() *InsightStore {
	return o.insights
}

// Decide runs all three agents in the correct order and returns the final decision.
// Bull and Bear run in parallel; Risk runs after both complete.
func (o *MultiAgentOrchestrator) Decide(ctx context.Context, market MarketContext) AIDecision {
	start := time.Now()

	// Give agents a deadline — if Claude is slow, don't block the engine
	agentCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	// ── Step 1: Bull and Bear run in parallel ──
	var (
		bullSig AgentSignal
		bearSig AgentSignal
		wg      sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		bullSig = runBullAgent(agentCtx, o.client, market)
	}()
	go func() {
		defer wg.Done()
		bearSig = runBearAgent(agentCtx, o.client, market)
	}()
	wg.Wait()

	log.Printf("[AI] Bull: trade=%v conf=%.2f | Bear: trade=%v conf=%.2f (%.0fms)",
		bullSig.ShouldTrade, bullSig.Confidence,
		bearSig.ShouldTrade, bearSig.Confidence,
		float64(time.Since(start).Milliseconds()),
	)

	// If neither agent wants to trade, skip Risk (save API cost)
	if !bullSig.ShouldTrade && !bearSig.ShouldTrade {
		decision := o.buildDecision(market, bullSig, bearSig, RiskVerdict{
			Approved:       false,
			ApprovedAction: "HOLD",
			Reasoning:      "Both Bull and Bear agents recommend HOLD.",
		})
		o.insights.Add(decision)
		log.Printf("[AI] Decision: HOLD (both agents quiet) — %.0fms total", float64(time.Since(start).Milliseconds()))
		return decision
	}

	// ── Step 2: Risk Agent arbitrates ──
	riskVerdict := runRiskAgent(agentCtx, o.client, bullSig, bearSig, market)
	log.Printf("[AI] Risk: approved=%v action=%s (%.0fms total)",
		riskVerdict.Approved, riskVerdict.ApprovedAction,
		float64(time.Since(start).Milliseconds()),
	)

	decision := o.buildDecision(market, bullSig, bearSig, riskVerdict)
	o.insights.Add(decision)
	return decision
}

func (o *MultiAgentOrchestrator) buildDecision(
	market MarketContext,
	bull AgentSignal,
	bear AgentSignal,
	risk RiskVerdict,
) AIDecision {
	o.mu.Lock()
	o.idSeq++
	id := fmt.Sprintf("AI-%d", o.idSeq)
	o.mu.Unlock()

	action := "HOLD"
	if risk.Approved {
		action = risk.ApprovedAction
	}

	// Build a human-readable summary of the full debate
	reasoning := fmt.Sprintf(
		"BULL [conf:%.0f%%]: %s\n\nBEAR [conf:%.0f%%]: %s\n\nRISK: %s",
		bull.Confidence*100, bull.Thesis,
		bear.Confidence*100, bear.Thesis,
		risk.Reasoning,
	)
	if risk.VetoReason != "" {
		reasoning += fmt.Sprintf("\n\n⛔ VETO: %s", risk.VetoReason)
	}

	return AIDecision{
		ID:             id,
		Timestamp:      time.Now(),
		Price:          market.Price,
		BullSignal:     bull,
		BearSignal:     bear,
		RiskVerdict:    risk,
		FinalAction:    action,
		FinalReasoning: reasoning,
		Regime:         market.Regime,
	}
}

// extractJSON pulls the JSON object out of a Claude response that may contain
// markdown code fences or surrounding text.
func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	// Remove ```json ... ``` wrapping
	if idx := strings.Index(raw, "```json"); idx != -1 {
		raw = raw[idx+7:]
		if end := strings.Index(raw, "```"); end != -1 {
			raw = raw[:end]
		}
	} else if idx := strings.Index(raw, "```"); idx != -1 {
		raw = raw[idx+3:]
		if end := strings.Index(raw, "```"); end != -1 {
			raw = raw[:end]
		}
	}
	// Find the outermost { ... }
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		return raw[start : end+1]
	}
	return raw
}
