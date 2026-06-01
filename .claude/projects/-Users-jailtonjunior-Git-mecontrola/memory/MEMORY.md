# Memory

- [No unnecessary comments](feedback_no_unnecessary_comments.md) — strip comments that restate code; keep only WHY comments
- [Tests must use testify/suite R4](feedback_tests_testify_suite.md) — all _test.go covering services/handlers must use suite pattern
- [Deployment folder is non-negotiable](feedback_deployment_folder.md) — docker/fly/grafana/runbooks go under deployment/ by category
- [Never remove shutdown error](feedback_shutdown_error_never_remove.md) — _ = mgr.Shutdown(ctx) must always be kept as-is
- [cmd file naming convention](feedback_cmd_file_naming.md) — name files after the folder, not cmd.go
- [Never discard errors with _ = err](feedback_no_discard_err.md) — use slog.ErrorContext or propagate; only _ = mgr.Shutdown approved
- [Load full go-implementation skill](feedback_load_go_implementation_skill.md) — load all needed references, mandatory for any Go task
