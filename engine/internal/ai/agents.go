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

const macroSystemPrompt = `You are the MACRO ANALYST AGENT for AntiGravity, an autonomous BTC scalping engine.
Your role: Provide an independent top-down macro and market-structure perspective.
You do NOT advocate for a specific trade direction — you assess the OVERALL CONDITIONS.
Consider: trend regime, momentum exhaustion, risk-on/risk-off environment, and whether the
current setup is favorable for short-term scalping at all.

CRITICAL: Respond ONLY with a valid JSON object. No markdown, no explanation outside the JSON.
JSON schema:
{
  "should_trade": boolean,
  "confidence": number (0.0 to 1.0, how favorable is the macro backdrop for ANY scalp?),
  "thesis": "string — 2-3 sentences top-down assessment of macro conditions",
  "bias": "BULLISH" or "BEARISH" or "NEUTRAL",
  "size_btc": number (0.001 to 0.05, suggested max size given macro risk),
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

const auditSystemPrompt = `You are the SENIOR SIGNAL AUDITOR for AntiGravity, an autonomous BTC scalping engine.
Your role: Review a proposed signal from a manual technical strategy (e.g., EMA Cross, RSI).
Decide if the signal is high-probability or a "trap" based on the provided market context.

Criteria for VETO:
- High RSI (+70) for a BUY signal, or Low RSI (-30) for a SELL.
- Moving Average misalignment (e.g. BUY signal when price is below both EMAs).
- Low volatility (ATR) making the spread/fees non-viable.
- Contradicting the Macro Analyst's bias.

CRITICAL: Respond ONLY with a valid JSON object. No markdown.
JSON schema:
{
  "approved": boolean,
  "confidence": number (0.0 to 1.0),
  "reason": "1 sentence explanation of why you approved or vetoed"
}`

// ─────────────────────────────────────────────────────────────────
// PROMPT BUILDER — formats MarketContext into LLM-readable text
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
// AGENT IMPLEMENTATIONS (OpenAI GPT-4o)
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

type macroAgentResponse struct {
	ShouldTrade   bool    `json:"should_trade"`
	Confidence    float64 `json:"confidence"`
	Thesis        string  `json:"thesis"`
	Bias          string  `json:"bias"` // "BULLISH", "BEARISH", "NEUTRAL"
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

type auditAgentResponse struct {
	Approved   bool    `json:"approved"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

func (o *MultiAgentOrchestrator) AuditSignal(ctx context.Context, market MarketContext, strategyName string, action string) (bool, string, float64) {
	start := time.Now()
	prompt := fmt.Sprintf("%s\n\nPROPOSED SIGNAL:\nStrategy: %s\nAction: %s\n\nAudit this signal. Should we execute it? Be strict.", 
		buildMarketPrompt(market), strategyName, action)
	
	raw, err := o.openai.ChatForAudit(ctx, auditSystemPrompt, prompt)
	if err != nil {
		log.Printf("[AI AUDIT ERROR] %v", err)
		return true, fmt.Sprintf("Audit error: %v", err), 0.5
	}

	raw = extractJSON(raw)
	var resp auditAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return true, "Audit parse error", 0.4
	}

	log.Printf("[AI AUDIT] %s %s -> %v | %s (%.0fms)", 
		strategyName, action, resp.Approved, resp.Reason, float64(time.Since(start).Milliseconds()))

	o.insights.AddAudit(AuditLog{
		ID:           fmt.Sprintf("AUD-%d", time.Now().UnixNano()/1e6),
		StrategyName: strategyName,
		Action:       action,
		Approved:     resp.Approved,
		Reason:       resp.Reason,
		Confidence:   resp.Confidence,
		Timestamp:    time.Now(),
	})

	return resp.Approved, resp.Reason, resp.Confidence
}

