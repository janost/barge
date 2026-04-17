package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/spf13/cobra"
)

var (
	listECSOnly bool
	listEC2Only bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List ECS clusters/services and EC2 instances",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listECSOnly, "ecs-only", false, "Show only ECS clusters and services")
	listCmd.Flags().BoolVar(&listEC2Only, "ec2-only", false, "Show only EC2 instances")
	listCmd.MarkFlagsMutuallyExclusive("ecs-only", "ec2-only")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := bridgeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	ecsRendered := false
	if !listEC2Only {
		clusters, err := client.ListClusters(ctx)
		if err != nil {
			return err
		}

		if len(clusters) == 0 {
			fmt.Fprintln(os.Stderr, "No ECS clusters found.")
		} else {
			ecsRendered = true
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
		}
	}

	if !listECSOnly {
		instances, err := client.ListInstances(ctx)
		if err == nil && len(instances) > 0 {
			if ecsRendered {
				fmt.Fprintln(w) // blank line separator between sections
			}
			fmt.Fprintln(w, "INSTANCE\tNAME\tPLATFORM\tPRIVATE IP\tPUBLIC IP")
			for _, inst := range instances {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", inst.ID, inst.Name, inst.Platform, inst.PrivateIP, inst.PublicIP)
			}
		}
	}

	return w.Flush()
}
