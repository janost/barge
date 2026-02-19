package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	bargeaws "github.com/MutuallyAssuredDeployment/barge/aws"
	"github.com/MutuallyAssuredDeployment/barge/config"
	"github.com/MutuallyAssuredDeployment/barge/exec"
	"github.com/MutuallyAssuredDeployment/barge/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	profile   string
	region    string
	cluster   string
	service   string
	task      string
	container string
	command   string
	instance  string
	dryRun    bool
)

func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

var rootCmd = &cobra.Command{
	Use:     "barge",
	Short:   "Open a shell in a running AWS ECS task or EC2 instance",
	Long:    "Barge simplifies connecting to running AWS ECS tasks and EC2 instances via SSM by providing an interactive drill-down selection or direct CLI access.",
	Version: version,
	RunE:    runRoot,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "AWS profile name")
	rootCmd.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS region")
	rootCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "ECS cluster name (skips TUI if set with --service)")
	rootCmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name (skips TUI if set with --cluster)")
	rootCmd.Flags().StringVarP(&task, "task", "t", "", "ECS task ID (optional, defaults to most recent)")
	rootCmd.Flags().StringVarP(&container, "container", "C", "", "Container name (optional, auto-selected if unambiguous)")
	rootCmd.Flags().StringVarP(&command, "command", "x", "/bin/sh", "Command to execute in the container")
	rootCmd.Flags().StringVarP(&instance, "instance", "i", "", "EC2 instance ID for direct SSM session (mutually exclusive with ECS flags)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the AWS CLI command instead of executing")

	rootCmd.MarkFlagsMutuallyExclusive("instance", "cluster")
	rootCmd.MarkFlagsMutuallyExclusive("instance", "service")
	rootCmd.MarkFlagsMutuallyExclusive("instance", "task")
	rootCmd.MarkFlagsMutuallyExclusive("instance", "container")
	rootCmd.MarkFlagsMutuallyExclusive("instance", "command")
}

func Execute() error {
	return rootCmd.Execute()
}

func runRoot(cmd *cobra.Command, args []string) error {
	if err := exec.CheckAWSCLI(); err != nil {
		return err
	}

	if instance != "" {
		return runDirectSSM()
	}

	if cluster != "" && service != "" {
		return runDirect()
	}

	return runTUI()
}

func runDirectSSM() error {
	if dryRun {
		fmt.Fprintf(os.Stdout, "aws ssm start-session --target %s\n", instance)
		return nil
	}
	fmt.Fprintf(os.Stderr, "Connecting to EC2 instance: %s\n", instance)
	_ = config.AppendHistory(config.HistoryEntry{
		Mode:       "ec2",
		InstanceID: instance,
		Timestamp:  time.Now(),
	})
	return exec.ExecSSM(instance)
}

func runDirect() error {
	ctx := context.Background()

	client, err := bargeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	tasks, err := client.ListTasks(ctx, cluster, service)
	if err != nil {
		return err
	}

	var selectedTask bargeaws.TaskInfo
	if task != "" {
		selectedTask, err = bargeaws.FindTask(tasks, task)
	} else {
		selectedTask, err = bargeaws.MostRecentTask(tasks)
	}
	if err != nil {
		return err
	}

	selectedContainer, err := bargeaws.ResolveContainer(selectedTask.Containers, container)
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

func runTUI() error {
	ctx := context.Background()

	client, err := bargeaws.NewClient(ctx, profile, region)
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
		fmt.Fprintf(os.Stderr, "Connecting to EC2 instance: %s\n", sel.InstanceID)
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
