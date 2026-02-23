# ECS Service Status TUI Resource

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an ECS Service Status resource to the TUI dashboard that shows service health: deployments, running/stopped tasks, and counts — accessible as an action from the service row.

**Architecture:** New `ECSStatusResource` implements `Resource`. It fetches combined data via `DescribeServiceDetail()`, `ListTasks()`, and `ListStoppedTasks()`. Displayed as a flat table combining deployments, running tasks, and recently stopped tasks in sections. Accessible as an action on `ECSServiceResource`.

**Tech Stack:** Go, Bubble Tea, existing `bargeaws.Client`

---

### Task 1: Create ECSStatusResource

**Files:**
- Create: `tui/dashboard/ecs_status.go`

**Step 1: Create the resource file**

```go
package dashboard

import (
	"context"
	"fmt"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsStatusFetchMsg struct {
	rows []statusRow
	err  error
}

type statusRow struct {
	section string
	col1    string
	col2    string
	col3    string
}

type ECSStatusResource struct {
	cluster string
	service string
	rows    []statusRow
	err     error
}

func NewECSStatusResource(cluster, service string) *ECSStatusResource {
	return &ECSStatusResource{cluster: cluster, service: service}
}

func (r *ECSStatusResource) Name() string { return "Status" }

func (r *ECSStatusResource) Columns() []Column {
	return []Column{
		{"SECTION", 15},
		{"NAME", 30},
		{"STATUS", 15},
		{"DETAIL", 40},
	}
}

func (r *ECSStatusResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		detail, err := client.DescribeServiceDetail(ctx, r.cluster, r.service)
		if err != nil {
			return ecsStatusFetchMsg{nil, err}
		}

		var rows []statusRow

		// Service summary
		rows = append(rows, statusRow{
			section: "Service",
			col1:    detail.Name,
			col2:    detail.Status,
			col3:    fmt.Sprintf("desired:%d running:%d pending:%d", detail.DesiredCount, detail.RunningCount, detail.PendingCount),
		})

		// Deployments
		for _, d := range detail.Deployments {
			rows = append(rows, statusRow{
				section: "Deployment",
				col1:    d.TaskDef,
				col2:    d.Status,
				col3:    fmt.Sprintf("desired:%d running:%d pending:%d updated:%s", d.DesiredCount, d.RunningCount, d.PendingCount, d.UpdatedAt.Format("15:04:05")),
			})
		}

		// Running tasks
		tasks, err := client.ListTasks(ctx, r.cluster, r.service)
		if err == nil {
			for _, t := range tasks {
				started := ""
				if !t.StartedAt.IsZero() {
					started = t.StartedAt.Format("15:04:05")
				}
				rows = append(rows, statusRow{
					section: "Task",
					col1:    t.ID,
					col2:    t.Status,
					col3:    fmt.Sprintf("rev:%s started:%s", t.Revision, started),
				})
			}
		}

		// Stopped tasks (last 24h)
		stopped, err := client.ListStoppedTasks(ctx, r.cluster, r.service)
		if err == nil {
			for _, s := range stopped {
				rows = append(rows, statusRow{
					section: "Stopped",
					col1:    s.ID,
					col2:    s.StoppedAt.Format("15:04:05"),
					col3:    s.StopReason,
				})
			}
		}

		return ecsStatusFetchMsg{rows, nil}
	}
}

func (r *ECSStatusResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsStatusFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.rows = m.rows
		}
		return true
	}
	return false
}

func (r *ECSStatusResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.rows))
	for i, sr := range r.rows {
		rows[i] = table.Row{sr.section, sr.col1, sr.col2, sr.col3}
	}
	return rows
}

func (r *ECSStatusResource) Actions(row table.Row) []Action { return nil }

func (r *ECSStatusResource) Error() error { return r.err }
```

**Step 2: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add tui/dashboard/ecs_status.go
git commit -m "feat: add ECS Status resource showing service health overview"
```

---

### Task 2: Add Status as an action on ECSServiceResource

**Files:**
- Modify: `tui/dashboard/ecs_service.go`

**Step 1: Replace the empty Actions method**

Change `Actions` from returning nil to returning a "View Status" action that drills into the status resource:

Note: Actions can't drill — they run subprocesses. Instead, we need a different approach. Since ECSServiceResource already uses primary drill for Tasks and we're adding secondary drill for Events, Status should be a third option.

**Alternative approach:** Add Status as a picker entry accessible from the root ResourcePicker under "ECS Service Status". This avoids overloading the service resource with too many drill paths.

Add to `picker.go` Rows():
```go
{"ECS Service Status", "Health overview for a specific service"}
```

And in ChildResource():
```go
case "ECS Service Status":
    return "ECS Service Status", NewECSClusterResource() // drill to cluster first, then service, then status
