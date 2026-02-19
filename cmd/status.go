package cmd

import (
	"context"
	"fmt"
	"os"

	bargeaws "github.com/MutuallyAssuredDeployment/barge/aws"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show ECS service health summary",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "ECS cluster name")
	statusCmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	resolvedCluster, resolvedService, err := resolveClusterService(ctx)
	if err != nil {
		return err
	}

	client, err := bargeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	detail, err := client.DescribeServiceDetail(ctx, resolvedCluster, resolvedService)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Service: %s  Cluster: %s\n\n", resolvedService, resolvedCluster)

	fmt.Fprintf(os.Stdout, "  Tasks:     %d running / %d desired / %d pending\n",
		detail.RunningCount, detail.DesiredCount, detail.PendingCount)

	for _, d := range detail.Deployments {
		age := formatAge(d.UpdatedAt)
		fmt.Fprintf(os.Stdout, "  Deploy:    %s — %s, updated %s ago\n",
			d.Status, d.TaskDef, age)
	}

	tasks, err := client.ListTasks(ctx, resolvedCluster, resolvedService)
	if err != nil {
		return err
	}

	if len(tasks) > 0 {
		fmt.Fprintf(os.Stdout, "\n  Running Tasks:\n")
		for _, t := range tasks {
			started := "pending"
			if !t.StartedAt.IsZero() {
				started = "started " + formatAge(t.StartedAt) + " ago"
			}
			fmt.Fprintf(os.Stdout, "    %-12s  %s  %s  rev %s\n",
				t.ID, t.Status, started, t.Revision)
		}
	}

	stoppedTasks, err := client.ListStoppedTasks(ctx, resolvedCluster, resolvedService)
	if err == nil && len(stoppedTasks) > 0 {
		fmt.Fprintf(os.Stdout, "\n  Recent Stops:\n")
		for _, t := range stoppedTasks {
			reason := t.StopReason
			if reason == "" {
				reason = "unknown"
			}
			stopped := formatAge(t.StoppedAt) + " ago"
			fmt.Fprintf(os.Stdout, "    %-12s  STOPPED  %s  %s\n",
				t.ID, reason, stopped)
		}
	}

	return nil
}
