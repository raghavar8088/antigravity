package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"antigravity-engine/internal/persistence"
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

const auditSystemPrompt = `You are the SENIOR SIGNAL AUDITOR for AntiGravity. 
Your role: Review a proposed signal from a manual technical strategy (e.g., EMA Cross, RSI).
Decide if the signal is high-probability or a "trap" based on the provided market context.

Criteria for VETO:
- RSI exhaustion (+70 for BUY, -30 for SELL).
- Moving Average misalignment.
- Macro bias contradiction.

CRITICAL: Respond ONLY with a valid JSON object.
{
  "approved": boolean,
  "confidence": number,
  "reason": "string"
}`

const batchAuditSystemPrompt = `You are the SENIOR SIGNAL AUDITOR for AntiGravity. 
Role: Review a BATCH of strategy signals. Decide which should be APPROVED and which VETOED.

CRITICAL: Respond ONLY with a valid JSON array of objects.
[
  {"strategy": "EMA_Cross", "approved": true, "reason": "..."},
  {"strategy": "RSI_Overbought", "approved": false, "reason": "..."}
]`

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

	log.Printf("[AI AUDIT ✅ OPENAI] %s %s -> %v | %s (%.0fms)", 
		strategyName, action, resp.Approved, resp.Reason, float64(time.Since(start).Milliseconds()))

	auditID := fmt.Sprintf("AUD-OA-%d", time.Now().UnixNano()/1e6)
	o.insights.AddAudit(AuditLog{
		ID:           auditID,
		StrategyName: strategyName,
		Action:       action,
		Approved:     resp.Approved,
		Reason:       resp.Reason,
		Confidence:   resp.Confidence,
		Timestamp:    time.Now(),
	})

	if o.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = o.store.SaveAuditLog(ctx, auditID, strategyName, action, resp.Approved, resp.Reason, resp.Confidence, "openai")
	}

	return resp.Approved, resp.Reason, resp.Confidence
}

// AuditSignalWithFallback tries all 5 providers in priority order.
// Free providers are tried first to minimise cost.
// It returns (approved, reason, confidence, provider).
func (o *MultiAgentOrchestrator) AuditSignalWithFallback(ctx context.Context, market MarketContext, strategyName string, action string) (bool, string, float64, string) {
	anyAvailable := o.groq.IsAvailable() || o.gemini.IsAvailable() || o.mistral.IsAvailable() ||
		o.huggingface.IsAvailable() || o.cloudflare.IsAvailable() || o.openrouter.IsAvailable() || o.openai.IsAvailable()
	if !anyAvailable {
		return true, "No AI Auditor available (running technicals only)", 0.5, "NONE"
	}

	o.auditMu.Lock()
	defer func() {
		time.Sleep(4200 * time.Millisecond) // Throttle to respect free-tier rate limits
		o.auditMu.Unlock()
	}()

	// 1. Groq — fastest free tier (Llama 3 70B, 14,400 req/day)
	if o.groq.IsAvailable() {
		approved, reason, conf := o.runGroqAudit(ctx, market, strategyName, action)
		if !isFatalError(reason) {
			return approved, reason, conf, "groq"
		}
		log.Printf("[AI AUDIT FALLBACK] Groq failed -> trying Gemini...")
	}

	// 2. Gemini — strong reasoning (1,500 req/day free)
	if o.gemini.IsAvailable() {
		approved, reason, conf := o.runGeminiAudit(ctx, market, strategyName, action)
		if !isFatalError(reason) {
			return approved, reason, conf, "gemini"
		}
		log.Printf("[AI AUDIT FALLBACK] Gemini failed -> trying Mistral...")
	}

	// 3. Mistral — reliable free tier (mistral-small)
	if o.mistral.IsAvailable() {
		approved, reason, conf := o.runMistralAudit(ctx, market, strategyName, action)
		if !isFatalError(reason) {
			return approved, reason, conf, "mistral"
		}
		log.Printf("[AI AUDIT FALLBACK] Mistral failed -> trying OpenRouter...")
	}

	// 4. HuggingFace — Qwen2.5-72B free
	if o.huggingface.IsAvailable() {
		approved, reason, conf := o.runHuggingFaceAudit(ctx, market, strategyName, action)
		if !isFatalError(reason) {
			return approved, reason, conf, "huggingface"
		}
		log.Printf("[AI AUDIT FALLBACK] HuggingFace failed -> trying Cloudflare...")
	}

	// 5. Cloudflare Workers AI — Llama-3.1-70B free
	if o.cloudflare.IsAvailable() {
		approved, reason, conf := o.runCloudflareAudit(ctx, market, strategyName, action)
		if !isFatalError(reason) {
			return approved, reason, conf, "cloudflare"
		}
		log.Printf("[AI AUDIT FALLBACK] Cloudflare failed -> trying OpenRouter...")
	}

	// 6. OpenRouter — 20+ free models as backstop
	if o.openrouter.IsAvailable() {
		approved, reason, conf := o.runOpenRouterAudit(ctx, market, strategyName, action)
		if !isFatalError(reason) {
			return approved, reason, conf, "openrouter"
		}
		log.Printf("[AI AUDIT FALLBACK] OpenRouter failed -> trying OpenAI...")
	}

	// 6. OpenAI — paid, last resort
	if o.openai.IsAvailable() {
		approved, reason, conf := o.AuditSignal(ctx, market, strategyName, action)
		if !isFatalError(reason) {
			return approved, reason, conf, "openai"
		}
	}

	return true, "All AI providers exhausted (neutral pass)", 0.5, "NONE"
}

