package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Client struct {
	ecs *ecs.Client
	ssm *ssm.Client
	ec2 *ec2.Client
	cwl *cloudwatchlogs.Client
	asg *autoscaling.Client
	rds *rds.Client
	cfn *cloudformation.Client
	s3c *s3.Client
}

type ServiceInfo struct {
	Name         string
	Status       string
	DesiredCount int32
	RunningCount int32
	PendingCount int32
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

type ClusterInfo struct {
	Name           string
	Status         string
	ActiveServices int32
	RunningTasks   int32
	PendingTasks   int32
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

	return &Client{
		ecs: ecs.NewFromConfig(cfg),
		ssm: ssm.NewFromConfig(cfg),
		ec2: ec2.NewFromConfig(cfg),
		cwl: cloudwatchlogs.NewFromConfig(cfg),
		asg: autoscaling.NewFromConfig(cfg),
		rds: rds.NewFromConfig(cfg),
		cfn: cloudformation.NewFromConfig(cfg),
		s3c: s3.NewFromConfig(cfg),
	}, nil
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

func (c *Client) ListClustersDetail(ctx context.Context) ([]ClusterInfo, error) {
	var clusterArns []string
	p := ecs.NewListClustersPaginator(c.ecs, &ecs.ListClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing clusters: %w", err)
		}
		clusterArns = append(clusterArns, page.ClusterArns...)
	}
	if len(clusterArns) == 0 {
		return nil, nil
	}

	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: clusterArns,
	})
	if err != nil {
		return nil, fmt.Errorf("describing clusters: %w", err)
	}

	infos := make([]ClusterInfo, 0, len(out.Clusters))
	for _, cl := range out.Clusters {
		infos = append(infos, ClusterInfo{
			Name:           aws.ToString(cl.ClusterName),
			Status:         aws.ToString(cl.Status),
			ActiveServices: cl.ActiveServicesCount,
			RunningTasks:   cl.RunningTasksCount,
			PendingTasks:   cl.PendingTasksCount,
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos, nil
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
				Status:       aws.ToString(svc.Status),
				DesiredCount: svc.DesiredCount,
				RunningCount: svc.RunningCount,
				PendingCount: svc.PendingCount,
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
	input := &ecs.ListTasksInput{
		Cluster: aws.String(cluster),
	}
	if service != "" {
		input.ServiceName = aws.String(service)
	}
	var allArns []string
	paginator := ecs.NewListTasksPaginator(c.ecs, input)
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

type ServiceEvent struct {
	Timestamp time.Time
	Message   string
}

type ServiceDetail struct {
	Name         string
	Status       string
	DesiredCount int32
	RunningCount int32
	PendingCount int32
	Events       []ServiceEvent
	Deployments  []DeploymentInfo
}

type DeploymentInfo struct {
	Status       string
	TaskDef      string
	DesiredCount int32
	RunningCount int32
	PendingCount int32
	UpdatedAt    time.Time
}

func (c *Client) DescribeServiceDetail(ctx context.Context, cluster, service string) (ServiceDetail, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{service},
	})
	if err != nil {
		return ServiceDetail{}, fmt.Errorf("describing service: %w", err)
	}
	if len(out.Services) == 0 {
		return ServiceDetail{}, fmt.Errorf("service %q not found", service)
	}

	svc := out.Services[0]

	var events []ServiceEvent
	for _, e := range svc.Events {
		var ts time.Time
		if e.CreatedAt != nil {
			ts = *e.CreatedAt
		}
		events = append(events, ServiceEvent{
			Timestamp: ts,
			Message:   aws.ToString(e.Message),
		})
	}

	var deployments []DeploymentInfo
	for _, d := range svc.Deployments {
		var updatedAt time.Time
		if d.UpdatedAt != nil {
			updatedAt = *d.UpdatedAt
		}
		deployments = append(deployments, DeploymentInfo{
			Status:       aws.ToString(d.Status),
			TaskDef:      shortName(aws.ToString(d.TaskDefinition)),
			DesiredCount: d.DesiredCount,
			RunningCount: d.RunningCount,
			PendingCount: d.PendingCount,
			UpdatedAt:    updatedAt,
		})
	}

	return ServiceDetail{
		Name:         aws.ToString(svc.ServiceName),
		Status:       aws.ToString(svc.Status),
		DesiredCount: svc.DesiredCount,
		RunningCount: svc.RunningCount,
		PendingCount: svc.PendingCount,
		Events:       events,
		Deployments:  deployments,
	}, nil
}

type StoppedTaskInfo struct {
	ID         string
	StopReason string
	StoppedAt  time.Time
}

func (c *Client) ListStoppedTasks(ctx context.Context, cluster, service string) ([]StoppedTaskInfo, error) {
	out, err := c.ecs.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       aws.String(cluster),
		ServiceName:   aws.String(service),
		DesiredStatus: ecstypes.DesiredStatusStopped,
		MaxResults:    aws.Int32(10),
	})
	if err != nil {
		return nil, err
	}
	if len(out.TaskArns) == 0 {
		return nil, nil
	}

	desc, err := c.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   out.TaskArns,
	})
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	var stopped []StoppedTaskInfo
	for _, t := range desc.Tasks {
		var stoppedAt time.Time
		if t.StoppedAt != nil {
			stoppedAt = *t.StoppedAt
		}
		if stoppedAt.Before(cutoff) {
			continue
		}
		stopped = append(stopped, StoppedTaskInfo{
			ID:         shortName(aws.ToString(t.TaskArn)),
			StopReason: aws.ToString(t.StoppedReason),
			StoppedAt:  stoppedAt,
		})
	}
	return stopped, nil
}

