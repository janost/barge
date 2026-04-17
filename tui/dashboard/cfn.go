package dashboard

import (
	"context"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// --- CFN Stacks ---

type cfnStackFetchMsg struct {
	stacks []bridgeaws.CFNStackInfo
	err    error
}

type CFNStackResource struct {
	stacks []bridgeaws.CFNStackInfo
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

func (r *CFNStackResource) FetchCmd(client *bridgeaws.Client) tea.Cmd {
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
	events []bridgeaws.CFNStackEvent
	err    error
}

type CFNStackEventsResource struct {
	stackName string
	events    []bridgeaws.CFNStackEvent
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

func (r *CFNStackEventsResource) FetchCmd(client *bridgeaws.Client) tea.Cmd {
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
