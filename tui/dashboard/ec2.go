// tui/dashboard/ec2.go
package dashboard

import (
	"context"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ec2FetchMsg struct {
	instances []bargeaws.InstanceInfo
	err       error
}

// EC2InstanceResource provides the EC2 instances view.
type EC2InstanceResource struct {
	instances   []bargeaws.InstanceInfo
	err         error
	instanceIDs []string // optional: only show these instances (e.g. ASG members)
}

func NewEC2InstanceResource() *EC2InstanceResource {
	return &EC2InstanceResource{}
}

func NewEC2InstanceResourceForIDs(ids []string) *EC2InstanceResource {
	return &EC2InstanceResource{instanceIDs: ids}
}

func (r *EC2InstanceResource) Name() string { return "EC2 Instances" }

func (r *EC2InstanceResource) Columns() []Column {
	return []Column{
		{"INSTANCE", 20},
		{"NAME", 24},
		{"PLATFORM", 10},
		{"PRIVATE IP", 16},
		{"PUBLIC IP", 16},
	}
}

func (r *EC2InstanceResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		instances, err := client.ListInstances(context.Background())
		if err == nil && len(r.instanceIDs) > 0 {
			idSet := make(map[string]bool, len(r.instanceIDs))
			for _, id := range r.instanceIDs {
				idSet[id] = true
			}
			var filtered []bargeaws.InstanceInfo
			for _, inst := range instances {
				if idSet[inst.ID] {
					filtered = append(filtered, inst)
				}
			}
			instances = filtered
		}
		return ec2FetchMsg{instances, err}
	}
}

func (r *EC2InstanceResource) HandleMsg(msg tea.Msg) bool {
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

func (r *EC2InstanceResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.instances))
	for i, inst := range r.instances {
		rows[i] = table.Row{inst.ID, inst.Name, inst.Platform, inst.PrivateIP, inst.PublicIP}
	}
	return rows
}

func (r *EC2InstanceResource) Actions(row table.Row) []Action {
	if len(row) == 0 {
		return nil
	}
	instanceID := row[0]
	name := row[1]
	privateIP := row[3]
	publicIP := row[4]

	// Build title: "EC2: name (i-xxx) │ 10.0.1.5 │ 54.23.45.67"
	title := "EC2: "
	if name != "" {
		title += name + " (" + instanceID + ")"
	} else {
		title += instanceID
	}
	if privateIP != "" {
		title += " │ " + privateIP
	}
	if publicIP != "" {
		title += " │ " + publicIP
	}

	return []Action{SSMShellAction(instanceID, title)}
}

func (r *EC2InstanceResource) Error() error {
	return r.err
}
