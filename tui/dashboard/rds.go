package dashboard

import (
	"context"
	"fmt"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// --- RDS Clusters ---

type rdsClusterFetchMsg struct {
	clusters []bridgeaws.RDSClusterInfo
	err      error
}

type RDSClusterResource struct {
	clusters []bridgeaws.RDSClusterInfo
	err      error
}

func NewRDSClusterResource() *RDSClusterResource {
	return &RDSClusterResource{}
}

func (r *RDSClusterResource) Name() string { return "RDS Clusters" }

func (r *RDSClusterResource) Columns() []Column {
	return []Column{
		{"CLUSTER", 25},
		{"ENGINE", 15},
		{"VERSION", 10},
		{"STATUS", 12},
		{"MEMBERS", 8},
		{"ENDPOINT", 40},
	}
}

func (r *RDSClusterResource) FetchCmd(client *bridgeaws.Client) tea.Cmd {
	return func() tea.Msg {
		clusters, err := client.ListRDSClusters(context.Background())
		return rdsClusterFetchMsg{clusters, err}
	}
}

func (r *RDSClusterResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(rdsClusterFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.clusters = m.clusters
		}
		return true
	}
	return false
}

func (r *RDSClusterResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.clusters))
	for i, c := range r.clusters {
		rows[i] = table.Row{
			c.ID,
			c.Engine,
			c.EngineVersion,
			c.Status,
			fmt.Sprintf("%d", c.Members),
			c.Endpoint,
		}
	}
	return rows
}

func (r *RDSClusterResource) Actions(row table.Row) []Action { return nil }
func (r *RDSClusterResource) Error() error                   { return r.err }

// ChildResource drills into the cluster's member instances.
func (r *RDSClusterResource) ChildResource(row table.Row) (string, Resource) {
	return row[0], NewRDSInstanceResource(row[0])
}

// --- RDS Instances ---

type rdsInstanceFetchMsg struct {
	instances []bridgeaws.RDSInstanceInfo
	err       error
}

type RDSInstanceResource struct {
	clusterFilter string // empty = all instances
	instances     []bridgeaws.RDSInstanceInfo
	err           error
}

func NewRDSInstanceResource(clusterFilter string) *RDSInstanceResource {
	return &RDSInstanceResource{clusterFilter: clusterFilter}
}

func (r *RDSInstanceResource) Name() string {
	if r.clusterFilter != "" {
		return "Instances"
	}
	return "RDS Instances"
}

func (r *RDSInstanceResource) Columns() []Column {
	return []Column{
		{"INSTANCE", 25},
		{"ENGINE", 15},
		{"CLASS", 15},
		{"STATUS", 12},
		{"STORAGE", 8},
		{"ENDPOINT", 40},
	}
}

func (r *RDSInstanceResource) FetchCmd(client *bridgeaws.Client) tea.Cmd {
	return func() tea.Msg {
		instances, err := client.ListRDSInstances(context.Background())
		if err != nil {
			return rdsInstanceFetchMsg{nil, err}
		}
		if r.clusterFilter != "" {
			var filtered []bridgeaws.RDSInstanceInfo
			for _, inst := range instances {
				if inst.ClusterID == r.clusterFilter {
					filtered = append(filtered, inst)
				}
			}
			instances = filtered
		}
		return rdsInstanceFetchMsg{instances, nil}
	}
}

func (r *RDSInstanceResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(rdsInstanceFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.instances = m.instances
		}
		return true
	}
	return false
}

func (r *RDSInstanceResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.instances))
	for i, inst := range r.instances {
		multiAZ := ""
		if inst.MultiAZ {
			multiAZ = " (multi-az)"
		}
		rows[i] = table.Row{
			inst.ID,
			inst.Engine,
			inst.Class,
			inst.Status + multiAZ,
			fmt.Sprintf("%dGB", inst.StorageGB),
			inst.Endpoint,
		}
	}
	return rows
}

func (r *RDSInstanceResource) Actions(row table.Row) []Action { return nil }
func (r *RDSInstanceResource) Error() error                   { return r.err }
