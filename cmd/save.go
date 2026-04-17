package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/janost/bridge/config"
	"github.com/janost/bridge/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save a connection target as a named bookmark",
	Args:  cobra.ExactArgs(1),
	RunE:  runSave,
}

func init() {
	saveCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "ECS cluster name")
	saveCmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name")
	saveCmd.Flags().StringVarP(&container, "container", "C", "", "Container name")
	saveCmd.Flags().StringVarP(&command, "command", "x", "/bin/sh", "Command to execute")
	saveCmd.Flags().StringVarP(&instance, "instance", "i", "", "EC2 instance ID")
	saveCmd.MarkFlagsMutuallyExclusive("instance", "cluster")
	execCmd.AddCommand(saveCmd)
}

func runSave(cmd *cobra.Command, args []string) error {
	name := args[0]

	existing, _ := config.LoadBookmarks()
	if _, ok := existing[name]; ok {
		fmt.Fprintf(os.Stderr, "Bookmark %q already exists. Overwrite? [y/N]: ", name)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(os.Stderr, "Cancelled.")
			return nil
		}
	}

	var bm config.Bookmark

	if instance != "" {
		bm = config.Bookmark{
			Mode:       "ec2",
			InstanceID: instance,
		}
	} else if cluster != "" && service != "" {
		bm = config.Bookmark{
			Mode:      "ecs",
			Cluster:   cluster,
			Service:   service,
			Container: container,
			Command:   command,
		}
	} else {
		resolved, err := saveViaTUI()
		if err != nil {
			return err
		}
		bm = resolved
	}

	if err := config.SaveBookmark(name, bm); err != nil {
		return fmt.Errorf("saving bookmark: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Bookmark %q saved.\n", name)
	return nil
}

func saveViaTUI() (config.Bookmark, error) {
	ctx := context.Background()
	client, err := bridgeaws.NewClient(ctx, profile, region)
	if err != nil {
		return config.Bookmark{}, err
	}

	model := tui.NewModel(client, "", "")
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return config.Bookmark{}, fmt.Errorf("TUI error: %w", err)
	}

	m, ok := finalModel.(tui.Model)
	if !ok {
		return config.Bookmark{}, fmt.Errorf("unexpected TUI state")
	}
	if m.Cancelled() {
		return config.Bookmark{}, fmt.Errorf("cancelled")
	}

	sel := m.Selection()

	if sel.Mode == "ec2" {
		return config.Bookmark{
			Mode:       "ec2",
			InstanceID: sel.InstanceID,
		}, nil
	}

	return config.Bookmark{
		Mode:      "ecs",
		Cluster:   sel.Cluster,
		Service:   sel.Service,
		Container: sel.Container,
		Command:   command,
	}, nil
}
