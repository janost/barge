package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/janost/bridge/config"
	"github.com/janost/bridge/exec"
	"github.com/janost/bridge/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Open a shell in a running ECS task or EC2 instance",
	Long:  "Connect to a running AWS ECS task or EC2 instance via SSM. Launches interactive drill-down if no flags are provided.",
	RunE:  runExec,
}

func init() {
	execCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "ECS cluster name (skips TUI if set with --service)")
	execCmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name (skips TUI if set with --cluster)")
	execCmd.Flags().StringVarP(&task, "task", "t", "", "ECS task ID (optional, defaults to most recent)")
	execCmd.Flags().StringVarP(&container, "container", "C", "", "Container name (optional, auto-selected if unambiguous)")
	execCmd.Flags().StringVarP(&command, "command", "x", "/bin/sh", "Command to execute in the container")
	execCmd.Flags().StringVarP(&instance, "instance", "i", "", "EC2 instance ID for direct SSM session")
	execCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the AWS CLI command instead of executing")

	execCmd.MarkFlagsMutuallyExclusive("instance", "cluster")
	execCmd.MarkFlagsMutuallyExclusive("instance", "service")
	execCmd.MarkFlagsMutuallyExclusive("instance", "task")
	execCmd.MarkFlagsMutuallyExclusive("instance", "container")
	execCmd.MarkFlagsMutuallyExclusive("instance", "command")

	rootCmd.AddCommand(execCmd)
}

func runExec(cmd *cobra.Command, args []string) error {
	if err := exec.CheckAWSCLI(); err != nil {
		return err
	}

	if instance != "" {
		return runDirectSSM()
	}

	if cluster != "" && service != "" {
		return runDirect()
	}

	return runExecTUI()
}

func printEC2ConnectInfo(instanceID string) {
	ctx := context.Background()
	client, err := bridgeaws.NewClient(ctx, profile, region)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connecting to EC2 instance: %s\n", instanceID)
		return
	}
	info, err := client.LookupInstance(ctx, instanceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connecting to EC2 instance: %s\n", instanceID)
		return
	}
	msg := "Connecting to EC2 instance: "
	if info.Name != "" {
		msg += info.Name + " (" + info.ID + ")"
	} else {
		msg += info.ID
	}
	if info.PrivateIP != "" {
		msg += " │ " + info.PrivateIP
	}
	if info.PublicIP != "" {
		msg += " │ " + info.PublicIP
	}
	fmt.Fprintln(os.Stderr, msg)
}

func runDirectSSM() error {
	if dryRun {
		fmt.Fprintf(os.Stdout, "aws ssm start-session --target %s\n", instance)
		return nil
	}
	printEC2ConnectInfo(instance)
	_ = config.AppendHistory(config.HistoryEntry{
		Mode:       "ec2",
		InstanceID: instance,
		Timestamp:  time.Now(),
	})
	return exec.ExecSSM(instance)
}

func runDirect() error {
	ctx := context.Background()

	client, err := bridgeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	tasks, err := client.ListTasks(ctx, cluster, service)
	if err != nil {
		return err
	}

	var selectedTask bridgeaws.TaskInfo
	if task != "" {
		selectedTask, err = bridgeaws.FindTask(tasks, task)
	} else {
		selectedTask, err = bridgeaws.MostRecentTask(tasks)
	}
	if err != nil {
		return err
	}

	selectedContainer, err := bridgeaws.ResolveContainer(selectedTask.Containers, container)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Fprintf(os.Stdout, "aws ecs execute-command --cluster %s --task %s --container %s --command %s --interactive\n",
			cluster, selectedTask.ID, selectedContainer.Name, command)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Connecting: %s > %s > %s > %s\n", cluster, service, selectedTask.ID, selectedContainer.Name)
	fmt.Fprintf(os.Stderr, "Command: %s\n", command)

	_ = config.AppendHistory(config.HistoryEntry{
		Mode:      "ecs",
		Cluster:   cluster,
		Service:   service,
		Task:      selectedTask.ID,
		Container: selectedContainer.Name,
		Command:   command,
		Timestamp: time.Now(),
	})
	return exec.Exec(cluster, selectedTask.ID, selectedContainer.Name, command)
}

func runExecTUI() error {
	ctx := context.Background()

	client, err := bridgeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	model := tui.NewModel(client, command, "")
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	m, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("unexpected TUI state")
	}
	if m.Cancelled() {
		return nil
	}

	sel := m.Selection()

	if sel.Mode == "ec2" {
		if sel.InstanceID == "" {
			return nil
		}
		if dryRun {
			fmt.Fprintf(os.Stdout, "aws ssm start-session --target %s\n", sel.InstanceID)
			return nil
		}
		printEC2ConnectInfo(sel.InstanceID)
		_ = config.AppendHistory(config.HistoryEntry{
			Mode:       "ec2",
			InstanceID: sel.InstanceID,
			Timestamp:  time.Now(),
		})
		return exec.ExecSSM(sel.InstanceID)
	}

	// ECS path
	if sel.Cluster == "" {
		return nil
	}

	if dryRun {
		fmt.Fprintf(os.Stdout, "aws ecs execute-command --cluster %s --task %s --container %s --command %s --interactive\n",
			sel.Cluster, sel.Task, sel.Container, command)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Connecting: %s > %s > %s > %s\n", sel.Cluster, sel.Service, sel.Task, sel.Container)
	fmt.Fprintf(os.Stderr, "Command: %s\n", command)

	_ = config.AppendHistory(config.HistoryEntry{
		Mode:      "ecs",
		Cluster:   sel.Cluster,
		Service:   sel.Service,
		Task:      sel.Task,
		Container: sel.Container,
		Command:   command,
		Timestamp: time.Now(),
	})
	return exec.Exec(sel.Cluster, sel.Task, sel.Container, command)
}