func (o *MultiAgentOrchestrator) runOpenRouterAudit(ctx context.Context, market MarketContext, strategyName string, action string) (bool, string, float64) {
	start := time.Now()
	prompt := fmt.Sprintf("%s\n\nPROPOSED SIGNAL:\nStrategy: %s\nAction: %s\n\nAudit this. Be strict.", 
		buildMarketPrompt(market), strategyName, action)
	
	raw, err := o.openrouter.ChatForAudit(ctx, auditSystemPrompt, prompt)
	if err != nil {
		log.Printf("[AI AUDIT OPENROUTER] Error: %v", err)
		return true, "Audit error (neutral)", 0.5
	}

	raw = extractJSON(raw)
	var resp auditAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return true, "Parse error", 0.4
	}

	log.Printf("[AI AUDIT ✅ OPENROUTER] %s %s -> %v | %s (%.0fms)", 
		strategyName, action, resp.Approved, resp.Reason, float64(time.Since(start).Milliseconds()))

	auditID := fmt.Sprintf("AUD-OR-%d", time.Now().UnixNano()/1e6)
	o.insights.AddAudit(AuditLog{
		ID:           auditID,
		StrategyName: strategyName,
		Action:       action,
		Approved:     resp.Approved,
		Reason:       "[🌐 OpenRouter] " + resp.Reason,
		Confidence:   resp.Confidence,
		Timestamp:    time.Now(),
	})

	if o.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = o.store.SaveAuditLog(ctx, auditID, strategyName, action, resp.Approved, "[🌐 OpenRouter] "+resp.Reason, resp.Confidence, "openrouter")
	}

	return resp.Approved, resp.Reason, resp.Confidence
}

func isFatalError(reason string) bool {
	low := strings.ToLower(reason)
	return strings.Contains(low, "quota") || strings.Contains(low, "billing") || strings.Contains(low, "error") || strings.Contains(low, "status 4")
}

