package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	bargeaws "github.com/MutuallyAssuredDeployment/barge/aws"
	"github.com/MutuallyAssuredDeployment/barge/config"
	"github.com/MutuallyAssuredDeployment/barge/exec"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "List recent connections and reconnect",
	RunE:  runHistory,
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all connection history",
	RunE:  runHistoryClear,
}

func init() {
	historyCmd.AddCommand(historyClearCmd)
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	entries, err := config.LoadHistory()
	if err != nil {
		return fmt.Errorf("loading history: %w", err)
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No connection history.")
		return nil
	}

	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		idx := len(entries) - i
		age := formatAge(e.Timestamp)
		switch e.Mode {
		case "ec2":
			fmt.Fprintf(os.Stdout, "  %2d. [%s ago] ec2 > %s\n", idx, age, e.InstanceID)
		default:
			summary := e.Cluster + " > " + e.Service
			if e.Container != "" {
				summary += " > " + e.Container
			}
			fmt.Fprintf(os.Stdout, "  %2d. [%s ago] ecs > %s\n", idx, age, summary)
		}
	}

	fmt.Fprintf(os.Stderr, "\nEnter number to reconnect (or q to quit): ")
	var input string
	fmt.Scanln(&input)
	if input == "q" || input == "" {
		return nil
	}

	var choice int
	if _, err := fmt.Sscanf(input, "%d", &choice); err != nil || choice < 1 || choice > len(entries) {
		return fmt.Errorf("invalid selection: %s", input)
	}

	entry := entries[len(entries)-choice]
	return reconnectEntry(entry)
}

func reconnectEntry(entry config.HistoryEntry) error {
	if err := exec.CheckAWSCLI(); err != nil {
		return err
	}

	if entry.Mode == "ec2" {
		fmt.Fprintf(os.Stderr, "Reconnecting to EC2 instance: %s\n", entry.InstanceID)
		return exec.ExecSSM(entry.InstanceID)
	}

	ctx := context.Background()
	client, err := bargeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	tasks, err := client.ListTasks(ctx, entry.Cluster, entry.Service)
	if err != nil {
		return fmt.Errorf("listing tasks for %s/%s: %w", entry.Cluster, entry.Service, err)
	}

	selectedTask, err := bargeaws.MostRecentTask(tasks)
	if err != nil {
		return err
	}

	containerName := entry.Container
	if containerName == "" {
		c, err := bargeaws.ResolveContainer(selectedTask.Containers, "")
		if err != nil {
			return err
		}
		containerName = c.Name
	}

	cmd := entry.Command
	if cmd == "" {
		cmd = "/bin/sh"
	}

	fmt.Fprintf(os.Stderr, "Reconnecting: %s > %s > %s > %s\n",
		entry.Cluster, entry.Service, selectedTask.ID, containerName)

	return exec.Exec(entry.Cluster, selectedTask.ID, containerName, cmd)
}

func runHistoryClear(cmd *cobra.Command, args []string) error {
	if err := config.ClearHistory(); err != nil {
		return fmt.Errorf("clearing history: %w", err)
	}
	fmt.Fprintln(os.Stderr, "History cleared.")
	return nil
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
