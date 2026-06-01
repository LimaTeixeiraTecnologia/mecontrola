---
name: feedback-no-discard-err
description: Never use _ = err to silently discard errors — use slog.ErrorContext or propagate
metadata:
  type: feedback
---

`_ = err` is forbidden. Every error must be either propagated, wrapped, or logged with `slog.ErrorContext(ctx, "message", "error", err)`.

The ONLY approved exception is `_ = mgr.Shutdown(context.Background())` in deferred cleanup — see [[feedback-shutdown-error-never-remove]].

**Why:** User said "Eu NÃO QUERO QUE ISSO ACONTEÇA: _ = err" — hard rule.
**How to apply:** In goroutines where errors cannot be propagated, log with slog. In deferred cleanup of the specific Shutdown pattern, keep `_ = mgr.Shutdown(ctx)`. Never silently discard any other error.
