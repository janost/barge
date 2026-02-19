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

var connectCmd = &cobra.Command{
	Use:   "connect <name>",
	Short: "Connect to a saved bookmark",
	Args:  cobra.ExactArgs(1),
	RunE:  runConnect,
}

func init() {
	rootCmd.AddCommand(connectCmd)
}

func runConnect(cmd *cobra.Command, args []string) error {
	if err := exec.CheckAWSCLI(); err != nil {
		return err
	}

	name := args[0]
	bookmarks, err := config.LoadBookmarks()
	if err != nil {
		return fmt.Errorf("loading bookmarks: %w", err)
	}

	bm, ok := bookmarks[name]
	if !ok {
		return fmt.Errorf("bookmark %q not found; run 'barge bookmarks' to list", name)
	}

	if bm.Mode == "ec2" {
		_ = config.AppendHistory(config.HistoryEntry{
			Mode:       "ec2",
			InstanceID: bm.InstanceID,
			Timestamp:  time.Now(),
		})
		fmt.Fprintf(os.Stderr, "Connecting to EC2 instance: %s\n", bm.InstanceID)
		return exec.ExecSSM(bm.InstanceID)
	}

	// ECS: resolve most recent task
	ctx := context.Background()
	client, err := bargeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	tasks, err := client.ListTasks(ctx, bm.Cluster, bm.Service)
	if err != nil {
		return fmt.Errorf("listing tasks for %s/%s: %w", bm.Cluster, bm.Service, err)
	}

	selectedTask, err := bargeaws.MostRecentTask(tasks)
	if err != nil {
		return err
	}

	containerName := bm.Container
	if containerName == "" {
		c, err := bargeaws.ResolveContainer(selectedTask.Containers, "")
		if err != nil {
			return err
		}
		containerName = c.Name
	}

	cmd2 := bm.Command
	if cmd2 == "" {
		cmd2 = "/bin/sh"
	}

	_ = config.AppendHistory(config.HistoryEntry{
		Mode:      "ecs",
		Cluster:   bm.Cluster,
		Service:   bm.Service,
		Task:      selectedTask.ID,
		Container: containerName,
		Command:   cmd2,
		Timestamp: time.Now(),
	})

	fmt.Fprintf(os.Stderr, "Connecting: %s > %s > %s > %s\n",
		bm.Cluster, bm.Service, selectedTask.ID, containerName)

	return exec.Exec(bm.Cluster, selectedTask.ID, containerName, cmd2)
}
