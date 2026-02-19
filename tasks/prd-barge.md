# PRD: Barge — AWS ECS Task Shell TUI

## Introduction

Barge is a terminal UI tool that simplifies connecting to running AWS ECS tasks. Today, opening a shell to an ECS container requires assembling a verbose `aws ecs execute-command` invocation with the correct cluster, service, task ID, and container name — information that must be gathered across multiple AWS CLI calls. Barge replaces this manual parameter-hunting with an interactive drill-down interface that discovers available resources and lets you connect in seconds.

Built as a single static Go binary using Bubble Tea, barge is portable, requires no runtime dependencies beyond the AWS CLI (for the `execute-command` session plugin), and provides a polished terminal experience with fuzzy filtering at every selection step.

## Goals

- Eliminate the need to manually look up cluster, service, task, and container identifiers
- Provide a fast, keyboard-driven hierarchical selection flow with fuzzy search/filtering
- Ship as a single portable binary (no Python, no pip, no virtualenv)
- Support both interactive TUI mode and fully non-interactive CLI mode (all params via flags)
- Support AWS profile and region selection via CLI flags with fallback to standard AWS environment/config
- Architect internals to allow future session-wrapping features (reconnect, logging) without rewrite

## User Stories

### US-001: Select AWS profile and region
**Description:** As a DevOps engineer, I want to specify which AWS profile and region to use so that I can connect to tasks across multiple accounts and regions.

**Acceptance Criteria:**
- [ ] `--profile` flag sets the AWS profile (overrides `AWS_PROFILE` env var)
- [ ] `--region` flag sets the AWS region (overrides `AWS_DEFAULT_REGION` / config file)
- [ ] When neither flag is provided, standard AWS SDK credential/config resolution is used
- [ ] Invalid profile or region produces a clear error message before entering the TUI

### US-002: Browse and select cluster
**Description:** As a DevOps engineer, I want to see all available ECS clusters and quickly filter them so that I can pick the right environment.

**Acceptance Criteria:**
- [ ] On launch (with no flags), barge lists all ECS clusters in the configured account/region
- [ ] List supports real-time fuzzy text filtering as the user types
- [ ] Clusters are sorted alphabetically
- [ ] Pressing Enter on a cluster advances to the service selection step
- [ ] If only one cluster exists, it is auto-selected (with a brief notice)

### US-003: Browse and select service
**Description:** As a DevOps engineer, I want to see all services in the selected cluster with key metadata so that I can pick the right service.

**Acceptance Criteria:**
- [ ] After selecting a cluster, barge lists all services in that cluster
- [ ] Each service row shows: service name, running task count, task definition family
- [ ] List supports real-time fuzzy text filtering
- [ ] Pressing Enter on a service advances to the task selection step
- [ ] Pressing Escape returns to cluster selection
- [ ] If only one service exists, it is auto-selected (with a brief notice)

### US-004: Browse and select task
**Description:** As a DevOps engineer, I want to see running tasks for the selected service so that I can pick the right instance.

**Acceptance Criteria:**
- [ ] After selecting a service, barge lists all running tasks
- [ ] Each task row shows: task ID (short), status, started-at timestamp, task definition revision
- [ ] Tasks are sorted by start time (most recent first)
- [ ] List supports real-time fuzzy text filtering
- [ ] Pressing Enter on a task advances to the container selection step
- [ ] Pressing Escape returns to service selection
- [ ] If only one task exists, it is auto-selected (with a brief notice)

### US-005: Browse and select container
**Description:** As a DevOps engineer, I want to pick which container in a task to connect to, with automatic selection when unambiguous.

**Acceptance Criteria:**
- [ ] After selecting a task, barge lists containers in that task
- [ ] Each container row shows: container name, image, status, whether it is the essential container
- [ ] If only one container exists, it is auto-selected (with a brief notice)
- [ ] If multiple containers exist but only one is marked essential, the essential one is pre-highlighted
- [ ] Pressing Enter on a container initiates the shell connection
- [ ] Pressing Escape returns to task selection

### US-006: Connect to container shell
**Description:** As a DevOps engineer, I want barge to open an interactive shell session in the selected container so that I can debug and operate within the running task.

**Acceptance Criteria:**
- [ ] Barge executes `aws ecs execute-command` with `--interactive` using the selected cluster, task, container, and command
- [ ] Default command is `/bin/sh` (not bash, since many containers lack bash)
- [ ] Command is configurable via `--command` flag
- [ ] Before handing off, barge displays a summary line: cluster, service, task ID, container, command
- [ ] Barge cleanly exits its TUI before handing off to the interactive session (no rendering artifacts)
- [ ] Ctrl+C during the session terminates the shell, not barge itself

