# CloudFormation Stacks Resource

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add CloudFormation stack browsing to the TUI dashboard with stack status, events drill-down, and outputs viewing.

**Architecture:** New AWS SDK dependency (`cloudformation`). New `aws/cfn.go` with client methods. Two resources: `CFNStackResource` (list stacks, drill into events) and `CFNStackEventsResource` (stack event timeline). Registered in the picker.

**Tech Stack:** Go, AWS CloudFormation SDK, Bubble Tea

---

### Task 1: Add CloudFormation SDK dependency and AWS client methods

**Files:**
- Run: `go get github.com/aws/aws-sdk-go-v2/service/cloudformation`
- Modify: `aws/ecs.go` — add `cfn` field to Client struct and NewClient
- Create: `aws/cfn.go`

**Step 1: Add the dependency**

Run: `go get github.com/aws/aws-sdk-go-v2/service/cloudformation`

**Step 2: Add cfn client to Client struct**

In `aws/ecs.go`, add to the Client struct:
```go
cfn *cloudformation.Client
```

Add import: `"github.com/aws/aws-sdk-go-v2/service/cloudformation"`

In `NewClient()`, add:
```go
cfn: cloudformation.NewFromConfig(cfg),
```

**Step 3: Create aws/cfn.go**

```go
package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

type CFNStackInfo struct {
	Name       string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Outputs    []CFNOutput
}

type CFNOutput struct {
	Key         string
	Value       string
	Description string
}

type CFNStackEvent struct {
	Timestamp    time.Time
	ResourceType string
	LogicalID    string
	Status       string
	Reason       string
}

func (c *Client) ListCFNStacks(ctx context.Context) ([]CFNStackInfo, error) {
	var stacks []CFNStackInfo
	p := cloudformation.NewListStacksPaginator(c.cfn, &cloudformation.ListStacksInput{
		StackStatusFilter: []cfntypes.StackStatus{
			cfntypes.StackStatusCreateComplete,
			cfntypes.StackStatusUpdateComplete,
			cfntypes.StackStatusUpdateRollbackComplete,
			cfntypes.StackStatusRollbackComplete,
			cfntypes.StackStatusCreateInProgress,
			cfntypes.StackStatusUpdateInProgress,
			cfntypes.StackStatusDeleteInProgress,
		},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing stacks: %w", err)
		}
		for _, s := range page.StackSummaries {
			var updatedAt time.Time
			if s.LastUpdatedTime != nil {
				updatedAt = *s.LastUpdatedTime
			}
			var createdAt time.Time
			if s.CreationTime != nil {
				createdAt = *s.CreationTime
			}
			stacks = append(stacks, CFNStackInfo{
				Name:      awssdk.ToString(s.StackName),
				Status:    string(s.StackStatus),
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			})
		}
	}
	sort.Slice(stacks, func(i, j int) bool {
		return stacks[i].Name < stacks[j].Name
	})
	return stacks, nil
}

func (c *Client) ListCFNStackEvents(ctx context.Context, stackName string) ([]CFNStackEvent, error) {
	out, err := c.cfn.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
		StackName: awssdk.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("describing stack events: %w", err)
	}

	events := make([]CFNStackEvent, 0, len(out.StackEvents))
	for _, e := range out.StackEvents {
		var ts time.Time
		if e.Timestamp != nil {
			ts = *e.Timestamp
		}
		events = append(events, CFNStackEvent{
			Timestamp:    ts,
			ResourceType: awssdk.ToString(e.ResourceType),
			LogicalID:    awssdk.ToString(e.LogicalResourceId),
			Status:       string(e.ResourceStatus),
			Reason:       awssdk.ToString(e.ResourceStatusReason),
		})
	}
	return events, nil
}
```

**Step 4: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add go.mod go.sum aws/ecs.go aws/cfn.go
git commit -m "feat: add CloudFormation AWS client methods"
```

---

### Task 2: Create CloudFormation dashboard resources

**Files:**
- Create: `tui/dashboard/cfn.go`

**Step 1: Create the resource file**

```go
package dashboard

import (
	"context"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// --- CFN Stacks ---

type cfnStackFetchMsg struct {
	stacks []bargeaws.CFNStackInfo
	err    error
}

type CFNStackResource struct {
	stacks []bargeaws.CFNStackInfo
	err    error
}

func NewCFNStackResource() *CFNStackResource {
	return &CFNStackResource{}
}

func (r *CFNStackResource) Name() string { return "CloudFormation Stacks" }

func (r *CFNStackResource) Columns() []Column {
	return []Column{
		{"STACK", 30},
		{"STATUS", 30},
		{"CREATED", 20},
		{"UPDATED", 20},
	}
}

func (r *CFNStackResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		stacks, err := client.ListCFNStacks(context.Background())
		return cfnStackFetchMsg{stacks, err}
	}
}

func (r *CFNStackResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(cfnStackFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.stacks = m.stacks
		}
		return true
	}
	return false
}

func (r *CFNStackResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.stacks))
	for i, s := range r.stacks {
		updated := ""
		if !s.UpdatedAt.IsZero() {
			updated = s.UpdatedAt.Format("2006-01-02 15:04:05")
		}
		rows[i] = table.Row{
			s.Name,
			s.Status,
			s.CreatedAt.Format("2006-01-02 15:04:05"),
			updated,
		}
	}
	return rows
}

func (r *CFNStackResource) Actions(row table.Row) []Action { return nil }
func (r *CFNStackResource) Error() error                   { return r.err }

func (r *CFNStackResource) ChildResource(row table.Row) (string, Resource) {
	return row[0], NewCFNStackEventsResource(row[0])
}

// --- CFN Stack Events ---

type cfnEventsFetchMsg struct {
	events []bargeaws.CFNStackEvent
	err    error
}

type CFNStackEventsResource struct {
	stackName string
	events    []bargeaws.CFNStackEvent
	err       error
}

func NewCFNStackEventsResource(stackName string) *CFNStackEventsResource {
	return &CFNStackEventsResource{stackName: stackName}
}

func (r *CFNStackEventsResource) Name() string { return "Stack Events" }

func (r *CFNStackEventsResource) Columns() []Column {
	return []Column{
		{"TIME", 20},
		{"RESOURCE", 30},
		{"LOGICAL ID", 25},
		{"STATUS", 25},
		{"REASON", 40},
	}
}

func (r *CFNStackEventsResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		events, err := client.ListCFNStackEvents(context.Background(), r.stackName)
		return cfnEventsFetchMsg{events, err}
	}
}

func (r *CFNStackEventsResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(cfnEventsFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.events = m.events
		}
		return true
	}
	return false
}

func (r *CFNStackEventsResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.events))
	for i, e := range r.events {
		rows[i] = table.Row{
			e.Timestamp.Format("2006-01-02 15:04:05"),
			e.ResourceType,
			e.LogicalID,
			e.Status,
			e.Reason,
		}
	}
	return rows
}

func (r *CFNStackEventsResource) Actions(row table.Row) []Action { return nil }
func (r *CFNStackEventsResource) Error() error                   { return r.err }
```

**Step 2: Register in picker**

Add to `picker.go` Rows():
```go
{"CloudFormation Stacks", "Stack status and events"},
```

Add to ChildResource():
```go
case "CloudFormation Stacks":
    return "CloudFormation Stacks", NewCFNStackResource()
```

**Step 3: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 4: Verify vet**

Run: `GOCACHE=/tmp go vet ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/dashboard/cfn.go tui/dashboard/picker.go
git commit -m "feat: add CloudFormation Stacks and Events resources to dashboard"
```