func shortName(arn string) string {
	parts := strings.Split(arn, "/")
	return parts[len(parts)-1]
}

type TaskDefDetail struct {
	Family      string
	Revision    int
	CPU         string
	Memory      string
	NetworkMode string
	Containers  []TaskDefContainerDetail
}

type TaskDefContainerDetail struct {
	Name      string
	Image     string
	CPU       int32
	Memory    int32
	Essential bool
	PortMaps  string // "80:8080, 443:8443"
	EnvCount  int    // number of env vars
}

type TaskDefInfo struct {
	Family   string
	Revision int
	Arn      string
}

func (c *Client) ListTaskDefinitions(ctx context.Context) ([]TaskDefInfo, error) {
	p := ecs.NewListTaskDefinitionsPaginator(c.ecs, &ecs.ListTaskDefinitionsInput{
		Status: ecstypes.TaskDefinitionStatusActive,
	})
	families := make(map[string]TaskDefInfo)
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing task definitions: %w", err)
		}
		for _, arn := range page.TaskDefinitionArns {
			name := shortName(arn)
			parts := strings.SplitN(name, ":", 2)
			family := parts[0]
			rev := 0
			if len(parts) == 2 {
				fmt.Sscanf(parts[1], "%d", &rev)
			}
			if existing, ok := families[family]; !ok || rev > existing.Revision {
				families[family] = TaskDefInfo{Family: family, Revision: rev, Arn: arn}
			}
		}
	}

	infos := make([]TaskDefInfo, 0, len(families))
	for _, info := range families {
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Family < infos[j].Family
	})
	return infos, nil
}

func (c *Client) DescribeTaskDefinitionDetail(ctx context.Context, family string, revision int) (TaskDefDetail, error) {
	taskDef := fmt.Sprintf("%s:%d", family, revision)
	out, err := c.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDef),
	})
	if err != nil {
		return TaskDefDetail{}, fmt.Errorf("describing task definition: %w", err)
	}

	td := out.TaskDefinition
	var containers []TaskDefContainerDetail
	for _, cd := range td.ContainerDefinitions {
		var ports []string
		for _, pm := range cd.PortMappings {
			hp := int32(0)
			if pm.HostPort != nil {
				hp = *pm.HostPort
			}
			cp := int32(0)
			if pm.ContainerPort != nil {
				cp = *pm.ContainerPort
			}
			ports = append(ports, fmt.Sprintf("%d:%d", hp, cp))
		}
		containers = append(containers, TaskDefContainerDetail{
			Name:      aws.ToString(cd.Name),
			Image:     aws.ToString(cd.Image),
			CPU:       cd.Cpu,
			Memory:    aws.ToInt32(cd.Memory),
			Essential: cd.Essential != nil && *cd.Essential,
			PortMaps:  strings.Join(ports, ", "),
			EnvCount:  len(cd.Environment),
		})
	}

	return TaskDefDetail{
		Family:      aws.ToString(td.Family),
		Revision:    int(td.Revision),
		CPU:         aws.ToString(td.Cpu),
		Memory:      aws.ToString(td.Memory),
		NetworkMode: string(td.NetworkMode),
		Containers:  containers,
	}, nil
}

// StopTask stops a running ECS task.
func (c *Client) StopTask(ctx context.Context, cluster, taskID, reason string) error {
	_, err := c.ecs.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: aws.String(cluster),
		Task:    aws.String(taskID),
		Reason:  aws.String(reason),
	})
	if err != nil {
		return fmt.Errorf("stopping task: %w", err)
	}
	return nil
}

// ForceNewDeployment triggers a new deployment of an ECS service.
func (c *Client) ForceNewDeployment(ctx context.Context, cluster, service string) error {
	_, err := c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            aws.String(cluster),
		Service:            aws.String(service),
		ForceNewDeployment: true,
	})
	if err != nil {
		return fmt.Errorf("forcing new deployment: %w", err)
	}
	return nil
}

// UpdateServiceDesiredCount changes the desired task count for an ECS service.
func (c *Client) UpdateServiceDesiredCount(ctx context.Context, cluster, service string, desiredCount int32) error {
	_, err := c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(cluster),
		Service:      aws.String(service),
		DesiredCount: aws.Int32(desiredCount),
	})
	if err != nil {
		return fmt.Errorf("updating desired count: %w", err)
	}
	return nil
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