### US-007: Non-interactive CLI mode
**Description:** As a DevOps engineer, I want to bypass the TUI entirely by providing all parameters as flags so that I can script connections or use shell history.

**Acceptance Criteria:**
- [ ] `barge --cluster X --service Y [--task Z] [--container C] [--command CMD]` skips the TUI and connects directly
- [ ] `--cluster` and `--service` are the minimum required flags for non-interactive mode
- [ ] When `--task` is omitted, the most recently started task is auto-selected
- [ ] When `--container` is omitted, auto-selection logic applies (single container or single essential container)
- [ ] If auto-selection fails (ambiguous), barge exits with a clear error listing available options
- [ ] Exit codes are meaningful: 0 = success, 1 = connection error, 2 = parameter/selection error

### US-008: List clusters and services (read-only overview)
**Description:** As a DevOps engineer, I want a quick way to see an overview of all my ECS clusters, services, and their status without entering the interactive flow.

**Acceptance Criteria:**
- [ ] `barge list` displays a table of all clusters and their services
- [ ] Table columns: Cluster, Service, Running Tasks, Task Definition
- [ ] Table is rendered with aligned columns in the terminal
- [ ] Supports `--profile` and `--region` flags
- [ ] Output is clean enough to pipe to other tools (no TUI escape codes in non-TTY mode)

## Functional Requirements

- FR-1: Barge must use the AWS SDK for Go v2 for all ECS API calls (list clusters, describe services, list/describe tasks, describe task definitions)
- FR-2: Barge must use Bubble Tea for all interactive TUI rendering
- FR-3: Each selection step (cluster, service, task, container) must support real-time fuzzy filtering via keyboard input
- FR-4: Single-option steps must auto-select with a brief visual indication, not require manual confirmation
- FR-5: Escape key must navigate back one step at any point in the drill-down flow
- FR-6: Barge must cleanly restore the terminal before executing `aws ecs execute-command` (via `os/exec` with stdin/stdout/stderr passthrough)
- FR-7: Barge must handle AWS API errors gracefully with human-readable messages (e.g., "No clusters found", "Access denied — check your AWS credentials")
- FR-8: Barge must show a loading indicator while fetching data from AWS APIs
- FR-9: The `--command` flag must default to `/bin/sh`
- FR-10: `barge list` must produce plain-text tabular output suitable for terminal display and piping
- FR-11: All CLI flags must have short-form aliases (`-p` for `--profile`, `-r` for `--region`, `-c` for `--cluster`, `-s` for `--service`, `-t` for `--task`, `-C` for `--container`, `-x` for `--command`)

## Non-Goals

- No session wrapping, reconnection, or session logging in v1 (architecture should not preclude it)
- No log tailing or CloudWatch integration
- No task/service mutation operations (stop, restart, scale)
- No support for ECS Anywhere or external launch types
- No built-in AWS SSO login flow (user must have valid credentials before running barge)
- No configuration file or persistent settings
- No multi-region aggregate view

## Technical Considerations

- **Language:** Go (latest stable)
- **TUI Framework:** Bubble Tea + Lip Gloss (styling) + Bubbles (common components like lists, spinners, text input)
- **AWS SDK:** `github.com/aws/aws-sdk-go-v2` with standard config loading
- **CLI Parsing:** Cobra or the stdlib `flag` package — Cobra preferred for subcommand support (`barge list`)
- **Shell Handoff:** `syscall.Exec` (on Unix) or `os/exec` to replace the barge process with the `aws ecs execute-command` process, ensuring clean signal handling
- **Build:** Single binary via `go build`, cross-compiled for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- **Architecture:** Separate the AWS data-fetching layer from the TUI layer so that future session features can be added without restructuring the UI
- **Dependencies at runtime:** AWS CLI v2 must be installed (required for `execute-command` session manager plugin). Barge should verify this on startup and show a helpful error if missing.

## Success Metrics

- Connect to a container shell in under 10 seconds (from launch to shell prompt, excluding AWS API latency)
- Zero manual ARN or ID lookup required for any connection
- Single binary under 20MB
- Works on macOS (arm64/amd64) and Linux (arm64/amd64) without modification

## Open Questions

- Should barge cache cluster/service discovery results for faster subsequent launches, or always fetch fresh?
- Should `barge list` support JSON output (`--output json`) for machine consumption?
- Should there be a `--dry-run` flag that prints the `aws ecs execute-command` invocation without executing it?
- For the shell handoff, should barge use `syscall.Exec` (full process replacement, cleanest) or `os/exec.Command` (subprocess, allows future post-session hooks)?
