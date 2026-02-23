# RDS/Aurora Resource

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add RDS/Aurora database instances and clusters as browsable resources in the TUI dashboard.

**Architecture:** New AWS SDK dependency (`rds`). New `aws/rds.go` with client methods. Two new resources: `RDSInstanceResource` (standalone instances) and `RDSClusterResource` (Aurora clusters that drill into member instances). Registered in the picker.

**Tech Stack:** Go, AWS RDS SDK, Bubble Tea

---

### Task 1: Add RDS SDK dependency and AWS client methods

**Files:**
- Run: `go get github.com/aws/aws-sdk-go-v2/service/rds`
- Modify: `aws/ecs.go` — add `rds` field to Client struct and NewClient
- Create: `aws/rds.go`

**Step 1: Add the dependency**

Run: `go get github.com/aws/aws-sdk-go-v2/service/rds`

**Step 2: Add rds client to Client struct**

In `aws/ecs.go`, add to the Client struct:
```go
rds *rds.Client
```

Add import: `"github.com/aws/aws-sdk-go-v2/service/rds"`

In `NewClient()`, add:
```go
rds: rds.NewFromConfig(cfg),
```

**Step 3: Create aws/rds.go**

```go
package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type RDSInstanceInfo struct {
	ID             string
	Engine         string
	EngineVersion  string
	Class          string
	Status         string
	Endpoint       string
	MultiAZ        bool
	StorageGB      int32
	ClusterID      string // non-empty if part of an Aurora cluster
}

type RDSClusterInfo struct {
	ID            string
	Engine        string
	EngineVersion string
	Status        string
	Endpoint      string
	ReaderEndpoint string
	Members       int
	MultiAZ       bool
}

func (c *Client) ListRDSInstances(ctx context.Context) ([]RDSInstanceInfo, error) {
	var instances []RDSInstanceInfo
	p := rds.NewDescribeDBInstancesPaginator(c.rds, &rds.DescribeDBInstancesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing RDS instances: %w", err)
		}
		for _, db := range page.DBInstances {
			endpoint := ""
			if db.Endpoint != nil {
				endpoint = fmt.Sprintf("%s:%d", awssdk.ToString(db.Endpoint.Address), db.Endpoint.Port)
			}
			clusterID := ""
			if db.DBClusterIdentifier != nil {
				clusterID = awssdk.ToString(db.DBClusterIdentifier)
			}
			instances = append(instances, RDSInstanceInfo{
				ID:            awssdk.ToString(db.DBInstanceIdentifier),
				Engine:        awssdk.ToString(db.Engine),
				EngineVersion: awssdk.ToString(db.EngineVersion),
				Class:         awssdk.ToString(db.DBInstanceClass),
				Status:        awssdk.ToString(db.DBInstanceStatus),
				Endpoint:      endpoint,
				MultiAZ:       db.MultiAZ != nil && *db.MultiAZ,
				StorageGB:     db.AllocatedStorage,
				ClusterID:     clusterID,
			})
		}
	}
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].ID < instances[j].ID
	})
	return instances, nil
}

func (c *Client) ListRDSClusters(ctx context.Context) ([]RDSClusterInfo, error) {
	var clusters []RDSClusterInfo
	p := rds.NewDescribeDBClustersPaginator(c.rds, &rds.DescribeDBClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing RDS clusters: %w", err)
		}
		for _, cl := range page.DBClusters {
			clusters = append(clusters, RDSClusterInfo{
				ID:             awssdk.ToString(cl.DBClusterIdentifier),
				Engine:         awssdk.ToString(cl.Engine),
				EngineVersion:  awssdk.ToString(cl.EngineVersion),
				Status:         awssdk.ToString(cl.Status),
				Endpoint:       awssdk.ToString(cl.Endpoint),
				ReaderEndpoint: awssdk.ToString(cl.ReaderEndpoint),
				Members:        len(cl.DBClusterMembers),
				MultiAZ:        cl.MultiAZ != nil && *cl.MultiAZ,
			})
		}
	}
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ID < clusters[j].ID
	})
	return clusters, nil
}
```

**Step 4: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add go.mod go.sum aws/ecs.go aws/rds.go
git commit -m "feat: add RDS/Aurora AWS client methods"
```

---

### Task 2: Create RDS dashboard resources

**Files:**
- Create: `tui/dashboard/rds.go`

**Step 1: Create the resource file with both cluster and instance resources**

```go
package dashboard

import (
	"context"
	"fmt"

	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// --- RDS Clusters ---

type rdsClusterFetchMsg struct {
	clusters []bargeaws.RDSClusterInfo
	err      error
}

type RDSClusterResource struct {
	clusters []bargeaws.RDSClusterInfo
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

func (r *RDSClusterResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
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

func (r *RDSClusterResource) ChildResource(row table.Row) (string, Resource) {
	return row[0], NewRDSInstanceResource(row[0])
}

// --- RDS Instances ---

type rdsInstanceFetchMsg struct {
	instances []bargeaws.RDSInstanceInfo
	err       error
}

type RDSInstanceResource struct {
	clusterFilter string // empty = all instances
	instances     []bargeaws.RDSInstanceInfo
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

func (r *RDSInstanceResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg {
		instances, err := client.ListRDSInstances(context.Background())
		if err != nil {
			return rdsInstanceFetchMsg{nil, err}
		}
		if r.clusterFilter != "" {
			var filtered []bargeaws.RDSInstanceInfo
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
```

**Step 2: Register in picker**

Add to `picker.go` Rows():
```go
{"RDS Clusters", "Aurora clusters and member instances"},
{"RDS Instances", "All RDS database instances"},
```

Add to ChildResource():
```go
case "RDS Clusters":
    return "RDS Clusters", NewRDSClusterResource()
case "RDS Instances":
    return "RDS Instances", NewRDSInstanceResource("")
```

**Step 3: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 4: Verify vet**

Run: `GOCACHE=/tmp go vet ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/dashboard/rds.go tui/dashboard/picker.go
git commit -m "feat: add RDS Clusters and Instances resources to dashboard"
```
