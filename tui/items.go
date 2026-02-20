package tui

import (
	"fmt"
	"strings"

	bargeaws "github.com/janost/barge/aws"
)

// ModeItem implements list.Item for target type selection (ECS/EC2).
type ModeItem struct {
	Mode string // "ecs" or "ec2"
	Label string
	Desc  string
}

func (i ModeItem) Title() string       { return i.Label }
func (i ModeItem) Description() string { return i.Desc }
func (i ModeItem) FilterValue() string { return i.Label }

// InstanceItem implements list.Item for EC2 instances.
type InstanceItem struct {
	Info bargeaws.InstanceInfo
}

func (i InstanceItem) Title() string {
	if i.Info.Name != "" {
		return i.Info.Name
	}
	return i.Info.ID
}
func (i InstanceItem) Description() string {
	parts := []string{i.Info.ID}
	if i.Info.Platform != "" {
		parts = append(parts, i.Info.Platform)
	}
	if i.Info.PrivateIP != "" {
		parts = append(parts, i.Info.PrivateIP)
	}
	if i.Info.PublicIP != "" {
		parts = append(parts, i.Info.PublicIP)
	}
	return strings.Join(parts, " | ")
}
func (i InstanceItem) FilterValue() string {
	if i.Info.Name != "" {
		return i.Info.Name + " " + i.Info.ID
	}
	return i.Info.ID
}

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
