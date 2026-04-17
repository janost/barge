package dashboard

import (
	"context"
	"fmt"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsTaskDefFetchMsg struct {
	taskDefs []bridgeaws.TaskDefInfo
	err      error
}

type ECSTaskDefResource struct {
	taskDefs []bridgeaws.TaskDefInfo
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

func (r *ECSTaskDefResource) FetchCmd(client *bridgeaws.Client) tea.Cmd {
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

func (r *ECSTaskDefResource) ChildResource(row table.Row) (string, Resource) {
	family := row[0]
	revision := 0
	fmt.Sscanf(row[1], "%d", &revision)
	return family, NewECSTaskDefDetailResource(family, revision)
}

func (r *ECSTaskDefResource) Actions(row table.Row) []Action { return nil }

func (r *ECSTaskDefResource) Error() error { return r.err }
