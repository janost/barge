package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	bargeaws "github.com/MutuallyAssuredDeployment/barge/aws"
	"github.com/MutuallyAssuredDeployment/barge/exec"
	"github.com/MutuallyAssuredDeployment/barge/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	cpCluster   string
	cpService   string
	cpTask      string
	cpContainer string
)

var cpCmd = &cobra.Command{
	Use:   "cp <src> <dst>",
	Short: "Copy files to/from an ECS container",
	Long: `Copy files between your local machine and a running ECS container.

Remote paths are prefixed with ':'. Local paths have no prefix.

Examples:
  barge cp ./local-file.txt :/remote/path          # upload
  barge cp :/remote/path ./local-file.txt           # download
  barge cp -c cluster -s service :/var/log/app.log . # download, skip TUI

Requires the container to have: sh, tar, base64.`,
	Args: cobra.ExactArgs(2),
	RunE: runCp,
}

func init() {
	cpCmd.Flags().StringVarP(&cpCluster, "cluster", "c", "", "ECS cluster name")
	cpCmd.Flags().StringVarP(&cpService, "service", "s", "", "ECS service name")
	cpCmd.Flags().StringVarP(&cpTask, "task", "t", "", "ECS task ID (defaults to most recent)")
	cpCmd.Flags().StringVarP(&cpContainer, "container", "C", "", "Container name (auto-selected if unambiguous)")
	execCmd.AddCommand(cpCmd)
}

func runCp(cmd *cobra.Command, args []string) error {
	if err := exec.CheckAWSCLI(); err != nil {
		return err
	}

	src, dst := args[0], args[1]

	srcRemote := strings.HasPrefix(src, ":")
	dstRemote := strings.HasPrefix(dst, ":")

	if srcRemote == dstRemote {
		if srcRemote {
			return fmt.Errorf("both paths are remote; one must be local")
		}
		return fmt.Errorf("neither path is remote; prefix the remote path with ':'")
	}

	// Resolve the target container
	sel, err := resolveTarget()
	if err != nil {
		return err
	}

	if srcRemote {
		// Download: remote -> local
		remotePath := strings.TrimPrefix(src, ":")
		return exec.CopyFromContainer(sel.cluster, sel.task, sel.container, remotePath, dst)
	}

	// Upload: local -> remote
	remotePath := strings.TrimPrefix(dst, ":")
	return exec.CopyToContainer(sel.cluster, sel.task, sel.container, src, remotePath)
}

type cpTarget struct {
	cluster   string
	task      string
	container string
}

func resolveTarget() (cpTarget, error) {
	if cpCluster != "" && cpService != "" {
		return resolveTargetDirect()
	}
	return resolveTargetTUI()
}

func resolveTargetDirect() (cpTarget, error) {
	ctx := context.Background()

	client, err := bargeaws.NewClient(ctx, profile, region)
	if err != nil {
		return cpTarget{}, err
	}

	tasks, err := client.ListTasks(ctx, cpCluster, cpService)
	if err != nil {
		return cpTarget{}, err
	}

	var selectedTask bargeaws.TaskInfo
	if cpTask != "" {
		selectedTask, err = bargeaws.FindTask(tasks, cpTask)
	} else {
		selectedTask, err = bargeaws.MostRecentTask(tasks)
	}
	if err != nil {
		return cpTarget{}, err
	}

	selectedContainer, err := bargeaws.ResolveContainer(selectedTask.Containers, cpContainer)
	if err != nil {
		return cpTarget{}, err
	}

	fmt.Fprintf(os.Stderr, "Target: %s > %s > %s > %s\n", cpCluster, cpService, selectedTask.ID, selectedContainer.Name)

	return cpTarget{
		cluster:   cpCluster,
		task:      selectedTask.ID,
		container: selectedContainer.Name,
	}, nil
}

func resolveTargetTUI() (cpTarget, error) {
	ctx := context.Background()

	client, err := bargeaws.NewClient(ctx, profile, region)
	if err != nil {
		return cpTarget{}, err
	}

	model := tui.NewModel(client, "", "ecs") // cp is ECS-only, skip mode selection
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return cpTarget{}, fmt.Errorf("TUI error: %w", err)
	}

	m, ok := finalModel.(tui.Model)
	if !ok {
		return cpTarget{}, fmt.Errorf("unexpected TUI state")
	}
	if m.Cancelled() {
		return cpTarget{}, fmt.Errorf("cancelled")
	}

	sel := m.Selection()
	if sel.Cluster == "" {
		return cpTarget{}, fmt.Errorf("no selection made")
	}

	fmt.Fprintf(os.Stderr, "Target: %s > %s > %s > %s\n", sel.Cluster, sel.Service, sel.Task, sel.Container)

	return cpTarget{
		cluster:   sel.Cluster,
		task:      sel.Task,
		container: sel.Container,
	}, nil
}
