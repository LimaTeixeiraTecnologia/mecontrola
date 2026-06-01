---
name: feedback-no-unnecessary-comments
description: Remove all unnecessary comments from Go code — only keep comments that explain WHY, never WHAT
metadata:
  type: feedback
---

Never write comments that restate what the code does. Only add a comment when the WHY is non-obvious: a hidden constraint, a subtle invariant, or a workaround. Doc comments that just describe the function signature or rephrase the type name are forbidden.

**Why:** User explicitly requested removing excessive comments — "muito comentário totalmente desnecessário".
**How to apply:** Before committing any Go file, strip all comments that would be obvious from the code or the name. Keep only comments explaining invariants, workarounds, or non-obvious constraints.
