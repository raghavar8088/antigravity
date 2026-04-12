# Verification Prompt Pack

Review the following repo issues and propose concrete fixes.
Keep changes minimal, preserve behavior, and call out verification steps.

## Issue 1: Verification command failed: python_bot_import

- Severity: high
- Category: verification
- Detail: A local verification command failed, which blocks reliable autonomous upgrades.
- Evidence: `Traceback (most recent call last):
  File "C:\Trading apllication\autonomous_ai_bot.py", line 9, in <module>
    import anthropic
ModuleNotFoundError: No module named 'anthropic'`
- Requested action: Fix the failure and rerun `C:\Users\ragha\AppData\Local\Programs\Python\Python311\python.exe autonomous_ai_bot.py` from `C:\Trading apllication`.

## Issue 2: Verification command failed: client_lint

- Severity: medium
- Category: verification
- Detail: A local verification command failed, which blocks reliable autonomous upgrades.
- Evidence: `C:\Trading apllication\client\src\hooks\useCryptoEquityEngine.ts
  888:11  error  Error: This value cannot be modified
Modifying a value previously passed as an argument to a hook is not allowed. Consider moving the modification before calling the hook.
C:\Trading apllication\client\src\hooks\useCryptoEquityEngine.ts:888:11
  886 |         const latest = engine.quotes[strategy.position.asset.symbol];
  887 |         if (latest?.price > 0) {
> 888 |           strategy.position.currentPrice = latest.price;
      |           ^^^^^^^^^^^^^^^^^ `engineRef` cannot be modified
  889 |           strategy.position.unrealizedPnl = calcPnl(strategy.position.side, strategy.position.entryPrice, latest.price, strategy.position.quantity);
  890 |           strategy.position.returnPct = strategy.position.notional > 0 ? (strategy.position.unrealizedPnl / strategy.position.notional) * 100 : 0;
  891 |           strategy.position.peakReturnPct = Math.max(strategy.position.peakReturnPct, strategy.position.returnPct);  react-hooks/immutability
âœ– 2 problems (1 error, 1 warning)`
- Requested action: Fix the failure and rerun `cmd /c npm run lint` from `C:\Trading apllication\client`.
