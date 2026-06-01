---
name: feedback-cmd-file-naming
description: Files inside cmd/ subpackages must be named after the folder, not cmd.go
metadata:
  type: feedback
---

Files inside `cmd/<subcommand>/` must be named after the folder (e.g., `cmd/server/server.go`, `cmd/worker/worker.go`, `cmd/migrate/migrate.go`), never `cmd.go`.

**Why:** User said "USE main.go ou o nome da pasta, não cmd: cmd". The file name `cmd.go` inside a folder named `server` is redundant and confusing.
**How to apply:** When creating cobra subcommands under `cmd/`, always name the file after the subcommand folder. Rename existing `cmd.go` files to match their parent directory name.
