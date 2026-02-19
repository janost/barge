package cmd

import (
	"context"
	"fmt"
	"os"

	bargeaws "github.com/MutuallyAssuredDeployment/barge/aws"
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
)

func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

var rootCmd = &cobra.Command{
	Use:     "barge",
	Short:   "Open a shell in a running AWS ECS task",
	Long:    "Barge simplifies connecting to running AWS ECS tasks by providing an interactive drill-down selection or direct CLI access.",
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
}

func Execute() error {
	return rootCmd.Execute()
}

func runRoot(cmd *cobra.Command, args []string) error {
	if err := exec.CheckAWSCLI(); err != nil {
		return err
	}

	if cluster != "" && service != "" {
		return runDirect()
	}

	return runTUI()
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

	fmt.Fprintf(os.Stderr, "Connecting: %s > %s > %s > %s\n", cluster, service, selectedTask.ID, selectedContainer.Name)
	fmt.Fprintf(os.Stderr, "Command: %s\n", command)

	return exec.Exec(cluster, selectedTask.ID, selectedContainer.Name, command)
}

func runTUI() error {
	ctx := context.Background()

	client, err := bargeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	model := tui.NewModel(client, command)
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
	if sel.Cluster == "" {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Connecting: %s > %s > %s > %s\n", sel.Cluster, sel.Service, sel.Task, sel.Container)
	fmt.Fprintf(os.Stderr, "Command: %s\n", command)

	return exec.Exec(sel.Cluster, sel.Task, sel.Container, command)
}
