package tui

import (
	"fmt"

	bargeaws "github.com/MutuallyAssuredDeployment/barge/aws"
)

// ClusterItem implements list.Item for ECS clusters.
type ClusterItem struct {
	Name string
}

func (i ClusterItem) Title() string       { return i.Name }
func (i ClusterItem) Description() string { return "" }
func (i ClusterItem) FilterValue() string { return i.Name }

// ServiceItem implements list.Item for ECS services.
type ServiceItem struct {
	Info bargeaws.ServiceInfo
}

func (i ServiceItem) Title() string { return i.Info.Name }
func (i ServiceItem) Description() string {
	return fmt.Sprintf("%d running | %s", i.Info.RunningCount, i.Info.TaskDefName)
}
func (i ServiceItem) FilterValue() string { return i.Info.Name }

// TaskItem implements list.Item for ECS tasks.
type TaskItem struct {
	Info bargeaws.TaskInfo
}

func (i TaskItem) Title() string { return i.Info.ID }
func (i TaskItem) Description() string {
	started := "pending"
	if !i.Info.StartedAt.IsZero() {
		started = i.Info.StartedAt.Local().Format("15:04:05")
	}
	return fmt.Sprintf("%s | started %s | rev %s", i.Info.Status, started, i.Info.Revision)
}
func (i TaskItem) FilterValue() string { return i.Info.ID }

// ContainerItem implements list.Item for ECS containers.
type ContainerItem struct {
	Info bargeaws.ContainerInfo
}

func (i ContainerItem) Title() string { return i.Info.Name }
func (i ContainerItem) Description() string {
	desc := fmt.Sprintf("%s | %s", i.Info.Image, i.Info.Status)
	if i.Info.Essential {
		desc += " [essential]"
	}
	return desc
}
func (i ContainerItem) FilterValue() string { return i.Info.Name }