// AuditBatchSignals vets multiple signals in a single AI call for massive efficiency.
func (o *MultiAgentOrchestrator) AuditBatchSignals(ctx context.Context, market MarketContext, signals []string) map[string]bool {
	if len(signals) == 0 {
		return nil
	}
	if len(signals) == 1 {
		approved, _, _, _ := o.AuditSignalWithFallback(ctx, market, signals[0], "BUY/SELL")
		return map[string]bool{signals[0]: approved}
	}

	// For simplicity in this version, we'll run them in parallel but throttled.
	// Future optimization: use batchAuditSystemPrompt for true single-call batching.
	results := make(map[string]bool)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, sigName := range signals {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			approved, _, _, _ := o.AuditSignalWithFallback(ctx, market, name, "BUY/SELL")
			mu.Lock()
			results[name] = approved
			mu.Unlock()
		}(sigName)
	}

	wg.Wait()
	return results
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

	auditID := fmt.Sprintf("AUD-G-%d", time.Now().UnixNano()/1e6)
	o.insights.AddAudit(AuditLog{
		ID:           auditID,
		StrategyName: strategyName,
		Action:       action,
		Approved:     resp.Approved,
		Reason:       "[⚡ Groq-Free] " + resp.Reason,
		Confidence:   resp.Confidence,
		Timestamp:    time.Now(),
	})

	if o.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = o.store.SaveAuditLog(ctx, auditID, strategyName, action, resp.Approved, "[⚡ Groq-Free] "+resp.Reason, resp.Confidence, "groq")
	}

	return resp.Approved, resp.Reason, resp.Confidence
}

func (o *MultiAgentOrchestrator) runGeminiAudit(ctx context.Context, market MarketContext, strategyName string, action string) (bool, string, float64) {
	start := time.Now()
	prompt := fmt.Sprintf("%s\n\nPROPOSED SIGNAL:\nStrategy: %s\nAction: %s\n\nAudit this signal. Be strict.",
		buildMarketPrompt(market), strategyName, action)

	raw, err := o.gemini.ChatForRisk(ctx, auditSystemPrompt, prompt)
	if err != nil {
		log.Printf("[AI AUDIT GEMINI] Error: %v", err)
		return true, "gemini error: " + err.Error(), 0.5
	}

	raw = extractJSON(raw)
	var resp auditAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return true, "gemini parse error (neutral)", 0.4
	}

	log.Printf("[AI AUDIT ✅ GEMINI] %s %s -> %v | %s (%.0fms)",
		strategyName, action, resp.Approved, resp.Reason, float64(time.Since(start).Milliseconds()))

	auditID := fmt.Sprintf("AUD-GM-%d", time.Now().UnixNano()/1e6)
	o.insights.AddAudit(AuditLog{
		ID:           auditID,
		StrategyName: strategyName,
		Action:       action,
		Approved:     resp.Approved,
		Reason:       "[♊ Gemini] " + resp.Reason,
		Confidence:   resp.Confidence,
		Timestamp:    time.Now(),
		Provider:     "gemini",
	})

	if o.store != nil {
		sCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = o.store.SaveAuditLog(sCtx, auditID, strategyName, action, resp.Approved, "[♊ Gemini] "+resp.Reason, resp.Confidence, "gemini")
	}

	return resp.Approved, resp.Reason, resp.Confidence
}

