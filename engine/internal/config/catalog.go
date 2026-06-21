package config

// Catalog is the single source of truth for every tunable trading threshold
// surfaced by the Trade Threshold Configuration module. Each entry's EnvVar
// is read at startup (InitFromEnv) using the same default-on-missing-or-
// invalid pattern as getInitialPaperBalanceUSD in cmd/antigravity/main.go.
//
// To add a new tunable threshold: add one Meta entry here, add its literal
// default to metaDefaults, and read it at the call site via
// config.Default().Get("KEY") (or GetWithDefault as a defensive fallback).

const (
	CategorySignalQuality       = "signal_quality"
	CategoryPositionSizing      = "position_sizing"
	CategoryRiskManagement      = "risk_management"
	CategoryDirectionalCaps     = "directional_caps"
	CategoryWalkForward         = "walk_forward"
	CategoryCooldowns           = "cooldowns"
	CategoryRegimeClassification = "regime_classification"
	CategoryExecution           = "execution"
)

// metaDefaults carries the literal shipped default for each key — the value
// used when the env var is unset or invalid. Kept separate from Meta so the
// catalog table below stays readable.
var metaDefaults = map[string]float64{
	// signal_quality
	"MIN_EXECUTABLE_CONFIDENCE":      0.68,
	"MIN_BRIDGE_CONFIDENCE":          0.65,
	"MIN_REWARD_TO_RISK_RATIO":       2.40,
	"MIN_SIGNAL_TAKE_PROFIT_PCT":     0.15,
	"MAX_SIGNAL_STOP_LOSS_PCT":       1.50,
	"SCALER_RR_MINIMUM":              2.00,
	"SCALER_SL_LOWER_BOUND_PCT":      0.25,

	// position_sizing
	"MIN_EXECUTION_SIZE_BTC":         0.01,
	"FUTURES_POSITION_CAPITAL_PCT":   0.01,
	"KELLY_MAX_POSITION_PCT":         0.10,
	"KELLY_MIN_TRADES_REQUIRED":      30,
	"AI_SIGNAL_MAX_SIZE_BTC":         0.50,

	// risk_management
	"RISK_INTRADAY_DRAWDOWN_PCT":         0.02,
	"RISK_CORRELATION_EXPOSURE_THRESHOLD": 0.60,
	"RISK_CORRELATION_MIN_CONFIDENCE":     0.80,
	"PMS_MAX_HEAT_PCT":               8,
	"PMS_MAX_VAR95_PCT":              6,
	"PMS_MAX_CVAR95_PCT":             9,
	"PMS_MAX_DRAWDOWN_PCT":           10,
	"PMS_MAX_DAILY_LOSS_PCT":         3,
	"PMS_MAX_GROSS_EXPOSURE_PCT":     250,
	"PMS_MAX_NET_EXPOSURE_PCT":       150,

	// directional_caps
	"MAX_CORRELATED_STRATEGIES": 2,
	"MAX_CORRELATED_BTC":        0.30,
	"MAX_OPEN_LONG_TRADES":      3,
	"MAX_OPEN_SHORT_TRADES":     3,

	// walk_forward
	"WALKFORWARD_MIN_TRADES":          30,
	"WALKFORWARD_WIN_RATE_THRESHOLD":  0.48,
	"WALKFORWARD_SHARPE_THRESHOLD":    0.60,
	"WALKFORWARD_MAX_DRAWDOWN":        0.20,
	"WALKFORWARD_CONSECUTIVE_WINDOWS": 2,

	// cooldowns — scaler strategy cooldown buckets, in seconds.
	"COOLDOWN_SNIPER_SWEEP_REVERSION_SEC": 30,
	"COOLDOWN_TREND_FADE_VWAP_SEC":        180,
	"COOLDOWN_BREAKOUT_FUNDING_OI_SEC":    600,
	"COOLDOWN_ADX_MOMENTUM_SEC":           1800,
	"COOLDOWN_DEFAULT_SEC":                30,

	// regime_classification
	"REGIME_ATR_HIGH_VOL_RATIO":   2.0,
	"REGIME_ATR_EXTREME_VOL_RATIO": 3.0,
	"REGIME_ADX_TREND_THRESHOLD":  25,
	"REGIME_ADX_RANGE_THRESHOLD":  20,

	// execution
	"MIN_EXECUTION_WEIGHT_TO_TRADE": 0.50,
}

