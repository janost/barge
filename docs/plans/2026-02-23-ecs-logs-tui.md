# ECS Service Logs TUI Resource

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an ECS Service Logs resource to the TUI dashboard that displays recent CloudWatch log events in a table, accessible from the picker.

**Architecture:** New `ECSLogsResource` implements `Resource`. It resolves the log config via `ResolveLogConfig()` and fetches recent events via `FetchLogs()`. Displayed as a table with TIME, STREAM, and MESSAGE columns. Registered in the picker via cluster/service drill-down (reusing the toStatus pattern from the status plan).

**Tech Stack:** Go, Bubble Tea, existing `bargeaws.Client`

---

### Task 1: Create ECSLogsResource

**Files:**
- Create: `tui/dashboard/ecs_logs.go`

**Step 1: Create the resource file**

```go
package dashboard

import (
	"context"
	"time"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsLogsFetchMsg struct {
	events []bargeaws.LogEvent
	err    error
}

type ECSLogsResource struct {
	cluster string
	service string
	events  []bargeaws.LogEvent
	err     error
}

func NewECSLogsResource(cluster, service string) *ECSLogsResource {
	return &ECSLogsResource{cluster: cluster, service: service}
}

func (r *ECSLogsResource) Name() string { return "Logs" }

func (r *ECSLogsResource) Columns() []Column {
	return []Column{
		{"TIME", 20},
		{"STREAM", 30},
		{"MESSAGE", 80},
	}
}

func (r *ECSLogsResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		logCfg, err := client.ResolveLogConfig(ctx, r.cluster, r.service)
		if err != nil {
			return ecsLogsFetchMsg{nil, err}
		}
		since := time.Now().Add(-15 * time.Minute)
		events, _, err := client.FetchLogs(ctx, logCfg.LogGroup, logCfg.StreamPrefix, "", since, nil)
		return ecsLogsFetchMsg{events, err}
	}
}

func (r *ECSLogsResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsLogsFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.events = m.events
		}
		return true
	}
	return false
}

func (r *ECSLogsResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.events))
	for i, e := range r.events {
		rows[i] = table.Row{
			e.Timestamp.Format("15:04:05.000"),
			e.Stream,
			e.Message,
		}
	}
	return rows
}

func (r *ECSLogsResource) Actions(row table.Row) []Action { return nil }

func (r *ECSLogsResource) Error() error { return r.err }
```

**Step 2: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add tui/dashboard/ecs_logs.go
git commit -m "feat: add ECS Logs resource for CloudWatch log viewing in TUI"
```

---

### Task 2: Wire ECSLogsResource into picker via cluster/service drill

**Files:**
- Modify: `tui/dashboard/ecs_service.go` — add `toLogs` drill mode
- Modify: `tui/dashboard/ecs_cluster.go` — add `toLogs` cluster variant
- Modify: `tui/dashboard/picker.go` — add "ECS Service Logs" entry

**Step 1: Add toLogs mode to ECSServiceResource**

This follows the same pattern as `toStatus` from the status plan. Add a `drillTarget` enum instead of multiple booleans:

Replace the `toStatus bool` field (if it exists from the status plan) with a more flexible approach:

```go
type serviceDrillTarget int

const (
	drillToTasks serviceDrillTarget = iota
	drillToStatus
	drillToLogs
)
```

Add to `ECSServiceResource`:
```go
type ECSServiceResource struct {
	cluster     string
	services    []bargeaws.ServiceInfo
	err         error
	drillTarget serviceDrillTarget
}
```

Add constructor:
```go
func NewECSServiceResourceForLogs(cluster string) *ECSServiceResource {
	return &ECSServiceResource{cluster: cluster, drillTarget: drillToLogs}
}
```

Modify ChildResource:
```go
func (r *ECSServiceResource) ChildResource(row table.Row) (string, Resource) {
	switch r.drillTarget {
	case drillToStatus:
		return row[0], NewECSStatusResource(r.cluster, row[0])
	case drillToLogs:
		return row[0], NewECSLogsResource(r.cluster, row[0])
	default:
		return row[0], NewECSTaskResource(r.cluster, row[0])
	}
}
```

**Step 2: Add toLogs mode to ECSClusterResource**

Same pattern — add a `drillTarget` field (shared enum or cluster-specific):

```go
func NewECSClusterResourceForLogs() *ECSClusterResource {
	return &ECSClusterResource{toLogs: true}
}
```

Modify ChildResource to dispatch appropriately.

**Step 3: Register in picker**

Add to Rows():
```go
{"ECS Service Logs", "Recent CloudWatch logs for a service"}
```

Add to ChildResource():
```go
case "ECS Service Logs":
    return "ECS Service Logs", NewECSClusterResourceForLogs()
```

**Step 4: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 5: Verify vet**

Run: `GOCACHE=/tmp go vet ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add tui/dashboard/ecs_logs.go tui/dashboard/ecs_service.go tui/dashboard/ecs_cluster.go tui/dashboard/picker.go
git commit -m "feat: add ECS Service Logs to picker with cluster/service drill-down"
```
