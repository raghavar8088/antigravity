# ChatGPT Browser Bridge

This is a prototype-only browser automation bridge for manual-assisted trading.

## Flow

1. The engine parks a strategy signal at `GET /api/ai/pending`.
2. `bridge.js` connects to your logged-in Chrome tab on `chatgpt.com`.
3. The bridge pastes the prompt and waits for ChatGPT to return strict JSON.
4. The bridge posts the result to `POST /api/ai/bridge-result`.
5. The engine validates the result and executes the trade if approved.

## Required ChatGPT reply format

The engine now asks ChatGPT to return only:

```json
{
  "approved": true,
  "action": "BUY",
  "confidence": 0.91,
  "reason": "short reason"
}
```

## How to run

1. Start the engine.
2. In another terminal:

```powershell
cd bridge
node bridge.js
```

Or use:

```powershell
cd bridge
.\RAIG_ROBOT.ps1
```

## Test the bridge

Send a fake signal:

```powershell
Invoke-WebRequest -Method Post -Uri http://localhost:8080/api/ai/test-signal
```

Then watch the bridge terminal and engine logs.

## Bridge logs

Each bridge decision is appended to:

```text
bridge/bridge-decisions.jsonl
```

This file records parsed ChatGPT verdicts, engine submissions, and bridge errors.

## Session readiness checks

Before sending a signal, the bridge now checks whether ChatGPT is actually usable.

It will stop and log a clear error if:

- ChatGPT login is required
- access is blocked by verification/CAPTCHA
- the prompt box is missing

These conditions are written to `bridge/bridge-decisions.jsonl` as `bridge_error` entries.

## Automatic recovery

If the ChatGPT tab refreshes, navigates, or loses the active session, the bridge now tries to:

- re-find the ChatGPT tab
- bring it to the front
- verify the prompt box is usable again

Recovery attempts are also written to `bridge/bridge-decisions.jsonl`.

## Important limitations

- This depends on ChatGPT web UI selectors and can break when the site changes.
- Browser sessions can expire.
- Response timing is variable.
- Use paper trading only until you are confident in the loop.