func (o *MultiAgentOrchestrator) runHuggingFaceAudit(ctx context.Context, market MarketContext, strategyName string, action string) (bool, string, float64) {
	start := time.Now()
	prompt := fmt.Sprintf("%s\n\nPROPOSED SIGNAL:\nStrategy: %s\nAction: %s\n\nAudit this signal. Be strict.",
		buildMarketPrompt(market), strategyName, action)

	raw, err := o.huggingface.ChatForAudit(ctx, auditSystemPrompt, prompt)
	if err != nil {
		log.Printf("[AI AUDIT HUGGINGFACE] Error: %v", err)
		return true, "huggingface error: " + err.Error(), 0.5
	}

	raw = extractJSON(raw)
	var resp auditAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return true, "huggingface parse error (neutral)", 0.4
	}

	log.Printf("[AI AUDIT ✅ HUGGINGFACE] %s %s -> %v | %s (%.0fms)",
		strategyName, action, resp.Approved, resp.Reason, float64(time.Since(start).Milliseconds()))

	auditID := fmt.Sprintf("AUD-HF-%d", time.Now().UnixNano()/1e6)
	o.insights.AddAudit(AuditLog{
		ID:           auditID,
		StrategyName: strategyName,
		Action:       action,
		Approved:     resp.Approved,
		Reason:       "[🤗 HuggingFace] " + resp.Reason,
		Confidence:   resp.Confidence,
		Timestamp:    time.Now(),
		Provider:     "huggingface",
	})

	if o.store != nil {
		sCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = o.store.SaveAuditLog(sCtx, auditID, strategyName, action, resp.Approved, "[🤗 HuggingFace] "+resp.Reason, resp.Confidence, "huggingface")
	}

	return resp.Approved, resp.Reason, resp.Confidence
}

func (o *MultiAgentOrchestrator) runCloudflareAudit(ctx context.Context, market MarketContext, strategyName string, action string) (bool, string, float64) {
	start := time.Now()
	prompt := fmt.Sprintf("%s\n\nPROPOSED SIGNAL:\nStrategy: %s\nAction: %s\n\nAudit this signal. Be strict.",
		buildMarketPrompt(market), strategyName, action)

	raw, err := o.cloudflare.ChatForAudit(ctx, auditSystemPrompt, prompt)
	if err != nil {
		log.Printf("[AI AUDIT CLOUDFLARE] Error: %v", err)
		return true, "cloudflare error: " + err.Error(), 0.5
	}

	raw = extractJSON(raw)
	var resp auditAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return true, "cloudflare parse error (neutral)", 0.4
	}

	log.Printf("[AI AUDIT ✅ CLOUDFLARE] %s %s -> %v | %s (%.0fms)",
		strategyName, action, resp.Approved, resp.Reason, float64(time.Since(start).Milliseconds()))

	auditID := fmt.Sprintf("AUD-CF-%d", time.Now().UnixNano()/1e6)
	o.insights.AddAudit(AuditLog{
		ID:           auditID,
		StrategyName: strategyName,
		Action:       action,
		Approved:     resp.Approved,
		Reason:       "[☁️ Cloudflare] " + resp.Reason,
		Confidence:   resp.Confidence,
		Timestamp:    time.Now(),
		Provider:     "cloudflare",
	})

	if o.store != nil {
		sCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = o.store.SaveAuditLog(sCtx, auditID, strategyName, action, resp.Approved, "[☁️ Cloudflare] "+resp.Reason, resp.Confidence, "cloudflare")
	}

	return resp.Approved, resp.Reason, resp.Confidence
}

func (o *MultiAgentOrchestrator) runMistralAudit(ctx context.Context, market MarketContext, strategyName string, action string) (bool, string, float64) {
	start := time.Now()
	prompt := fmt.Sprintf("%s\n\nPROPOSED SIGNAL:\nStrategy: %s\nAction: %s\n\nAudit this signal. Be strict.",
		buildMarketPrompt(market), strategyName, action)

	raw, err := o.mistral.ChatForAudit(ctx, auditSystemPrompt, prompt)
	if err != nil {
		log.Printf("[AI AUDIT MISTRAL] Error: %v", err)
		return true, "mistral error: " + err.Error(), 0.5
	}

	raw = extractJSON(raw)
	var resp auditAgentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return true, "mistral parse error (neutral)", 0.4
	}

	log.Printf("[AI AUDIT ✅ MISTRAL] %s %s -> %v | %s (%.0fms)",
		strategyName, action, resp.Approved, resp.Reason, float64(time.Since(start).Milliseconds()))

	auditID := fmt.Sprintf("AUD-MS-%d", time.Now().UnixNano()/1e6)
	o.insights.AddAudit(AuditLog{
		ID:           auditID,
		StrategyName: strategyName,
		Action:       action,
		Approved:     resp.Approved,
		Reason:       "[🌬 Mistral] " + resp.Reason,
		Confidence:   resp.Confidence,
		Timestamp:    time.Now(),
		Provider:     "mistral",
	})

	if o.store != nil {
		sCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = o.store.SaveAuditLog(sCtx, auditID, strategyName, action, resp.Approved, "[🌬 Mistral] "+resp.Reason, resp.Confidence, "mistral")
	}

	return resp.Approved, resp.Reason, resp.Confidence
}

