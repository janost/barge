package dashboard

import (
	"context"
	"fmt"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsServiceFetchMsg struct {
	services []bargeaws.ServiceInfo
	err      error
}

type ECSServiceResource struct {
	cluster  string
	services []bargeaws.ServiceInfo
	err      error
}

func NewECSServiceResource(cluster string) *ECSServiceResource {
	return &ECSServiceResource{cluster: cluster}
}

func (r *ECSServiceResource) Name() string { return "Services" }

func (r *ECSServiceResource) Columns() []Column {
	return []Column{
		{"NAME", 30},
		{"STATUS", 10},
		{"DESIRED", 8},
		{"RUNNING", 8},
		{"PENDING", 8},
		{"TASK DEF", 30},
	}
}

func (r *ECSServiceResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		services, err := client.ListServices(context.Background(), r.cluster)
		return ecsServiceFetchMsg{services, err}
	}
}

func (r *ECSServiceResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsServiceFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.services = m.services
		}
		return true
	}
	return false
}

func (r *ECSServiceResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.services))
	for i, svc := range r.services {
		rows[i] = table.Row{
			svc.Name,
			svc.Status,
			fmt.Sprintf("%d", svc.DesiredCount),
			fmt.Sprintf("%d", svc.RunningCount),
			fmt.Sprintf("%d", svc.PendingCount),
			svc.TaskDefName,
		}
	}
	return rows
}

func (r *ECSServiceResource) Actions(row table.Row) []Action { return nil }

func (r *ECSServiceResource) Error() error { return r.err }

func (r *ECSServiceResource) ChildResource(row table.Row) (string, Resource) {
	return row[0], NewECSTaskResource(r.cluster, row[0])
}
