// tui/dashboard/ec2.go
package dashboard

import (
	"context"
	osexec "os/exec"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ec2FetchMsg struct {
	instances []bargeaws.InstanceInfo
	err       error
}

// processExitMsg is sent when a subprocess (e.g. SSM shell) completes.
type processExitMsg struct{ err error }

// EC2Resource provides the EC2 instances view.
type EC2Resource struct {
	instances []bargeaws.InstanceInfo
	err       error
}

func NewEC2Resource() *EC2Resource {
	return &EC2Resource{}
}

func (r *EC2Resource) Name() string { return "EC2 Instances" }

func (r *EC2Resource) Columns() []Column {
	return []Column{
		{"INSTANCE", 22},
		{"NAME", 28},
		{"PLATFORM", 10},
		{"IP", 16},
	}
}

func (r *EC2Resource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		instances, err := client.ListInstances(context.Background())
		return ec2FetchMsg{instances, err}
	}
}

func (r *EC2Resource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ec2FetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.instances = m.instances
		}
		// On error, existing instances are preserved so stale data remains visible.
		return true
	}
	return false
}

func (r *EC2Resource) Rows() []table.Row {
	rows := make([]table.Row, len(r.instances))
	for i, inst := range r.instances {
		rows[i] = table.Row{inst.ID, inst.Name, inst.Platform, inst.IPAddress}
	}
	return rows
}

func (r *EC2Resource) Actions(row table.Row) []Action {
	if len(row) == 0 {
		return nil
	}
	instanceID := row[0]
	return []Action{
		{
			Name: "SSM Shell",
			Run: func() tea.Cmd {
				c := osexec.Command("aws", "ssm", "start-session", "--target", instanceID)
				return tea.ExecProcess(c, func(err error) tea.Msg {
					return processExitMsg{err}
				})
			},
		},
	}
}

func (r *EC2Resource) Error() error {
	return r.err
}
