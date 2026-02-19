package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type Client struct {
	ecs *ecs.Client
}

type ServiceInfo struct {
	Name         string
	RunningCount int32
	TaskDefName  string
}

type TaskInfo struct {
	ID        string
	Arn       string
	Status    string
	StartedAt time.Time
	TaskDef   string
	Revision  string
	Containers []ContainerInfo
}

type ContainerInfo struct {
	Name      string
	Image     string
	Status    string
	Essential bool
}

func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	var opts []func(*config.LoadOptions) error

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	return &Client{ecs: ecs.NewFromConfig(cfg)}, nil
}

func (c *Client) ListClusters(ctx context.Context) ([]string, error) {
	var arns []string
	paginator := ecs.NewListClustersPaginator(c.ecs, &ecs.ListClustersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing clusters: %w", err)
		}
		arns = append(arns, page.ClusterArns...)
	}

	names := make([]string, len(arns))
	for i, arn := range arns {
		names[i] = shortName(arn)
	}
	sort.Strings(names)
	return names, nil
}

func (c *Client) ListServices(ctx context.Context, cluster string) ([]ServiceInfo, error) {
	var arns []string
	paginator := ecs.NewListServicesPaginator(c.ecs, &ecs.ListServicesInput{
		Cluster: aws.String(cluster),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing services: %w", err)
		}
		arns = append(arns, page.ServiceArns...)
	}

	if len(arns) == 0 {
		return nil, nil
	}

	// DescribeServices accepts max 10 at a time
	var services []ServiceInfo
	for i := 0; i < len(arns); i += 10 {
		end := i + 10
		if end > len(arns) {
			end = len(arns)
		}
		out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(cluster),
			Services: arns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing services: %w", err)
		}
		for _, svc := range out.Services {
			services = append(services, ServiceInfo{
				Name:         aws.ToString(svc.ServiceName),
				RunningCount: svc.RunningCount,
				TaskDefName:  shortName(aws.ToString(svc.TaskDefinition)),
			})
		}
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})
	return services, nil
}

func (c *Client) ListTasks(ctx context.Context, cluster, service string) ([]TaskInfo, error) {
	var allArns []string
	paginator := ecs.NewListTasksPaginator(c.ecs, &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing tasks: %w", err)
		}
		allArns = append(allArns, page.TaskArns...)
	}

	if len(allArns) == 0 {
		return nil, nil
	}

	// DescribeTasks accepts max 100 at a time
	var described []ecstypes.Task
	for i := 0; i < len(allArns); i += 100 {
		end := i + 100
		if end > len(allArns) {
			end = len(allArns)
		}
		desc, err := c.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(cluster),
			Tasks:   allArns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing tasks: %w", err)
		}
		described = append(described, desc.Tasks...)
	}

	// Fetch task definitions to get essential flag
	taskDefContainers := make(map[string]map[string]bool) // taskDefArn -> containerName -> essential
	for _, task := range described {
		arn := aws.ToString(task.TaskDefinitionArn)
		if _, ok := taskDefContainers[arn]; !ok {
			defs, err := c.describeTaskDef(ctx, arn)
			if err != nil {
				return nil, err
			}
			m := make(map[string]bool)
			for _, d := range defs {
				m[d.name] = d.essential
			}
			taskDefContainers[arn] = m
		}
	}

	var tasks []TaskInfo
	for _, task := range described {
		taskArn := aws.ToString(task.TaskArn)
		taskDefArn := aws.ToString(task.TaskDefinitionArn)
		essentialMap := taskDefContainers[taskDefArn]

		var containers []ContainerInfo
		for _, c := range task.Containers {
			name := aws.ToString(c.Name)
			containers = append(containers, ContainerInfo{
				Name:      name,
				Image:     aws.ToString(c.Image),
				Status:    aws.ToString(c.LastStatus),
				Essential: essentialMap[name],
			})
		}

		var startedAt time.Time
		if task.StartedAt != nil {
			startedAt = *task.StartedAt
		}

		revision := ""
		if parts := strings.Split(shortName(taskDefArn), ":"); len(parts) == 2 {
			revision = parts[1]
		}

		tasks = append(tasks, TaskInfo{
			ID:         shortName(taskArn),
			Arn:        taskArn,
			Status:     aws.ToString(task.LastStatus),
			StartedAt:  startedAt,
			TaskDef:    shortName(taskDefArn),
			Revision:   revision,
			Containers: containers,
		})
	}

	// Sort by start time, most recent first
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartedAt.After(tasks[j].StartedAt)
	})
	return tasks, nil
}

func (c *Client) describeTaskDef(ctx context.Context, taskDefArn string) ([]containerDef, error) {
	out, err := c.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefArn),
	})
	if err != nil {
		return nil, fmt.Errorf("describing task definition: %w", err)
	}

	var defs []containerDef
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		defs = append(defs, containerDef{
			name:      aws.ToString(cd.Name),
			essential: cd.Essential != nil && *cd.Essential,
		})
	}
	return defs, nil
}

type containerDef struct {
	name      string
	essential bool
}

// MostRecentTask returns the most recently started task, or an error if no tasks are provided.
func MostRecentTask(tasks []TaskInfo) (TaskInfo, error) {
	if len(tasks) == 0 {
		return TaskInfo{}, fmt.Errorf("no running tasks found")
	}
	// Already sorted most recent first by ListTasks
	return tasks[0], nil
}

// ResolveContainer picks the right container given a name hint. If hint is empty,
// it auto-selects if there's only one container or only one essential container.
func ResolveContainer(containers []ContainerInfo, hint string) (ContainerInfo, error) {
	if len(containers) == 0 {
		return ContainerInfo{}, fmt.Errorf("no containers found in task")
	}

	if hint != "" {
		for _, c := range containers {
			if c.Name == hint {
				return c, nil
			}
		}
		names := make([]string, len(containers))
		for i, c := range containers {
			names[i] = c.Name
		}
		return ContainerInfo{}, fmt.Errorf("container %q not found; available: %s", hint, strings.Join(names, ", "))
	}

	if len(containers) == 1 {
		return containers[0], nil
	}

	var essential []ContainerInfo
	for _, c := range containers {
		if c.Essential {
			essential = append(essential, c)
		}
	}
	if len(essential) == 1 {
		return essential[0], nil
	}

	names := make([]string, len(containers))
	for i, c := range containers {
		names[i] = c.Name
	}
	return ContainerInfo{}, fmt.Errorf("cannot auto-select container; specify with --container: %s", strings.Join(names, ", "))
}

func shortName(arn string) string {
	parts := strings.Split(arn, "/")
	return parts[len(parts)-1]
}

// FindTask finds a task by ID in a slice, or returns an error.
func FindTask(tasks []TaskInfo, id string) (TaskInfo, error) {
	for _, t := range tasks {
		if t.ID == id {
			return t, nil
		}
	}
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return TaskInfo{}, fmt.Errorf("task %q not found; available: %s", id, strings.Join(ids, ", "))
}

