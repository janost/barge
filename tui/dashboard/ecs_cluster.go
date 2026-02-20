package dashboard

import (
	"context"
	"fmt"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ecsClusterFetchMsg struct {
	clusters []bargeaws.ClusterInfo
	err      error
}

type ECSClusterResource struct {
	clusters []bargeaws.ClusterInfo
	err      error
}

func NewECSClusterResource() *ECSClusterResource {
	return &ECSClusterResource{}
}

func (r *ECSClusterResource) Name() string { return "ECS Clusters" }

func (r *ECSClusterResource) Columns() []Column {
	return []Column{
		{"NAME", 30},
		{"STATUS", 10},
		{"SERVICES", 10},
		{"RUNNING", 10},
		{"PENDING", 10},
	}
}

func (r *ECSClusterResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		clusters, err := client.ListClustersDetail(context.Background())
		return ecsClusterFetchMsg{clusters, err}
	}
}

func (r *ECSClusterResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(ecsClusterFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.clusters = m.clusters
		}
		return true
	}
	return false
}

func (r *ECSClusterResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.clusters))
	for i, cl := range r.clusters {
		rows[i] = table.Row{
			cl.Name,
			cl.Status,
			fmt.Sprintf("%d", cl.ActiveServices),
			fmt.Sprintf("%d", cl.RunningTasks),
			fmt.Sprintf("%d", cl.PendingTasks),
		}
	}
	return rows
}

func (r *ECSClusterResource) Actions(row table.Row) []Action { return nil }

func (r *ECSClusterResource) Error() error { return r.err }

func (r *ECSClusterResource) ChildResource(row table.Row) (string, Resource) {
	return row[0], NewECSServiceResource(row[0])
}

// SecondaryChildResource drills into all tasks on the cluster (including standalone tasks).
func (r *ECSClusterResource) SecondaryChildResource(row table.Row) (string, Resource) {
	return row[0], NewECSTaskResource(row[0], "")
}