// AuditSignalWithFallback tries OpenAI first, then Groq as a free fallback.
// It also enforces a throttle to stay within free tier rate limits.
func (o *MultiAgentOrchestrator) AuditSignalWithFallback(ctx context.Context, market MarketContext, strategyName string, action string) (bool, string, float64) {
	// 1. Throttling: stay below 15 req/min (1 req every 4.2s to be safe)
	o.auditMu.Lock()
	defer func() {
		time.Sleep(4200 * time.Millisecond)
		o.auditMu.Unlock()
	}()

	// 2. Try OpenAI (paid/high quality)
	if o.openai.IsAvailable() {
		approved, reason, conf := o.AuditSignal(ctx, market, strategyName, action)
		// If it's a quota/billing error, fall through to Groq
		lowReason := strings.ToLower(reason)
		if !strings.Contains(lowReason, "quota") && !strings.Contains(lowReason, "billing") && !strings.Contains(lowReason, "error") {
			return approved, reason, conf
		}
		log.Printf("[AI AUDIT FALLBACK] OpenAI Quota/Error hit — switching to Free Auditor (Groq/Llama-3)...")
	}

	// 3. Try Groq (Free fallback)
	if o.groq.IsAvailable() {
		return o.runGroqAudit(ctx, market, strategyName, action)
	}

	// 4. Final Fallback (Neutral)
	return true, "No AI Auditor available (running technicals only)", 0.5
}

func (o *MultiAgentOrchestrator) runGroqAudit(ctx context.Context, market MarketContext, strategyName string, action string) (bool, string, float64) {
	start := time.Now()
	prompt := fmt.Sprintf("%s\n\nPROPOSED SIGNAL:\nStrategy: %s\nAction: %s\n\nAudit this signal. Be strict.", 
		buildMarketPrompt(market), strategyName, action)
	
	raw, err := o.groq.ChatForAudit(ctx, auditSystemPrompt, prompt)
	if err != nil {
		log.Printf("[AI AUDIT GROQ] Error: %v", err)
		return true, "Groq audit failed (neutral)", 0.5
	}

	raw = extractJSON(raw)
	var resp auditAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return true, "Groq parse error (neutral)", 0.4
	}

	log.Printf("[AI AUDIT ✅ GROQ] %s %s -> %v | %s (%.0fms)", 
		strategyName, action, resp.Approved, resp.Reason, float64(time.Since(start).Milliseconds()))

	o.insights.AddAudit(AuditLog{
		ID:           fmt.Sprintf("AUD-G-%d", time.Now().UnixNano()/1e6),
		StrategyName: strategyName,
		Action:       action,
		Approved:     resp.Approved,
		Reason:       "[⚡ Groq-Free] " + resp.Reason,
		Confidence:   resp.Confidence,
		Timestamp:    time.Now(),
	})

	return resp.Approved, resp.Reason, resp.Confidence
}

