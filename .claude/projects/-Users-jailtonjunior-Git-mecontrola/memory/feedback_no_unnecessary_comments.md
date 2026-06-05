---
name: feedback-no-unnecessary-comments
description: Zero comments in Go code — absolute rule, no exceptions
metadata:
  type: feedback
---

Zero comments in all Go source files. No package doc comments, no inline comments, no block comments. The only allowed exception is a `doc.go` file that exists purely as a package declaration placeholder with no comment body.

**Why:** User escalated to "0 comentários obrigatório e inegociável" — stricter than "only WHY comments". Every comment is forbidden.
**How to apply:** Before any commit touching Go files, run `grep -rn "^[[:space:]]*//" internal/` and remove every match. Also remove block comments `/* ... */`. Do not add any comments when writing new Go code.