// Catalog returns the full static metadata for every tunable threshold.
func Catalog() []Meta {
	return []Meta{
		// ── signal_quality ──────────────────────────────────────────────────
		{
			Key: "MIN_EXECUTABLE_CONFIDENCE", Category: CategorySignalQuality,
			Label: "Minimum Executable Confidence", EnvVar: "MIN_EXECUTABLE_CONFIDENCE",
			Description: "Floor confidence score (0-1) a signal must clear before the engine will execute it. Used across loop.go and scalers_eval.go as the baseline gate ahead of the adaptive confidence floor.",
			DataType: "decimal", Unit: "", Min: 0.50, Max: 0.95, RecommendedMin: 0.62, RecommendedMax: 0.75,
			RiskLevel: "high",
			WhatIfIncreased: "Fewer signals pass the gate — trade frequency drops roughly 30-50% per +0.05, but average signal quality and win rate typically improve. Raising above 0.80 risks the engine going dormant for hours in calm markets.",
			WhatIfDecreased: "More marginal signals execute, increasing trade frequency but degrading average edge per trade. Below 0.60 the engine has historically taken low-conviction trades that widen drawdown.",
		},
		{
			Key: "MIN_BRIDGE_CONFIDENCE", Category: CategorySignalQuality,
			Label: "Minimum Bridge Approval Confidence", EnvVar: "MIN_BRIDGE_CONFIDENCE",
			Description: "Confidence floor required for AI-bridge-submitted signals to be approved before they reach the execution path.",
			DataType: "decimal", Unit: "", Min: 0.50, Max: 0.95, RecommendedMin: 0.60, RecommendedMax: 0.72,
			RiskLevel: "high",
			WhatIfIncreased: "AI bridge signals are rejected more often; reduces exposure to noisy AI-generated setups but can starve the bridge path of any approved trades if set above 0.80.",
			WhatIfDecreased: "More AI-bridge signals are approved, increasing trade count from that path but also increasing exposure to lower-conviction AI calls.",
		},
		{
			Key: "MIN_REWARD_TO_RISK_RATIO", Category: CategorySignalQuality,
			Label: "Minimum Reward-to-Risk Ratio", EnvVar: "MIN_REWARD_TO_RISK_RATIO",
			Description: "Minimum reward:risk a signal must offer in loop.go's constant table. Note this competes with the separate SCALER_RR_MINIMUM (2.0) hardcoded in scalers_eval.go's sanitizeScalerSignal — the two have historically drifted out of sync.",
			DataType: "decimal", Unit: ":1", Min: 1.0, Max: 5.0, RecommendedMin: 2.0, RecommendedMax: 3.0,
			RiskLevel: "critical",
			WhatIfIncreased: "Only highly asymmetric setups qualify — trade frequency falls sharply above 3.0:1 since most intraday BTC setups offer 1.5-2.5:1. Past incident: an R:R math mismatch between this value and the scaler-side 2.0 floor produced a dead-strategy event where strategies passed loop.go's gate but were rejected downstream, silently halting trading.",
			WhatIfDecreased: "Lower-quality asymmetric setups qualify, increasing trade frequency but eroding the statistical edge Kelly sizing assumes (AvgWinPct/AvgLossPct ratio degrades). Below 1.5:1 the system's win-rate assumptions used for sizing become invalid.",
		},
		{
			Key: "MIN_SIGNAL_TAKE_PROFIT_PCT", Category: CategorySignalQuality,
			Label: "Minimum Signal Take-Profit %", EnvVar: "MIN_SIGNAL_TAKE_PROFIT_PCT",
			Description: "Lower bound enforced on every signal's take-profit percentage during sanitization (loop.go and scalers_eval.go share this floor).",
			DataType: "percentage", Unit: "%", Min: 0.05, Max: 1.00, RecommendedMin: 0.10, RecommendedMax: 0.25,
			RiskLevel: "medium",
			WhatIfIncreased: "Signals are forced to target larger moves before exiting profitably; fewer trades hit target in choppy BTC ranges, increasing average hold time.",
			WhatIfDecreased: "Trades take profit on smaller moves, increasing win rate but reducing average win size — can erode the reward side of the R:R ratio enforced above.",
		},
		{
			Key: "MAX_SIGNAL_STOP_LOSS_PCT", Category: CategorySignalQuality,
			Label: "Maximum Signal Stop-Loss %", EnvVar: "MAX_SIGNAL_STOP_LOSS_PCT",
			Description: "Upper bound enforced on every signal's stop-loss percentage. Shared between loop.go's stopLossFromSignal fallback and scalers_eval.go's sanitizeScalerSignal.",
			DataType: "percentage", Unit: "%", Min: 0.25, Max: 3.00, RecommendedMin: 1.00, RecommendedMax: 2.00,
			RiskLevel: "high",
			WhatIfIncreased: "Stops are allowed further from entry, reducing premature stop-outs from BTC noise but increasing per-trade max loss and Kelly sizing's effective risk-per-trade.",
			WhatIfDecreased: "Tighter stops reduce per-trade loss but increase the rate of being stopped out by normal volatility, which can directly degrade the win rate the walk-forward validator measures.",
		},
		{
			Key: "SCALER_RR_MINIMUM", Category: CategorySignalQuality,
			Label: "Scaler Strategy R:R Floor", EnvVar: "SCALER_RR_MINIMUM",
			Description: "Hardcoded 2.0 R:R floor inside scalers_eval.go's sanitizeScalerSignal. This is the value actually enforced for scaler-originated signals and can diverge from MIN_REWARD_TO_RISK_RATIO above.",
			DataType: "decimal", Unit: ":1", Min: 1.0, Max: 5.0, RecommendedMin: 1.8, RecommendedMax: 2.5,
			RiskLevel: "critical",
			WhatIfIncreased: "Scaler strategies reject more setups, reducing trade count from the scalper roster specifically (separate from the AI/loop.go path).",
			WhatIfDecreased: "Scaler strategies accept thinner setups. Keep this aligned with MIN_REWARD_TO_RISK_RATIO — historically a mismatch here silently demoted strategies that passed one gate but failed the other.",
		},
		{
			Key: "SCALER_SL_LOWER_BOUND_PCT", Category: CategorySignalQuality,
			Label: "Scaler Stop-Loss Lower Bound %", EnvVar: "SCALER_SL_LOWER_BOUND_PCT",
			Description: "Hardcoded 0.25% inline floor in sanitizeScalerSignal preventing stops from being set unrealistically tight on scaler signals.",
			DataType: "percentage", Unit: "%", Min: 0.05, Max: 1.00, RecommendedMin: 0.15, RecommendedMax: 0.40,
			RiskLevel: "medium",
			WhatIfIncreased: "Scaler trades require wider minimum stops, reducing whipsaw stop-outs on fast scalper timeframes but raising per-trade risk floor.",
			WhatIfDecreased: "Allows tighter stops on fast scalper entries; below 0.10% stops can sit inside normal bid/ask spread noise and trigger on fill slippage alone.",
		},

		// ── position_sizing ─────────────────────────────────────────────────
		{
			Key: "MIN_EXECUTION_SIZE_BTC", Category: CategoryPositionSizing,
			Label: "Minimum Execution Size", EnvVar: "MIN_EXECUTION_SIZE_BTC",
			Description: "Smallest BTC size the engine will submit as an order; sizes below this are treated as a no-trade.",
			DataType: "decimal", Unit: "BTC", Min: 0.001, Max: 0.05, RecommendedMin: 0.005, RecommendedMax: 0.02,
			RiskLevel: "low",
			WhatIfIncreased: "Smaller, marginal-conviction signals are skipped entirely instead of executing at a tiny size — reduces fee drag from micro-trades.",
			WhatIfDecreased: "Allows smaller positions through, increasing trade count but raising the share of PnL eaten by fees/slippage on each tiny fill.",
		},
		{
			Key: "FUTURES_POSITION_CAPITAL_PCT", Category: CategoryPositionSizing,
			Label: "Futures Position Capital %", EnvVar: "FUTURES_POSITION_CAPITAL_PCT",
			Description: "Legacy fixed-capital sizing: percent of paper capital allocated per futures entry before Kelly sizing took over the scalers path. Still used as a fallback for non-Kelly call sites.",
			DataType: "percentage", Unit: "%", Min: 0.002, Max: 0.05, RecommendedMin: 0.005, RecommendedMax: 0.02,
			RiskLevel: "medium",
			WhatIfIncreased: "Each non-Kelly entry risks a larger slice of capital, amplifying both gains and drawdowns on that path.",
			WhatIfDecreased: "Smaller fixed-capital entries reduce per-trade impact but also reduce compounding speed on strategies still using this legacy path.",
		},
		{
			Key: "KELLY_MAX_POSITION_PCT", Category: CategoryPositionSizing,
			Label: "Kelly Max Position %", EnvVar: "KELLY_MAX_POSITION_PCT",
			Description: "MaxPositionPct ceiling fed into kelly.Compute(). The package also enforces an absolute hardMaxPositionPct=0.10 (10%) regardless of this setting — this value can only tighten, never loosen, past that hard ceiling.",
			DataType: "percentage", Unit: "%", Min: 0.01, Max: 0.10, RecommendedMin: 0.03, RecommendedMax: 0.08,
			RiskLevel: "critical",
			WhatIfIncreased: "Kelly-sized positions can grow larger (up to the hard 10% ceiling), increasing both expected return and variance of the paper account.",
			WhatIfDecreased: "Caps Kelly sizing more conservatively; reduces variance but also reduces compounding speed when win rate/edge is genuinely favorable.",
			RequiresRestart: true,
		},
		{
			Key: "KELLY_MIN_TRADES_REQUIRED", Category: CategoryPositionSizing,
			Label: "Kelly Minimum Trade History", EnvVar: "KELLY_MIN_TRADES_REQUIRED",
			Description: "Number of closed trades a strategy needs before Kelly sizing replaces the conservative 5% probationary fallback (WinRate=0.50/AvgWinPct=0.04/AvgLossPct=0.02 neutral assumption).",
			DataType: "integer", Unit: "trades", Min: 10, Max: 100, RecommendedMin: 20, RecommendedMax: 50,
			RiskLevel: "medium",
			WhatIfIncreased: "Strategies spend longer in the conservative probationary sizing regime before Kelly's data-driven sizing kicks in — safer but slower to capitalize on early outperformance.",
			WhatIfDecreased: "Kelly sizing activates on thinner trade samples, which can produce overconfident position sizes from a statistically unreliable win rate.",
			RequiresRestart: true,
		},
		{
			Key: "AI_SIGNAL_MAX_SIZE_BTC", Category: CategoryPositionSizing,
			Label: "AI Signal Max Size", EnvVar: "AI_SIGNAL_MAX_SIZE_BTC",
			Description: "Hard cap on BTC size for any AI-bridge-originated signal in runAIDecision(), independent of Kelly or confidence-based sizing.",
			DataType: "decimal", Unit: "BTC", Min: 0.05, Max: 2.00, RecommendedMin: 0.25, RecommendedMax: 0.75,
			RiskLevel: "high",
			WhatIfIncreased: "AI-originated trades can size up further, increasing both upside and the blast radius of a bad AI call.",
			WhatIfDecreased: "Limits AI signal exposure regardless of confidence — protective against AI bridge misbehavior but caps participation in genuinely strong AI setups.",
		},

		// ── risk_management ─────────────────────────────────────────────────
		{
			Key: "RISK_INTRADAY_DRAWDOWN_PCT", Category: CategoryRiskManagement,
			Label: "Intraday Drawdown Pause Threshold", EnvVar: "RISK_INTRADAY_DRAWDOWN_PCT",
			Description: "RISK 1 ratchet: trading pauses for the rest of the UTC day once equity falls this % below the day's peak equity.",
			DataType: "percentage", Unit: "%", Min: 0.01, Max: 0.10, RecommendedMin: 0.015, RecommendedMax: 0.04,
			RiskLevel: "critical",
			WhatIfIncreased: "The circuit breaker tolerates deeper intraday losses before halting — more room to recover from a bad stretch, but materially larger worst-case daily loss.",
			WhatIfDecreased: "Trading pauses sooner on a losing day, capping downside tightly but increasing the chance a normal volatile session triggers an unnecessary full-day pause.",
			RequiresRestart: true,
		},
		{
			Key: "RISK_CORRELATION_EXPOSURE_THRESHOLD", Category: CategoryRiskManagement,
			Label: "Correlation Exposure Threshold", EnvVar: "RISK_CORRELATION_EXPOSURE_THRESHOLD",
			Description: "BUG 12 guard: once net exposure exceeds this fraction of MaxPositionBTC, additional same-direction exposure requires confidence above RISK_CORRELATION_MIN_CONFIDENCE.",
			DataType: "percentage", Unit: "%", Min: 0.30, Max: 0.90, RecommendedMin: 0.50, RecommendedMax: 0.70,
			RiskLevel: "high",
			WhatIfIncreased: "The stronger-conviction requirement kicks in later (closer to max exposure), allowing more concentrated directional bets before the extra confidence bar applies.",
			WhatIfDecreased: "The extra confidence requirement applies earlier, throttling concentrated directional exposure sooner.",
			RequiresRestart: true,
		},
		{
			Key: "RISK_CORRELATION_MIN_CONFIDENCE", Category: CategoryRiskManagement,
			Label: "Correlation Min Confidence", EnvVar: "RISK_CORRELATION_MIN_CONFIDENCE",
			Description: "Confidence floor required to add same-direction exposure once RISK_CORRELATION_EXPOSURE_THRESHOLD is breached.",
			DataType: "decimal", Unit: "", Min: 0.50, Max: 0.95, RecommendedMin: 0.70, RecommendedMax: 0.85,
			RiskLevel: "high",
			WhatIfIncreased: "Harder to add to an already-concentrated position — reduces risk of over-leveraging into one direction during a strong trend.",
			WhatIfDecreased: "Easier to add to concentrated exposure on moderate conviction, increasing correlation risk if the trend reverses.",
			RequiresRestart: true,
		},
		{
			Key: "PMS_MAX_HEAT_PCT", Category: CategoryRiskManagement,
			Label: "Portfolio Max Heat %", EnvVar: "PMS_MAX_HEAT_PCT",
			Description: "Portfolio risk budget MaxHeatPct used by the institutional execution path's RiskBudget. Currently a hardcoded literal in loop.go's executeThroughInstitutionalPathWithFill.",
			DataType: "percentage", Unit: "%", Min: 2, Max: 20, RecommendedMin: 5, RecommendedMax: 12,
			RiskLevel: "critical",
			WhatIfIncreased: "Portfolio is allowed more aggregate at-risk exposure across open positions before the PMS gate blocks new trades.",
			WhatIfDecreased: "PMS blocks new trades sooner as aggregate heat accumulates, capping total simultaneous risk more conservatively.",
			RequiresRestart: true,
		},
		{
			Key: "PMS_MAX_VAR95_PCT", Category: CategoryRiskManagement,
			Label: "Portfolio Max VaR95 %", EnvVar: "PMS_MAX_VAR95_PCT",
			Description: "95% Value-at-Risk ceiling in the PMS RiskBudget.",
			DataType: "percentage", Unit: "%", Min: 2, Max: 15, RecommendedMin: 4, RecommendedMax: 8,
			RiskLevel: "critical",
			WhatIfIncreased: "Tolerates a statistically larger 1-day loss estimate before blocking new risk.",
			WhatIfDecreased: "Forces the portfolio to stay within a tighter modeled 1-day loss envelope.",
			RequiresRestart: true,
		},
		{
			Key: "PMS_MAX_CVAR95_PCT", Category: CategoryRiskManagement,
			Label: "Portfolio Max CVaR95 %", EnvVar: "PMS_MAX_CVAR95_PCT",
			Description: "95% Conditional VaR (tail loss) ceiling in the PMS RiskBudget.",
			DataType: "percentage", Unit: "%", Min: 3, Max: 18, RecommendedMin: 6, RecommendedMax: 12,
			RiskLevel: "critical",
			WhatIfIncreased: "Permits a fatter modeled tail-loss before the gate blocks further risk-taking.",
			WhatIfDecreased: "Tightens tolerance for tail-risk scenarios, blocking new trades sooner during stressed conditions.",
			RequiresRestart: true,
		},
		{
			Key: "PMS_MAX_DRAWDOWN_PCT", Category: CategoryRiskManagement,
			Label: "Portfolio Max Drawdown %", EnvVar: "PMS_MAX_DRAWDOWN_PCT",
			Description: "Portfolio-level max drawdown ceiling in the PMS RiskBudget (distinct from the per-day intraday ratchet above).",
			DataType: "percentage", Unit: "%", Min: 5, Max: 25, RecommendedMin: 8, RecommendedMax: 15,
			RiskLevel: "critical",
			WhatIfIncreased: "The portfolio can draw down further from its high-water mark before the PMS gate halts new risk.",
			WhatIfDecreased: "New trades get blocked earlier in a drawdown, protecting capital but possibly missing the recovery leg.",
			RequiresRestart: true,
		},
		{
			Key: "PMS_MAX_DAILY_LOSS_PCT", Category: CategoryRiskManagement,
			Label: "Portfolio Max Daily Loss %", EnvVar: "PMS_MAX_DAILY_LOSS_PCT",
			Description: "Daily loss ceiling in the PMS RiskBudget, layered on top of the legacy RiskEngine circuit breaker (profile.MaxDailyLossPct).",
			DataType: "percentage", Unit: "%", Min: 1, Max: 10, RecommendedMin: 2, RecommendedMax: 5,
			RiskLevel: "critical",
			WhatIfIncreased: "Allows a larger same-day loss before the PMS-level breaker fires (separate from RiskEngine's own daily loss check).",
			WhatIfDecreased: "PMS halts new risk sooner on a losing day.",
			RequiresRestart: true,
		},
		{
			Key: "PMS_MAX_GROSS_EXPOSURE_PCT", Category: CategoryRiskManagement,
			Label: "Portfolio Max Gross Exposure %", EnvVar: "PMS_MAX_GROSS_EXPOSURE_PCT",
			Description: "Maximum gross (long+short absolute) exposure as % of equity in the PMS RiskBudget.",
			DataType: "percentage", Unit: "%", Min: 100, Max: 400, RecommendedMin: 150, RecommendedMax: 300,
			RiskLevel: "high",
			WhatIfIncreased: "Allows more total notional exposure relative to equity (effectively more leverage) before the gate blocks new orders.",
			WhatIfDecreased: "Caps total notional exposure tighter relative to equity, reducing effective leverage headroom.",
			RequiresRestart: true,
		},
		{
			Key: "PMS_MAX_NET_EXPOSURE_PCT", Category: CategoryRiskManagement,
			Label: "Portfolio Max Net Exposure %", EnvVar: "PMS_MAX_NET_EXPOSURE_PCT",
			Description: "Maximum net (long-short) directional exposure as % of equity in the PMS RiskBudget.",
			DataType: "percentage", Unit: "%", Min: 50, Max: 250, RecommendedMin: 100, RecommendedMax: 180,
			RiskLevel: "high",
			WhatIfIncreased: "Allows a larger one-directional net bet relative to equity.",
			WhatIfDecreased: "Forces more balanced/hedged exposure, reducing single-direction concentration risk.",
			RequiresRestart: true,
		},

		// ── directional_caps ────────────────────────────────────────────────
		{
			Key: "MAX_CORRELATED_STRATEGIES", Category: CategoryDirectionalCaps,
			Label: "Max Same-Direction Strategies", EnvVar: "MAX_CORRELATED_STRATEGIES",
			Description: "ConcentrationGate.maxSameDirection: max number of strategies concurrently open in the same direction (LONG or SHORT), a size-aware check beyond the plain trade-count cap.",
			DataType: "integer", Unit: "strategies", Min: 1, Max: 8, RecommendedMin: 2, RecommendedMax: 4,
			RiskLevel: "high",
			WhatIfIncreased: "More strategies can pile into the same directional bet simultaneously — increases correlated-bet concentration risk even though each looks diversified by name.",
			WhatIfDecreased: "Limits how many strategies can simultaneously express the same directional view, reducing hidden correlation risk but possibly blocking genuinely independent setups that happen to agree on direction.",
			RequiresRestart: true,
		},
		{
			Key: "MAX_CORRELATED_BTC", Category: CategoryDirectionalCaps,
			Label: "Max Correlated BTC Exposure", EnvVar: "MAX_CORRELATED_BTC",
			Description: "ConcentrationGate.maxCorrelatedBTC: max combined BTC size across all same-direction open positions, regardless of strategy count.",
			DataType: "decimal", Unit: "BTC", Min: 0.05, Max: 2.00, RecommendedMin: 0.15, RecommendedMax: 0.60,
			RiskLevel: "high",
			WhatIfIncreased: "Allows a larger combined same-direction BTC notional across all strategies before the gate blocks further opens.",
			WhatIfDecreased: "Tightens the total same-direction BTC notional ceiling, capping the size of any single correlated directional bet across the whole strategy roster.",
			RequiresRestart: true,
		},
		{
			Key: "MAX_OPEN_LONG_TRADES", Category: CategoryDirectionalCaps,
			Label: "Max Open Long Trades", EnvVar: "MAX_OPEN_LONG_TRADES",
			Description: "Orchestrator-level directional cap (BUG 4) on concurrently open long trades. Duplicated separately as ScalerBundle.maxOpenLongs (hardcoded =3 in newScalerBundle) — the two are not currently unified through this registry.",
			DataType: "integer", Unit: "trades", Min: 1, Max: 10, RecommendedMin: 2, RecommendedMax: 5,
			RiskLevel: "high",
			WhatIfIncreased: "More simultaneous long positions can be open, increasing aggregate long exposure and the number of positions to manage during a reversal.",
			WhatIfDecreased: "Caps the engine's ability to add new long trades sooner, reducing maximum simultaneous long exposure.",
			RequiresRestart: true,
		},
		{
			Key: "MAX_OPEN_SHORT_TRADES", Category: CategoryDirectionalCaps,
			Label: "Max Open Short Trades", EnvVar: "MAX_OPEN_SHORT_TRADES",
			Description: "Orchestrator-level directional cap (BUG 4) on concurrently open short trades. Same ScalerBundle duplication caveat as MAX_OPEN_LONG_TRADES.",
			DataType: "integer", Unit: "trades", Min: 1, Max: 10, RecommendedMin: 2, RecommendedMax: 5,
			RiskLevel: "high",
			WhatIfIncreased: "More simultaneous short positions can be open, increasing aggregate short exposure.",
			WhatIfDecreased: "Caps the engine's ability to add new short trades sooner.",
			RequiresRestart: true,
		},

		// ── walk_forward ────────────────────────────────────────────────────
		{
			Key: "WALKFORWARD_MIN_TRADES", Category: CategoryWalkForward,
			Label: "Walk-Forward Window Size", EnvVar: "WALKFORWARD_MIN_TRADES",
			Description: "Number of closed trades per non-overlapping evaluation window. Status (PROBATIONARY/ACTIVE/DEMOTED) is only re-evaluated at window boundaries, not on every trade.",
			DataType: "integer", Unit: "trades", Min: 10, Max: 100, RecommendedMin: 20, RecommendedMax: 50,
			RiskLevel: "medium",
			WhatIfIncreased: "Promotion/demotion decisions use a larger, more statistically reliable sample, but strategies take longer to reach ACTIVE status or get caught underperforming.",
			WhatIfDecreased: "Faster promotion/demotion cycles but on noisier, smaller samples — increases the chance a lucky or unlucky streak misclassifies a strategy.",
			RequiresRestart: true,
		},
		{
			Key: "WALKFORWARD_WIN_RATE_THRESHOLD", Category: CategoryWalkForward,
			Label: "Walk-Forward Win Rate Threshold", EnvVar: "WALKFORWARD_WIN_RATE_THRESHOLD",
			Description: "Minimum win rate a window must clear (alongside Sharpe) to count toward promotion to ACTIVE.",
			DataType: "decimal", Unit: "", Min: 0.30, Max: 0.70, RecommendedMin: 0.42, RecommendedMax: 0.55,
			RiskLevel: "high",
			WhatIfIncreased: "Harder for strategies to earn a passing window — fewer promotions, more demotions, smaller live-trading roster.",
			WhatIfDecreased: "Easier to pass — more strategies reach ACTIVE faster, including ones whose edge depends more on R:R than win rate. Historical win-rate collapses have occurred when this was set too permissively alongside a low R:R floor.",
		},
		{
			Key: "WALKFORWARD_SHARPE_THRESHOLD", Category: CategoryWalkForward,
			Label: "Walk-Forward Sharpe Threshold", EnvVar: "WALKFORWARD_SHARPE_THRESHOLD",
			Description: "Minimum annualized Sharpe ratio a window must clear (alongside win rate) for promotion.",
			DataType: "decimal", Unit: "", Min: 0.20, Max: 2.00, RecommendedMin: 0.40, RecommendedMax: 1.00,
			RiskLevel: "high",
			WhatIfIncreased: "Stricter risk-adjusted-return bar — fewer strategies promoted, but those that are tend to have smoother equity curves.",
			WhatIfDecreased: "Looser bar admits strategies with choppier, higher-variance return profiles into ACTIVE rotation.",
		},
		{
			Key: "WALKFORWARD_MAX_DRAWDOWN", Category: CategoryWalkForward,
			Label: "Walk-Forward Max Drawdown (reserved)", EnvVar: "WALKFORWARD_MAX_DRAWDOWN",
			Description: "Reserved threshold — the validator does not yet track drawdown directly, but the env var and field exist for a future wiring. Edits here currently have no runtime effect.",
			DataType: "percentage", Unit: "%", Min: 0.05, Max: 0.50, RecommendedMin: 0.10, RecommendedMax: 0.25,
			RiskLevel: "low",
			WhatIfIncreased: "No runtime effect today — reserved for a future drawdown-based demotion rule.",
			WhatIfDecreased: "No runtime effect today — reserved for a future drawdown-based demotion rule.",
			RequiresRestart: true,
		},
		{
			Key: "WALKFORWARD_CONSECUTIVE_WINDOWS", Category: CategoryWalkForward,
			Label: "Consecutive Passing Windows for Promotion", EnvVar: "WALKFORWARD_CONSECUTIVE_WINDOWS",
			Description: "Number of consecutive passing windows required before a strategy is promoted to ACTIVE — guards against promoting on a single lucky window.",
			DataType: "integer", Unit: "windows", Min: 1, Max: 5, RecommendedMin: 2, RecommendedMax: 3,
			RiskLevel: "medium",
			WhatIfIncreased: "Strategies need a longer sustained streak before promotion — slower rotation but higher confidence in promoted strategies.",
			WhatIfDecreased: "Strategies promote faster off shorter streaks, increasing the chance of promoting on variance rather than genuine edge.",
			RequiresRestart: true,
		},

		// ── cooldowns ───────────────────────────────────────────────────────
		{
			Key: "COOLDOWN_SNIPER_SWEEP_REVERSION_SEC", Category: CategoryCooldowns,
			Label: "Sniper/Sweep/Reversion Cooldown", EnvVar: "COOLDOWN_SNIPER_SWEEP_REVERSION_SEC",
			Description: "Per-strategy re-entry cooldown (scalerStrategyCooldown name-pattern match) for fast scalper families: Sniper, Sweep, Reversion.",
			DataType: "duration", Unit: "sec", Min: 5, Max: 300, RecommendedMin: 15, RecommendedMax: 60,
			RiskLevel: "low",
			WhatIfIncreased: "These fast scalpers re-enter less often, reducing overtrading risk but also reducing capture of rapid repeated setups.",
			WhatIfDecreased: "Allows faster re-entry after a close, increasing trade frequency and fee drag for this strategy family.",
			RequiresRestart: true,
		},
		{
			Key: "COOLDOWN_TREND_FADE_VWAP_SEC", Category: CategoryCooldowns,
			Label: "Trend/Fade/VWAP Cooldown", EnvVar: "COOLDOWN_TREND_FADE_VWAP_SEC",
			Description: "Per-strategy re-entry cooldown for Trend, Fade, VWAP strategy families.",
			DataType: "duration", Unit: "sec", Min: 30, Max: 1800, RecommendedMin: 120, RecommendedMax: 300,
			RiskLevel: "low",
			WhatIfIncreased: "Trend/fade/VWAP strategies wait longer before re-entering, avoiding rapid flip-flopping on the same signal.",
			WhatIfDecreased: "Faster re-entry risk chasing the same trend signal repeatedly in a short window.",
			RequiresRestart: true,
		},
		{
			Key: "COOLDOWN_BREAKOUT_FUNDING_OI_SEC", Category: CategoryCooldowns,
			Label: "Breakout/Funding/OI Cooldown", EnvVar: "COOLDOWN_BREAKOUT_FUNDING_OI_SEC",
			Description: "Per-strategy re-entry cooldown for Breakout, Funding, and Open-Interest-divergence strategy families.",
			DataType: "duration", Unit: "sec", Min: 60, Max: 3600, RecommendedMin: 300, RecommendedMax: 900,
			RiskLevel: "low",
			WhatIfIncreased: "Breakout/funding strategies wait longer to re-fire, reducing whipsaw re-entries on a false breakout retest.",
			WhatIfDecreased: "Faster re-entry can catch a genuine continuation move sooner but increases risk of repeated false-breakout entries.",
			RequiresRestart: true,
		},
		{
			Key: "COOLDOWN_ADX_MOMENTUM_SEC", Category: CategoryCooldowns,
			Label: "ADX/Momentum Cooldown", EnvVar: "COOLDOWN_ADX_MOMENTUM_SEC",
			Description: "Per-strategy re-entry cooldown for ADX/Momentum strategy families — the longest default cooldown bucket.",
			DataType: "duration", Unit: "sec", Min: 300, Max: 7200, RecommendedMin: 900, RecommendedMax: 2700,
			RiskLevel: "low",
			WhatIfIncreased: "Momentum strategies re-enter much less often, missing faster continuation legs but avoiding repeated entries into the same exhausted move.",
			WhatIfDecreased: "More frequent re-entry on momentum signals, raising the chance of entering late in an already-extended move.",
			RequiresRestart: true,
		},
		{
			Key: "COOLDOWN_DEFAULT_SEC", Category: CategoryCooldowns,
			Label: "Default Strategy Cooldown", EnvVar: "COOLDOWN_DEFAULT_SEC",
			Description: "Fallback cooldown applied to any strategy name that does not match one of the named pattern buckets in scalerStrategyCooldown.",
			DataType: "duration", Unit: "sec", Min: 5, Max: 600, RecommendedMin: 15, RecommendedMax: 60,
			RiskLevel: "low",
			WhatIfIncreased: "Unmatched/new strategies re-enter less often by default.",
			WhatIfDecreased: "Unmatched/new strategies re-enter more freely by default — review whether they should instead be added to a named bucket.",
			RequiresRestart: true,
		},

		// ── regime_classification ───────────────────────────────────────────
		{
			Key: "REGIME_ATR_HIGH_VOL_RATIO", Category: CategoryRegimeClassification,
			Label: "High-Volatility ATR Ratio", EnvVar: "REGIME_ATR_HIGH_VOL_RATIO",
			Description: "ATR-ratio threshold (current ATR / 20-bar average) above which the Classifier declares HIGH_VOLATILITY and suspends all new entries (PositionSizeMult=0, AllowNewEntries=false).",
			DataType: "decimal", Unit: "x avg", Min: 1.2, Max: 4.0, RecommendedMin: 1.8, RecommendedMax: 2.5,
			RiskLevel: "critical",
			WhatIfIncreased: "The engine tolerates more volatility expansion before halting new entries — more trades continue during turbulent moves, raising tail risk.",
			WhatIfDecreased: "The engine halts new entries earlier during volatility spikes, missing some opportunities but reducing exposure to whipsaw conditions.",
			RequiresRestart: true,
		},
		{
			Key: "REGIME_ATR_EXTREME_VOL_RATIO", Category: CategoryRegimeClassification,
			Label: "Extreme-Volatility ATR Ratio", EnvVar: "REGIME_ATR_EXTREME_VOL_RATIO",
			Description: "Higher ATR-ratio tier that raises HIGH_VOLATILITY confidence from 80 to 95.",
			DataType: "decimal", Unit: "x avg", Min: 2.0, Max: 6.0, RecommendedMin: 2.5, RecommendedMax: 4.0,
			RiskLevel: "medium",
			WhatIfIncreased: "Requires more extreme conditions before the highest-confidence volatility-halt classification fires.",
			WhatIfDecreased: "Reaches maximum volatility-halt confidence sooner during a spike.",
			RequiresRestart: true,
		},
		{
			Key: "REGIME_ADX_TREND_THRESHOLD", Category: CategoryRegimeClassification,
			Label: "ADX Trend Threshold", EnvVar: "REGIME_ADX_TREND_THRESHOLD",
			Description: "Minimum ADX for the Classifier to declare TRENDING_BULL or TRENDING_BEAR (combined with price/EMA/RSI conditions).",
			DataType: "decimal", Unit: "ADX", Min: 15, Max: 40, RecommendedMin: 20, RecommendedMax: 30,
			RiskLevel: "medium",
			WhatIfIncreased: "Requires a stronger trend before applying trend-favoring position-size multipliers (1.25x bull / 0.75x bear) and directional bias — fewer false trend calls but later entry into genuine trends.",
			WhatIfDecreased: "Classifies weaker trends as TRENDING sooner, applying directional bias and size multipliers earlier but with more false positives in choppy conditions.",
			RequiresRestart: true,
		},
		{
			Key: "REGIME_ADX_RANGE_THRESHOLD", Category: CategoryRegimeClassification,
			Label: "ADX Ranging Threshold", EnvVar: "REGIME_ADX_RANGE_THRESHOLD",
			Description: "Maximum ADX (combined with a Bollinger Band width contraction check) for the Classifier to declare RANGING — triggers mean-reversion-only mode with 0.5x position sizing.",
			DataType: "decimal", Unit: "ADX", Min: 10, Max: 30, RecommendedMin: 15, RecommendedMax: 25,
			RiskLevel: "medium",
			WhatIfIncreased: "More conditions get classified as RANGING, applying the conservative 0.5x size multiplier and mean-reversion-only posture more often.",
			WhatIfDecreased: "Fewer conditions qualify as RANGING, leaving more periods in the default/indeterminate posture (0.75x multiplier) instead of the more conservative ranging treatment.",
			RequiresRestart: true,
		},

		// ── execution ───────────────────────────────────────────────────────
		{
			Key: "MIN_EXECUTION_WEIGHT_TO_TRADE", Category: CategoryExecution,
			Label: "Minimum Execution Weight to Trade", EnvVar: "MIN_EXECUTION_WEIGHT_TO_TRADE",
			Description: "Strategy-quality weight floor required before the engine will execute through a strategy at all (separate from per-signal confidence).",
			DataType: "decimal", Unit: "", Min: 0.20, Max: 0.90, RecommendedMin: 0.40, RecommendedMax: 0.65,
			RiskLevel: "high",
			WhatIfIncreased: "Only higher-quality-weighted strategies are allowed to execute at all — shrinks the effective live roster.",
			WhatIfDecreased: "More strategies (including lower-weighted ones) are allowed to execute, widening the roster but admitting weaker performers.",
		},
	}
}