func runBullAgent(ctx context.Context, client *OpenAIClient, market MarketContext) AgentSignal {
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

func runBearAgent(ctx context.Context, client *OpenAIClient, market MarketContext) AgentSignal {
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

func runMacroAgent(ctx context.Context, gemini *GeminiClient, market MarketContext) AgentSignal {
	if !gemini.IsAvailable() {
		return AgentSignal{
			Role:        RoleMacro,
			ShouldTrade: false,
			Confidence:  0,
			Thesis:      "Gemini Macro Agent disabled (GEMINI_API_KEY not set)",
			Error:       "GEMINI_API_KEY not configured",
		}
	}

	prompt := buildMarketPrompt(market) + "\n\nProvide your top-down macro assessment. Is the backdrop favorable for scalping right now?"
	raw, err := gemini.ChatForMacro(ctx, macroSystemPrompt, prompt)
	if err != nil {
		return AgentSignal{Role: RoleMacro, Error: err.Error()}
	}

	raw = extractJSON(raw)
	var resp macroAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return AgentSignal{Role: RoleMacro, Error: fmt.Sprintf("parse error: %v | raw: %s", err, raw)}
	}

	thesis := resp.Thesis
	if resp.Bias != "" {
		thesis = fmt.Sprintf("[%s] %s", resp.Bias, resp.Thesis)
	}

	return AgentSignal{
		Role:          RoleMacro,
		ShouldTrade:   resp.ShouldTrade,
		Confidence:    resp.Confidence,
		Thesis:        thesis,
		SizeBTC:       resp.SizeBTC,
		StopLossPct:   resp.StopLossPct,
		TakeProfitPct: resp.TakeProfitPct,
	}
}

func runRiskAgent(ctx context.Context, client *OpenAIClient, bull, bear, macro AgentSignal, market MarketContext) RiskVerdict {
	bullJSON, _ := json.MarshalIndent(bull, "", "  ")
	bearJSON, _ := json.MarshalIndent(bear, "", "  ")
	macroJSON, _ := json.MarshalIndent(macro, "", "  ")

	macroSection := ""
	if macro.Error == "" {
		macroSection = fmt.Sprintf("\nMACRO ANALYST (Gemini) ASSESSMENT:\n%s\n", string(macroJSON))
	}

	prompt := fmt.Sprintf(`%s

TRADING CONSTITUTION:
%s

BULL AGENT PROPOSAL:
%s

BEAR AGENT PROPOSAL:
%s
%s
Review all proposals. The Macro Agent gives the overall backdrop — use it to increase or decrease
conviction. Approve the strongest directional signal if it passes the constitution. Veto all if
neither qualifies or macro conditions are too adverse.`,
		buildMarketPrompt(market),
		ConstitutionRules(),
		string(bullJSON),
		string(bearJSON),
		macroSection,
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

type MultiAgentOrchestrator struct {
	openai   *OpenAIClient
	gemini   *GeminiClient
	groq     *GroqClient
	insights *InsightStore
	mu       sync.Mutex
	auditMu  sync.Mutex // For throttling AI audits
	idSeq    int
}

func NewMultiAgentOrchestrator(openai *OpenAIClient, gemini *GeminiClient, groq *GroqClient) *MultiAgentOrchestrator {
	return &MultiAgentOrchestrator{
		openai:   openai,
		gemini:   gemini,
		groq:     groq,
		insights: NewInsightStore(50),
	}
}

func (o *MultiAgentOrchestrator) IsAvailable() bool {
	return o != nil && o.openai != nil && o.openai.IsAvailable()
}

func (o *MultiAgentOrchestrator) GeminiEnabled() bool {
	return o != nil && o.gemini != nil && o.gemini.IsAvailable()
}

func (o *MultiAgentOrchestrator) GetInsights() *InsightStore {
	return o.insights
}

func (o *MultiAgentOrchestrator) Decide(ctx context.Context, market MarketContext) AIDecision {
	start := time.Now()

	agentCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var (
		bullSig  AgentSignal
		bearSig  AgentSignal
		macroSig AgentSignal
		wg       sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		bullSig = runBullAgent(agentCtx, o.openai, market)
	}()
	go func() {
		defer wg.Done()
		bearSig = runBearAgent(agentCtx, o.openai, market)
	}()
	go func() {
		defer wg.Done()
		macroSig = runMacroAgent(agentCtx, o.gemini, market)
	}()
	wg.Wait()

	log.Printf("[AI] OpenAI Bull: trade=%v conf=%.2f | Bear: trade=%v conf=%.2f | Gemini Macro: trade=%v conf=%.2f bias=%s (%.0fms)",
		bullSig.ShouldTrade, bullSig.Confidence,
		bearSig.ShouldTrade, bearSig.Confidence,
		macroSig.ShouldTrade, macroSig.Confidence,
		macroSig.Thesis,
		float64(time.Since(start).Milliseconds()),
	)

	if !bullSig.ShouldTrade && !bearSig.ShouldTrade && !macroSig.ShouldTrade {
		decision := o.buildDecision(market, bullSig, bearSig, macroSig, RiskVerdict{
			Approved:       false,
			ApprovedAction: "HOLD",
			Reasoning:      "Council (OpenAI+Gemini) recommended HOLD.",
		})
		o.insights.Add(decision)
		return decision
	}

	riskVerdict := runRiskAgent(agentCtx, o.openai, bullSig, bearSig, macroSig, market)
	log.Printf("[AI] Risk: approved=%v action=%s (%.0fms total)",
		riskVerdict.Approved, riskVerdict.ApprovedAction,
		float64(time.Since(start).Milliseconds()),
	)

	decision := o.buildDecision(market, bullSig, bearSig, macroSig, riskVerdict)
	o.insights.Add(decision)
	return decision
}

func (o *MultiAgentOrchestrator) buildDecision(
	market MarketContext,
	bull AgentSignal,
	bear AgentSignal,
	macro AgentSignal,
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

	macroLine := ""
	if macro.Error == "" {
		macroLine = fmt.Sprintf("\n\nMACRO [Gemini, conf:%.0f%%]: %s", macro.Confidence*100, macro.Thesis)
	}

	reasoning := fmt.Sprintf(
		"BULL [conf:%.0f%%]: %s\n\nBEAR [conf:%.0f%%]: %s%s\n\nRISK: %s",
		bull.Confidence*100, bull.Thesis,
		bear.Confidence*100, bear.Thesis,
		macroLine,
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
		MacroSignal:    macro,
		RiskVerdict:    risk,
		FinalAction:    action,
		FinalReasoning: reasoning,
		Regime:         market.Regime,
	}
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
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
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		return raw[start : end+1]
	}
	return raw
}
