package cmd

import (
	"context"
	"fmt"
	"os"

	bargeaws "github.com/MutuallyAssuredDeployment/barge/aws"
	"github.com/spf13/cobra"
)

var eventsLimit int

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Show recent ECS service events",
	RunE:  runEvents,
}

func init() {
	eventsCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "ECS cluster name")
	eventsCmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name")
	eventsCmd.Flags().IntVarP(&eventsLimit, "limit", "n", 25, "Maximum number of events to display")
	rootCmd.AddCommand(eventsCmd)
}

func runEvents(cmd *cobra.Command, args []string) error {
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

	if len(detail.Events) == 0 {
		fmt.Fprintln(os.Stderr, "No events found.")
		return nil
	}

	limit := eventsLimit
	if limit > len(detail.Events) {
		limit = len(detail.Events)
	}

	for _, e := range detail.Events[:limit] {
		ts := e.Timestamp.Local().Format("2006-01-02 15:04:05")
		fmt.Fprintf(os.Stdout, "%s  %s\n", ts, e.Message)
	}

	return nil
}
