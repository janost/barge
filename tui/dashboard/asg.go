package dashboard

import (
	"context"
	"fmt"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type asgFetchMsg struct {
	asgs []bridgeaws.ASGInfo
	err  error
}

// ASGResource provides the Auto Scaling Groups view.
type ASGResource struct {
	asgs []bridgeaws.ASGInfo
	err  error
}

func NewASGResource() *ASGResource {
	return &ASGResource{}
}

func (r *ASGResource) Name() string { return "Auto Scaling Groups" }

func (r *ASGResource) Columns() []Column {
	return []Column{
		{"NAME", 30},
		{"MIN", 6},
		{"MAX", 6},
		{"DESIRED", 8},
		{"IN SERVICE", 10},
	}
}

func (r *ASGResource) FetchCmd(client *bridgeaws.Client) tea.Cmd {
	return func() tea.Msg {
		asgs, err := client.ListASGs(context.Background())
		return asgFetchMsg{asgs, err}
	}
}

func (r *ASGResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(asgFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.asgs = m.asgs
		}
		return true
	}
	return false
}

func (r *ASGResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.asgs))
	for i, asg := range r.asgs {
		rows[i] = table.Row{
			asg.Name,
			fmt.Sprintf("%d", asg.MinSize),
			fmt.Sprintf("%d", asg.MaxSize),
			fmt.Sprintf("%d", asg.Desired),
			fmt.Sprintf("%d", asg.InService),
		}
	}
	return rows
}

func (r *ASGResource) Actions(row table.Row) []Action { return nil }

func (r *ASGResource) Error() error { return r.err }

// ChildResource drills into an ASG showing its member EC2 instances.
func (r *ASGResource) ChildResource(row table.Row) (string, Resource) {
	asgName := row[0]
	for _, asg := range r.asgs {
		if asg.Name == asgName {
			return asgName, NewEC2InstanceResourceForIDs(asg.InstanceIDs)
		}
	}
	return asgName, NewEC2InstanceResource()
}
