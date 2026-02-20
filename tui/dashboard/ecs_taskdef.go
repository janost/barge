package dashboard

import (
	"context"
	"fmt"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsTaskDefFetchMsg struct {
	taskDefs []bargeaws.TaskDefInfo
	err      error
}

type ECSTaskDefResource struct {
	taskDefs []bargeaws.TaskDefInfo
	err      error
}

func NewECSTaskDefResource() *ECSTaskDefResource {
	return &ECSTaskDefResource{}
}

func (r *ECSTaskDefResource) Name() string { return "ECS Task Definitions" }

func (r *ECSTaskDefResource) Columns() []Column {
	return []Column{
		{"FAMILY", 40},
		{"LATEST REV", 12},
	}
}

func (r *ECSTaskDefResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		taskDefs, err := client.ListTaskDefinitions(context.Background())
		return ecsTaskDefFetchMsg{taskDefs, err}
	}
}

func (r *ECSTaskDefResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsTaskDefFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.taskDefs = m.taskDefs
		}
		return true
	}
	return false
}

func (r *ECSTaskDefResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.taskDefs))
	for i, td := range r.taskDefs {
		rows[i] = table.Row{
			td.Family,
			fmt.Sprintf("%d", td.Revision),
		}
	}
	return rows
}

func (r *ECSTaskDefResource) Actions(row table.Row) []Action { return nil }

func (r *ECSTaskDefResource) Error() error { return r.err }
