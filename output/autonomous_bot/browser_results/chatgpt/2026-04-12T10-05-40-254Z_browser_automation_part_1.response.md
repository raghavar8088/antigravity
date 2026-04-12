Findings

bridge\README.md: The current browser bridge is prototype-only and relies on fragile ChatGPT web selectors. This makes it unsuitable as the primary execution path.

bridge\bridge-decisions.jsonl: The recorded Request failed with status code 404 shows the bridge depends on unstable or mismatched engine endpoints.

Both issues point to the same root problem: browser automation is being used as core infrastructure instead of as a fallback.

Fix plan

Make local analysis, planning, and decision generation the default path.

Downgrade browser chat to a fallback transport only, used when the primary local or API path is unavailable.

Add a queue-based handoff between planning and execution so work is durable and replayable.

Introduce endpoint validation before requests, plus bounded retries with backoff and clear failure classification for 404 vs transient errors.

Add a non-browser primary execution path, such as direct local execution or a stable API/client adapter.

Update bridge docs to mark selector-based automation as best-effort and non-critical.

Verification

Confirm repo can complete core analysis and planning with browser automation disabled.

Simulate invalid endpoints and verify 404 errors are caught, logged, and do not block queued work.

Test retry behavior on transient failures and confirm no retry loop on permanent 404s.

Verify fallback to browser transport occurs only after primary path failure.

Review docs and config to ensure browser mode is explicitly non-primary.