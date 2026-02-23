package dashboard

import (
	bargeaws "github.com/janost/barge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type pickerReadyMsg struct{}

// ResourcePicker is the root resource showing available resource types.
type ResourcePicker struct{}

func NewResourcePicker() *ResourcePicker {
	return &ResourcePicker{}
}

func (r *ResourcePicker) Name() string { return "Resources" }

func (r *ResourcePicker) Columns() []Column {
	return []Column{
		{"TYPE", 30},
		{"DESCRIPTION", 50},
	}
}

func (r *ResourcePicker) FetchCmd(client *bargeaws.Client) tea.Cmd {
	return func() tea.Msg { return pickerReadyMsg{} }
}

func (r *ResourcePicker) HandleMsg(msg tea.Msg) bool {
	_, ok := msg.(pickerReadyMsg)
	return ok
}

func (r *ResourcePicker) Rows() []table.Row {
	return []table.Row{
		{"EC2 Instances", "Managed instances with SSM agent"},
		{"Auto Scaling Groups", "EC2 Auto Scaling Groups"},
		{"ECS Clusters", "ECS clusters, services, and tasks"},
		{"ECS Service Status", "Deployments, tasks, and recent stops"},
		{"ECS Service Logs", "Recent CloudWatch logs for a service"},
		{"ECS Task Definitions", "Task definition families"},
		{"RDS Clusters", "RDS/Aurora database clusters"},
		{"CloudFormation Stacks", "CloudFormation stack resources and events"},
		{"S3 Buckets", "S3 bucket browser"},
	}
}

func (r *ResourcePicker) Actions(row table.Row) []Action { return nil }

func (r *ResourcePicker) Error() error { return nil }

func (r *ResourcePicker) ChildResource(row table.Row) (string, Resource) {
	switch row[0] {
	case "Auto Scaling Groups":
		return "Auto Scaling Groups", NewASGResource()
	case "ECS Clusters":
		return "ECS Clusters", NewECSClusterResource()
	case "ECS Service Status":
		return "ECS Service Status", NewECSClusterResourceForStatus()
	case "ECS Service Logs":
		return "ECS Service Logs", NewECSClusterResourceForLogs()
	case "ECS Task Definitions":
		return "ECS Task Definitions", NewECSTaskDefResource()
	case "RDS Clusters":
		return "RDS Clusters", NewRDSClusterResource()
	case "CloudFormation Stacks":
		return "CloudFormation Stacks", NewCFNStackResource()
	case "S3 Buckets":
		return "S3 Buckets", NewS3BucketResource()
	default:
		return "EC2 Instances", NewEC2InstanceResource()
	}
}