// ─────────────────────────────────────────────────────────────────
// AGENT INTERFACES
// ─────────────────────────────────────────────────────────────────

type SignalClient interface {
	ChatForSignal(ctx context.Context, system, prompt string) (string, error)
	IsAvailable() bool
}

type RiskClient interface {
	ChatForRisk(ctx context.Context, system, prompt string) (string, error)
	IsAvailable() bool
}

type MacroClient interface {
	ChatForMacro(ctx context.Context, system, prompt string) (string, error)
	IsAvailable() bool
}

func runBullAgent(ctx context.Context, client SignalClient, market MarketContext) AgentSignal {
	if client == nil || !client.IsAvailable() {
		return AgentSignal{Role: RoleBull, Error: "provider unavailable"}
	}
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

func runBearAgent(ctx context.Context, client SignalClient, market MarketContext) AgentSignal {
	if client == nil || !client.IsAvailable() {
		return AgentSignal{Role: RoleBear, Error: "provider unavailable"}
	}
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

func runMacroAgent(ctx context.Context, client MacroClient, market MarketContext) AgentSignal {
	if client == nil || !client.IsAvailable() {
		return AgentSignal{
			Role:        RoleMacro,
			ShouldTrade: false,
			Confidence:  0,
			Thesis:      "Macro Agent provider disabled / unavailable",
			Error:       "provider not configured",
		}
	}

	prompt := buildMarketPrompt(market) + "\n\nProvide your top-down macro assessment. Is the backdrop favorable for scalping right now?"
	raw, err := client.ChatForMacro(ctx, macroSystemPrompt, prompt)
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

func runRiskAgent(ctx context.Context, client RiskClient, bull, bear, macro AgentSignal, market MarketContext) RiskVerdict {
	if client == nil || !client.IsAvailable() {
		return RiskVerdict{Approved: false, ApprovedAction: "HOLD", Error: "provider unavailable"}
	}
	bullJSON, _ := json.MarshalIndent(bull, "", "  ")
	bearJSON, _ := json.MarshalIndent(bear, "", "  ")
	macroJSON, _ := json.MarshalIndent(macro, "", "  ")

	macroSection := ""
	if macro.Error == "" {
		macroSection = fmt.Sprintf("\nMACRO ANALYST ASSESSMENT:\n%s\n", string(macroJSON))
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
	openai      *OpenAIClient
	gemini      *GeminiClient
	groq        *GroqClient
	openrouter  *OpenRouterClient
	mistral     *MistralClient
	huggingface *HuggingFaceClient
	cloudflare  *CloudflareClient
	insights    *InsightStore
	store       *persistence.Store
	mu          sync.Mutex
	auditMu     sync.Mutex
	idSeq       int
}

// ─────────────────────────────────────────────────────────────────
// RESILIENT AGENT WRAPPERS (MULTI-AI FALLBACK)
// ─────────────────────────────────────────────────────────────────

func (o *MultiAgentOrchestrator) runBullAgentWithFallback(ctx context.Context, market MarketContext) AgentSignal {
	// Priority: Groq → Mistral → HuggingFace → OpenRouter → OpenAI
	sig := runBullAgent(ctx, o.groq, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bull (Groq) failed: %v. Trying Mistral...", sig.Error)
	sig = runBullAgent(ctx, o.mistral, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bull (Mistral) failed: %v. Trying HuggingFace...", sig.Error)
	sig = runBullAgent(ctx, o.huggingface, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bull (HuggingFace) failed: %v. Trying Cloudflare...", sig.Error)
	sig = runBullAgent(ctx, o.cloudflare, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bull (Cloudflare) failed: %v. Trying OpenRouter...", sig.Error)
	sig = runBullAgent(ctx, o.openrouter, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bull (OpenRouter) failed: %v. Trying OpenAI...", sig.Error)
	return runBullAgent(ctx, o.openai, market)
}

func (o *MultiAgentOrchestrator) runBearAgentWithFallback(ctx context.Context, market MarketContext) AgentSignal {
	// Priority: Groq → Mistral → HuggingFace → OpenRouter → OpenAI
	sig := runBearAgent(ctx, o.groq, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bear (Groq) failed: %v. Trying Mistral...", sig.Error)
	sig = runBearAgent(ctx, o.mistral, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bear (Mistral) failed: %v. Trying HuggingFace...", sig.Error)
	sig = runBearAgent(ctx, o.huggingface, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bear (HuggingFace) failed: %v. Trying Cloudflare...", sig.Error)
	sig = runBearAgent(ctx, o.cloudflare, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bear (Cloudflare) failed: %v. Trying OpenRouter...", sig.Error)
	sig = runBearAgent(ctx, o.openrouter, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Bear (OpenRouter) failed: %v. Trying OpenAI...", sig.Error)
	return runBearAgent(ctx, o.openai, market)
}

func (o *MultiAgentOrchestrator) runMacroAgentWithFallback(ctx context.Context, market MarketContext) AgentSignal {
	// Primary: Gemini → Groq → Mistral → HuggingFace → OpenRouter
	sig := runMacroAgent(ctx, o.gemini, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Macro (Gemini) failed: %v. Trying Groq...", sig.Error)
	sig = runMacroAgent(ctx, o.groq, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Macro (Groq) failed: %v. Trying Mistral...", sig.Error)
	sig = runMacroAgent(ctx, o.mistral, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Macro (Mistral) failed: %v. Trying HuggingFace...", sig.Error)
	sig = runMacroAgent(ctx, o.huggingface, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Macro (HuggingFace) failed: %v. Trying Cloudflare...", sig.Error)
	sig = runMacroAgent(ctx, o.cloudflare, market)
	if sig.Error == "" { return sig }

	log.Printf("[AI FALLBACK] Macro (Cloudflare) failed: %v. Trying OpenRouter...", sig.Error)
	return runMacroAgent(ctx, o.openrouter, market)
}

func (o *MultiAgentOrchestrator) runRiskAgentWithFallback(ctx context.Context, bull, bear, macro AgentSignal, market MarketContext) RiskVerdict {
	// Priority: Groq → Gemini → Mistral → HuggingFace → OpenRouter → OpenAI
	v := runRiskAgent(ctx, o.groq, bull, bear, macro, market)
	if v.Error == "" { return v }

	log.Printf("[AI FALLBACK] Risk (Groq) failed: %v. Trying Gemini...", v.Error)
	v = runRiskAgent(ctx, o.gemini, bull, bear, macro, market)
	if v.Error == "" { return v }

	log.Printf("[AI FALLBACK] Risk (Gemini) failed: %v. Trying Mistral...", v.Error)
	v = runRiskAgent(ctx, o.mistral, bull, bear, macro, market)
	if v.Error == "" { return v }

	log.Printf("[AI FALLBACK] Risk (Mistral) failed: %v. Trying HuggingFace...", v.Error)
	v = runRiskAgent(ctx, o.huggingface, bull, bear, macro, market)
	if v.Error == "" { return v }

	log.Printf("[AI FALLBACK] Risk (HuggingFace) failed: %v. Trying Cloudflare...", v.Error)
	v = runRiskAgent(ctx, o.cloudflare, bull, bear, macro, market)
	if v.Error == "" { return v }

	log.Printf("[AI FALLBACK] Risk (Cloudflare) failed: %v. Trying OpenRouter...", v.Error)
	v = runRiskAgent(ctx, o.openrouter, bull, bear, macro, market)
	if v.Error == "" { return v }

	log.Printf("[AI FALLBACK] Risk (OpenRouter) failed: %v. Trying OpenAI...", v.Error)
	return runRiskAgent(ctx, o.openai, bull, bear, macro, market)
}

func NewMultiAgentOrchestrator(openai *OpenAIClient, gemini *GeminiClient, groq *GroqClient, openrouter *OpenRouterClient, mistral *MistralClient, huggingface *HuggingFaceClient, cloudflare *CloudflareClient, store *persistence.Store) *MultiAgentOrchestrator {
	return &MultiAgentOrchestrator{
		openai:      openai,
		gemini:      gemini,
		groq:        groq,
		openrouter:  openrouter,
		mistral:     mistral,
		huggingface: huggingface,
		cloudflare:  cloudflare,
		store:       store,
		insights:    NewInsightStore(50),
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

// AddHistoricalAudit populates the in-memory store from database records on startup.
func (o *MultiAgentOrchestrator) AddHistoricalAudit(data map[string]interface{}) {
	approved, _ := data["approved"].(bool)
	conf, _ := data["confidence"].(float64)
	ts, _ := data["timestamp"].(time.Time)

	logEntry := AuditLog{
		ID:           fmt.Sprintf("%v", data["id"]),
		StrategyName: fmt.Sprintf("%v", data["strategyName"]),
		Action:       fmt.Sprintf("%v", data["action"]),
		Approved:     approved,
		Reason:       fmt.Sprintf("%v", data["reason"]),
		Confidence:   conf,
		Timestamp:    ts,
		Provider:     fmt.Sprintf("%v", data["provider"]),
	}

	o.insights.AddAudit(logEntry)
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
		bullSig = o.runBullAgentWithFallback(agentCtx, market)
	}()
	go func() {
		defer wg.Done()
		bearSig = o.runBearAgentWithFallback(agentCtx, market)
	}()
	go func() {
		defer wg.Done()
		macroSig = o.runMacroAgentWithFallback(agentCtx, market)
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
		reasoning := "Council (OpenAI+Gemini) recommended HOLD."
		// Append errors if any agent failed, to help debug "Why it's zero"
		var errors []string
		if bullSig.Error != "" { errors = append(errors, "OpenAI Bull: "+bullSig.Error) }
		if bearSig.Error != "" { errors = append(errors, "OpenAI Bear: "+bearSig.Error) }
		if macroSig.Error != "" { errors = append(errors, "Gemini Macro: "+macroSig.Error) }
		
		if len(errors) > 0 {
			reasoning = "⚠️ AI ERRORS:\n" + strings.Join(errors, "\n")
		}

		decision := o.buildDecision(market, bullSig, bearSig, macroSig, RiskVerdict{
			Approved:       false,
			ApprovedAction: "HOLD",
			Reasoning:      reasoning,
		})
		o.insights.Add(decision)
		return decision
	}

	riskVerdict := o.runRiskAgentWithFallback(agentCtx, bullSig, bearSig, macroSig, market)
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

	bullLine := fmt.Sprintf("BULL [conf:%.0f%%]: %s", bull.Confidence*100, bull.Thesis)
	if bull.Error != "" {
		bullLine = fmt.Sprintf("BULL: ⚠️ ERROR [%s]", bull.Error)
	}

	bearLine := fmt.Sprintf("BEAR [conf:%.0f%%]: %s", bear.Confidence*100, bear.Thesis)
	if bear.Error != "" {
		bearLine = fmt.Sprintf("BEAR: ⚠️ ERROR [%s]", bear.Error)
	}

	reasoning := fmt.Sprintf(
		"%s\n\n%s%s\n\nRISK: %s",
		bullLine,
		bearLine,
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
