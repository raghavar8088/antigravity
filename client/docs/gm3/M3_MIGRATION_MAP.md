# M3 Token Migration Map

| Legacy (globals.css) | Desk (--desk-*) | M3 Canonical |
|---------------------|-----------------|--------------|
| `--bg` | `--desk-surface-dim` | `--m3-surface-dim` |
| `--surface` | `--desk-surface` | `--m3-surface-container` |
| `--accent` #2962ff | `--desk-primary` | `--m3-primary` |
| `--green` | `--desk-success` | `--m3-profit` |
| `--red` | `--desk-error` | `--m3-loss` |
| `--text-primary` | `--desk-on-surface` | `--m3-on-surface` |
| `--border` | `--desk-outline` | `--m3-outline-variant` |
| `--radius-card` | `--desk-radius-card` | `--m3-radius-sm` |
| Tailwind `#080b10` | — | `--m3-surface-dim` |
| Tailwind `zinc-800` | — | `--m3-outline-variant` |
| Tailwind `sky-400` | — | `--m3-primary` |
| Inline `#0f172a` | — | `--m3-surface-dim` (remove inline) |

**Rule:** New code uses `--m3-*` only. Legacy aliases remain for backward compatibility until Phase 10 cleanup.
