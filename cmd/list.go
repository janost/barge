package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	bargeaws "github.com/MutuallyAssuredDeployment/barge/aws"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all ECS clusters and services",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := bargeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	clusters, err := client.ListClusters(ctx)
	if err != nil {
		return err
	}

	if len(clusters) == 0 {
		fmt.Fprintln(os.Stderr, "No ECS clusters found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "CLUSTER\tSERVICE\tRUNNING\tTASK DEFINITION")

	for _, cluster := range clusters {
		services, err := client.ListServices(ctx, cluster)
		if err != nil {
			fmt.Fprintf(w, "%s\t(error: %s)\t\t\n", cluster, err)
			continue
		}

		if len(services) == 0 {
			fmt.Fprintf(w, "%s\t(no services)\t\t\n", cluster)
			continue
		}

		for _, svc := range services {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", cluster, svc.Name, svc.RunningCount, svc.TaskDefName)
		}
	}

	return w.Flush()
}
