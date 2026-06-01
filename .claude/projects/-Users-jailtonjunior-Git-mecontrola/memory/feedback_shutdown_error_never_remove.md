---
name: feedback-shutdown-error-never-remove
description: Never remove _ = mgr.Shutdown(context.Background()) — hard rule about deferred error handling
metadata:
  type: feedback
---

The pattern `defer func() { _ = mgr.Shutdown(context.Background()) }()` must NEVER be removed or simplified to discard the error silently without the explicit `_ =` assignment. This is the intentional error-discard pattern for deferred shutdown calls that cannot propagate errors.

**Why:** User explicitly said "EM HIPOTESE NENHUMA SUMA COM O ERRO: _ = mgr.Shutdown(context.Background())" — this is a hard rule. The `_ =` is intentional and must be preserved.
**How to apply:** When reviewing or refactoring shutdown/cleanup code, always keep the `_ = resource.Shutdown(ctx)` pattern. Never replace with bare `resource.Shutdown(ctx)` (which discards without intent) or remove the call entirely.
