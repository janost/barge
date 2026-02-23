# ECS Service Events TUI Resource

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an ECS Service Events resource to the TUI dashboard, accessible by drilling into a service and pressing the DrillAlt key.

**Architecture:** New `ECSEventsResource` implements `Resource`. It fetches events via the existing `DescribeServiceDetail()` method and displays them as a table with TIME and MESSAGE columns. Accessible as a secondary drill from `ECSServiceResource`.

**Tech Stack:** Go, Bubble Tea, existing `bargeaws.Client`

---

### Task 1: Create ECSEventsResource

**Files:**
- Create: `tui/dashboard/ecs_events.go`

**Step 1: Create the resource file**

```go
package dashboard

import (
	"context"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsEventsFetchMsg struct {
	events []bargeaws.ServiceEvent
	err    error
}

type ECSEventsResource struct {
	cluster string
	service string
	events  []bargeaws.ServiceEvent
	err     error
}

func NewECSEventsResource(cluster, service string) *ECSEventsResource {
	return &ECSEventsResource{cluster: cluster, service: service}
}

func (r *ECSEventsResource) Name() string { return "Events" }

func (r *ECSEventsResource) Columns() []Column {
	return []Column{
		{"TIME", 20},
		{"MESSAGE", 80},
	}
}

func (r *ECSEventsResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		detail, err := client.DescribeServiceDetail(context.Background(), r.cluster, r.service)
		if err != nil {
			return ecsEventsFetchMsg{nil, err}
		}
		return ecsEventsFetchMsg{detail.Events, nil}
	}
}

func (r *ECSEventsResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsEventsFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.events = m.events
		}
		return true
	}
	return false
}

func (r *ECSEventsResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.events))
	for i, e := range r.events {
		rows[i] = table.Row{
			e.Timestamp.Format("2006-01-02 15:04:05"),
			e.Message,
		}
	}
	return rows
}

func (r *ECSEventsResource) Actions(row table.Row) []Action { return nil }

func (r *ECSEventsResource) Error() error { return r.err }
```

**Step 2: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS (no errors)

**Step 3: Commit**

```bash
git add tui/dashboard/ecs_events.go
git commit -m "feat: add ECS Events resource for service event timeline"
```

---

### Task 2: Wire ECSEventsResource as secondary drill from ECSServiceResource

**Files:**
- Modify: `tui/dashboard/ecs_service.go`

**Step 1: Add SecondaryDrillable implementation**

Add this method to `ECSServiceResource`:

```go
func (r *ECSServiceResource) SecondaryChildResource(row table.Row) (string, Resource) {
	return row[0], NewECSEventsResource(r.cluster, row[0])
}
```

**Step 2: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 3: Verify vet**

Run: `GOCACHE=/tmp go vet ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add tui/dashboard/ecs_service.go
git commit -m "feat: wire service events as secondary drill (DrillAlt) from ECS Services"
```
