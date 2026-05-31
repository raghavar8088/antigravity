"use client";

import { useState } from "react";
import { TerminalPanel } from "./terminal/TerminalPanel";
import { DeskButton, DeskCard } from "./desk/ui";
import { PineScriptEngine, type PineExecutionResult } from "@/lib/PineScriptEngine";
import type { OHLCVCandle } from "@/lib/mockResearchIndicators";

interface PineEditorPanelProps {
  candles: OHLCVCandle[];
  onExecute?: (result: PineExecutionResult) => void;
}

const DEFAULT_SCRIPT = `// Simple SMA Crossover
fastSMA = ta.sma(close, 10)
slowSMA = ta.sma(close, 20)

longCondition = ta.crossover(fastSMA, slowSMA)
shortCondition = ta.crossunder(fastSMA, slowSMA)

strategy.entry("Long", strategy.long, when = longCondition)
strategy.close("Long", when = shortCondition)
`;

export function PineEditorPanel({ candles, onExecute }: PineEditorPanelProps) {
  const [script, setScript] = useState(DEFAULT_SCRIPT);
  const [result, setResult] = useState<PineExecutionResult | null>(null);

  const handleRun = () => {
    const engine = new PineScriptEngine(candles);
    const res = engine.execute(script);
    setResult(res);
    if (onExecute) onExecute(res);
  };

  return (
    <TerminalPanel
      title="Pine Script™ Editor"
      subtitle="Paste and test institutional Pine strategies"
      headerActions={
        <DeskButton variant="filled" onClick={handleRun}>
          Compile & Run
        </DeskButton>
      }
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 12, height: "100%" }}>
        <textarea
          value={script}
          onChange={(e) => setScript(e.target.value)}
          style={{
            flex: 1,
            minHeight: 200,
            background: "#0d1117",
            color: "#e6edf3",
            fontFamily: "JetBrains Mono, monospace",
            fontSize: 12,
            padding: 12,
            border: "1px solid #30363d",
            borderRadius: 6,
            outline: "none",
            resize: "none",
          }}
          spellCheck={false}
        />

        {result && (
          <div style={{ padding: 10, background: "rgba(48, 54, 61, 0.2)", borderRadius: 6 }}>
            <div style={{ fontSize: 11, fontWeight: 700, marginBottom: 4, color: "var(--text-secondary)" }}>
              EXECUTION LOG
            </div>
            {result.errors.length > 0 ? (
              result.errors.map((err, i) => (
                <div key={i} style={{ color: "#ef5350", fontSize: 11 }}>
                  Error: {err}
                </div>
              ))
            ) : (
              <>
                <div style={{ color: "#26a69a", fontSize: 11 }}>
                  Compilation successful. {result.signals.length} signals generated.
                </div>
                {result.signals.map((sig, i) => (
                  <div key={i} style={{ fontSize: 10, color: "var(--text-muted)", marginTop: 2 }}>
                    [{sig.action.toUpperCase()}] {sig.side?.toUpperCase()} ID: {sig.id}
                  </div>
                ))}
              </>
            )}
          </div>
        )}
      </div>
    </TerminalPanel>
  );
}
