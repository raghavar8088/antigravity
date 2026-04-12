Findings

bridge\README.md already signals the browser bridge is prototype-only, so the current browser-automation path is too fragile to be a primary execution layer.

Reliance on ChatGPT web selectors creates break risk from UI changes, auth/session changes, and timing issues.

bridge\bridge-decisions.jsonl shows a hard runtime failure: Request failed with status code 404, which means the bridge is expecting endpoints that are missing, changed, or unavailable.

A browser-first design is the wrong control plane for core analysis/planning; those should run locally and deterministically, with browser chat only as fallback transport.

Fix plan

Move core analysis, planning, and decision logic into a local execution module with stable inputs/outputs.

Redefine the browser bridge as fallback-only transport behind an adapter interface.

Add a queue-based handoff between planner and transport so failed browser delivery does not block execution.

Implement retry policy with bounded exponential backoff, idempotent message IDs, and failure classification for 404 vs transient errors.

Add a non-browser primary path such as local CLI/service execution, with browser automation used only when explicitly required.

Update docs to mark browser selectors as best-effort and unsupported for primary reliability.

Verification

Confirm local deterministic path completes end-to-end with browser disabled.

Simulate 404 responses and verify queue persistence, retry behavior, and fallback routing.

Run contract tests for transport adapters and ensure planner output is identical across transports.

Update README and check it no longer presents browser automation as primary.