# Barge - Project Instructions

## What this is

Barge is a k9s-style TUI for AWS ECS and EC2 management. Go + Bubble Tea + Cobra.

## Build & test

```bash
make build                    # build ./barge
make vet                      # go vet ./...
go build ./...                # check compilation
GOCACHE=/tmp go build ./...   # if sandbox blocks default cache
```

No test suite yet. Verify changes compile and run `go vet ./...`.

## Architecture

```
main.go           Entry point, sets version, calls cmd.Execute()
cmd/               Cobra CLI commands (root, tui, exec, logs, status, events, cp, list, etc.)
aws/               AWS SDK wrappers (ecs.go, ssm.go, asg.go, logs.go) — no UI logic
tui/               Terminal UI
  model.go         Legacy drill-down model (still used by logs, status commands)
  dashboard/       New resource-based dashboard (the active architecture)
    model.go       Main Bubble Tea model — handles state, keys, rendering
    resource.go    Core interfaces: Resource, Drillable, SecondaryDrillable, Action
    picker.go      ResourcePicker (root view — menu of resource types)
    ec2.go         EC2InstanceResource
    asg.go         ASGResource (drills to filtered EC2 instances)
    ecs_cluster.go ECSClusterResource (primary: services, secondary: all tasks)
    ecs_service.go ECSServiceResource (drills to tasks)
    ecs_task.go    ECSTaskResource (leaf — actions: ECS Exec)
    ecs_taskdef.go ECSTaskDefResource
    actions.go     Action factories: SSMShellAction, ECSExecAction, NewExecAction
    styles.go      Dashboard lipgloss styles
config/            Configuration (keybinds, bookmarks, history)
  config.go        Keybinds — YAML at ~/.config/barge/config.yaml
  bookmarks.go     Named connection targets
  history.go       Recent connections — JSON at ~/.local/share/barge/
  paths.go         Directory helpers
exec/              Process execution
  handoff.go       Launch SSM sessions or ECS exec
  copy.go          File copy via tar-over-exec
  bordered.go      BorderedExec: PTY proxy with border/title bar
```

## Key interfaces

**Resource** (`tui/dashboard/resource.go`): Core abstraction for all browsable items.
Every resource type implements: `Name()`, `Columns()`, `FetchCmd()`, `HandleMsg()`, `Rows()`, `Actions()`, `Error()`.

**Drillable**: Optional interface — `ChildResource(row)` returns a child resource for Enter key drill-down.

**SecondaryDrillable**: Optional — `SecondaryChildResource(row)` for alternate drill-down path (DrillAlt keybind).

**Action**: Struct with `Name` and `Run func() tea.Cmd`. Actions appear in a popup menu on leaf resources.

## Adding a new resource type

1. Create `tui/dashboard/<name>.go`
2. Implement the `Resource` interface (and optionally `Drillable`/`SecondaryDrillable`)
3. Define a fetch message type and handle it in `HandleMsg()`
4. Register in `tui/dashboard/picker.go` inside `newResourcePicker()`
5. If it needs new AWS API calls, add methods to `aws/` (keep AWS logic out of TUI code)

## Patterns to follow

- **Async data fetching**: `FetchCmd()` returns a `tea.Cmd` that calls AWS and sends a typed message. `HandleMsg()` stores the result. `Rows()` returns current state.
- **Resource stack**: Dashboard model maintains `resourceStack []Resource` and `breadcrumbs []string`. `drillDown()` pushes, `drillBack()` pops.
- **Subprocess execution**: Use `NewExecAction()` which wraps commands in `BorderedExec` (PTY proxy with title bar). The TUI suspends during exec via `tea.Exec`.
- **Search**: Substring matching, multi-term AND logic, case-insensitive. Applied in `applySearch()` on the model.
- **Config merging**: `config.Default()` provides base values. `config.Load()` unmarshals YAML on top — unspecified fields keep defaults.

## Conventions

- Module path: `github.com/janost/barge`
- AWS client wrapper package imported as `bargeaws`
- Exec package imported as `bargeexec`
- ARN helper: `shortName(arn)` extracts the short name from any AWS ARN
- All AWS API calls go through `aws/*.go`, never called directly from TUI code
- Resource files are named after the AWS resource they represent (`ec2.go`, `ecs_cluster.go`, `asg.go`)
- No tests yet — verify with `go build ./...` and `go vet ./...`

## Gotchas

- `GOCACHE` may need to be set to `/tmp` in sandboxed environments where the default cache dir isn't writable
- The legacy `tui/model.go` is still used by `cmd/logs.go` and `cmd/status.go` — the new dashboard lives in `tui/dashboard/`
- Bubbletea v1.3.10 does not support enhanced keyboard protocol — modifier keys (Shift+Enter) don't work reliably. That's why DrillAlt defaults to `T` instead of `shift+enter`.
- `BorderedExec` intercepts ANSI sequences (DECSTR, ED) to protect the border from being cleared by shell programs
