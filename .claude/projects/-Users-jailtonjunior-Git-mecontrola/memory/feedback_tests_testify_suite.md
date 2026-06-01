---
name: feedback-tests-testify-suite
description: All _test.go files must follow testify/suite R4 pattern from go-implementation/references/testing.md
metadata:
  type: feedback
---

Every `_test.go` covering use cases, services, or handlers MUST use `testify/suite` with table-driven tests per R4 from `.agents/skills/go-implementation/references/testing.md`. Bare `t.Run` with `t.Parallel()` is only allowed for simple utility functions without injectable dependencies.

Required structure: (1) package declaration, (2) imports in 3 groups (stdlib | testify/external | mocks/internal), (3) Suite struct embedding `suite.Suite` with typed mock fields, (4) `TestXxx(t)` registrar calling only `suite.Run(t, new(XxxSuite))`, (5) `SetupTest()` reinitializing all mocks, (6) test method with `scenarios` table, (7) loop with `s.Run` instantiating SUT inside loop.

**Why:** User explicitly flagged that test files were not following testing.md standard.
**How to apply:** Any time a _test.go is written or modified, apply the R4 structure. Check `.agents/skills/go-implementation/references/testing.md` for the canonical skeleton.
