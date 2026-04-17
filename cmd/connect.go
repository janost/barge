package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/janost/bridge/config"
	"github.com/janost/bridge/exec"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect <name>",
	Short: "Connect to a saved bookmark",
	Args:  cobra.ExactArgs(1),
	RunE:  runConnect,
}

func init() {
	connectCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the AWS CLI command instead of executing")
	execCmd.AddCommand(connectCmd)
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
		return fmt.Errorf("bookmark %q not found; run 'bridge exec bookmarks' to list", name)
	}

	if bm.Mode == "ec2" {
		if dryRun {
			fmt.Fprintf(os.Stdout, "aws ssm start-session --target %s\n", bm.InstanceID)
			return nil
		}
		_ = config.AppendHistory(config.HistoryEntry{
			Mode:       "ec2",
			InstanceID: bm.InstanceID,
			Timestamp:  time.Now(),
		})
		printEC2ConnectInfo(bm.InstanceID)
		return exec.ExecSSM(bm.InstanceID)
	}

	// ECS: resolve most recent task
	ctx := context.Background()
	client, err := bridgeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	tasks, err := client.ListTasks(ctx, bm.Cluster, bm.Service)
	if err != nil {
		return fmt.Errorf("listing tasks for %s/%s: %w", bm.Cluster, bm.Service, err)
	}

	selectedTask, err := bridgeaws.MostRecentTask(tasks)
	if err != nil {
		return err
	}

	containerName := bm.Container
	if containerName == "" {
		c, err := bridgeaws.ResolveContainer(selectedTask.Containers, "")
		if err != nil {
			return err
		}
		containerName = c.Name
	}

	cmd2 := bm.Command
	if cmd2 == "" {
		cmd2 = "/bin/sh"
	}

	if dryRun {
		fmt.Fprintf(os.Stdout, "aws ecs execute-command --cluster %s --task %s --container %s --command %s --interactive\n",
			bm.Cluster, selectedTask.ID, containerName, cmd2)
		return nil
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