```

Actually, the cleanest approach: make ECSServiceResource implement Actions() to show a "Status" option that the model handles as a drill-down rather than a subprocess.

**Revised approach:** We'll add a dedicated `Actionable` concept. But that's overengineering. The simplest approach is: **add "ECS Service Status" to the ResourcePicker** so users can navigate Cluster -> Service -> Status through the picker. The status resource needs a cluster+service selection step.

**Simplest approach:** Make ECSStatusResource Drillable from ECSServiceResource by adding it as the **primary** drill and moving Tasks to secondary. But that changes existing behavior.

**Final decision:** Add Status to the ECS service Actions list, but instead of running a subprocess, return a tea.Cmd that sends a custom drill message. Actually, this is still overengineering.

**Simplest correct approach:** Keep `ecs_status.go` as a standalone resource. Create a new `ECSServiceForStatusResource` that lists services (same as ECSServiceResource) but drills into Status instead of Tasks. Register it in the picker as "ECS Service Status".

That's duplicate code. Let's just use a parameter:

**Step 1: Add a drillTarget field to ECSServiceResource**

This allows reusing the service list with different drill targets:

In `ecs_service.go`, add a `drillMode` to the struct:

```go
type ecsServiceDrillMode int

const (
	drillToTasks  ecsServiceDrillMode = iota
	drillToStatus
)
```

And a constructor:
```go
func NewECSServiceResourceForStatus(cluster string) *ECSServiceResource {
	return &ECSServiceResource{cluster: cluster, drillMode: drillToStatus}
}
```

Modify ChildResource to check the mode. But this adds complexity to a simple file.

**FINAL simplest approach:** Just register "ECS Service Status" in picker, which creates an ECSClusterResource variant that drills to a service list that drills to status. Actually no — let's not overthink this.

**The right answer:** Add a keybind. The service resource already has Enter (Tasks) and DrillAlt/T (Events via Task 2 of events plan). We need a third path for Status.

**Actual simplest approach that works today:** Add Status to the picker as a top-level item. When selected, show clusters, then services, then the status view. This reuses existing resources with a parameter.

**Step 1: Add drill mode to ECSServiceResource**

In `ecs_service.go`, add field and modify ChildResource:

```go
type ECSServiceResource struct {
	cluster   string
	services  []bargeaws.ServiceInfo
	err       error
	toStatus  bool
}

func NewECSServiceResourceForStatus(cluster string) *ECSServiceResource {
	return &ECSServiceResource{cluster: cluster, toStatus: true}
}

func (r *ECSServiceResource) ChildResource(row table.Row) (string, Resource) {
	if r.toStatus {
		return row[0], NewECSStatusResource(r.cluster, row[0])
	}
	return row[0], NewECSTaskResource(r.cluster, row[0])
}
```

**Step 2: Create a cluster variant that drills to status**

In `ecs_cluster.go`, add:

```go
func NewECSClusterResourceForStatus() *ECSClusterResource {
	return &ECSClusterResource{toStatus: true}
}
```

Add a `toStatus bool` field and modify ChildResource:

```go
func (r *ECSClusterResource) ChildResource(row table.Row) (string, Resource) {
	if r.toStatus {
		return row[0], NewECSServiceResourceForStatus(row[0])
	}
	return row[0], NewECSServiceResource(row[0])
}
```

**Step 3: Register in picker**

Add to `picker.go` Rows():
```go
{"ECS Service Status", "Deployments, tasks, and recent stops"}
```

Add to ChildResource():
```go
case "ECS Service Status":
    return "ECS Service Status", NewECSClusterResourceForStatus()
```

**Step 4: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 5: Verify vet**

Run: `GOCACHE=/tmp go vet ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add tui/dashboard/ecs_status.go tui/dashboard/ecs_service.go tui/dashboard/ecs_cluster.go tui/dashboard/picker.go
git commit -m "feat: add ECS Service Status to picker with cluster/service drill-down"
```
