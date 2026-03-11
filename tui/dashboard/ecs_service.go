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

type serviceDrillTarget int

const (
	drillToTasks serviceDrillTarget = iota
	drillToStatus
	drillToLogs
)

type ECSServiceResource struct {
	cluster     string
	services    []bargeaws.ServiceInfo
	err         error
	drillTarget serviceDrillTarget
	client      *bargeaws.Client
}

func NewECSServiceResource(cluster string) *ECSServiceResource {
	return &ECSServiceResource{cluster: cluster}
}

func NewECSServiceResourceForStatus(cluster string) *ECSServiceResource {
	return &ECSServiceResource{cluster: cluster, drillTarget: drillToStatus}
}

func NewECSServiceResourceForLogs(cluster string) *ECSServiceResource {
	return &ECSServiceResource{cluster: cluster, drillTarget: drillToLogs}
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
	r.client = client
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

func (r *ECSServiceResource) Actions(row table.Row) []Action {
	if r.client == nil {
		return nil
	}
	serviceName := row[0]
	cluster := r.cluster
	client := r.client
	return []Action{
		{
			Name: "Scale Service",
			Run: func() tea.Cmd {
				return func() tea.Msg {
					return inputRequestMsg{
						prompt: fmt.Sprintf("Scale %s — enter desired count:", serviceName),
						callback: func(value string) tea.Cmd {
							var count int32
							if _, err := fmt.Sscanf(value, "%d", &count); err != nil {
								return func() tea.Msg {
									return apiResultMsg{"", fmt.Errorf("invalid number: %s", value)}
								}
							}
							return func() tea.Msg {
								err := client.UpdateServiceDesiredCount(context.Background(), cluster, serviceName, count)
								if err != nil {
									return apiResultMsg{"", err}
								}
								return apiResultMsg{fmt.Sprintf("Scaled %s to %d", serviceName, count), nil}
							}
						},
					}
				}
			},
		},
	}
}

func (r *ECSServiceResource) Error() error { return r.err }

func (r *ECSServiceResource) ChildResource(row table.Row) (string, Resource) {
	switch r.drillTarget {
	case drillToStatus:
		return row[0], NewECSStatusResource(r.cluster, row[0])
	case drillToLogs:
		return row[0], NewECSLogsResource(r.cluster, row[0])
	default:
		return row[0], NewECSTaskResource(r.cluster, row[0])
	}
}

func (r *ECSServiceResource) SecondaryChildResource(row table.Row) (string, Resource) {
	return row[0], NewECSEventsResource(r.cluster, row[0])
}
