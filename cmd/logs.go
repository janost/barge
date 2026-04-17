package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/janost/bridge/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	logsSince    string
	logsFilter   string
	logsNoFollow bool
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail CloudWatch logs for an ECS service",
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "ECS cluster name")
	logsCmd.Flags().StringVarP(&service, "service", "s", "", "ECS service name")
	logsCmd.Flags().StringVar(&logsSince, "since", "5m", "How far back to start (e.g. 5m, 1h, 24h)")
	logsCmd.Flags().StringVar(&logsFilter, "filter", "", "CloudWatch filter pattern")
	logsCmd.Flags().BoolVar(&logsNoFollow, "no-follow", false, "Print logs and exit instead of tailing")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	resolvedCluster, resolvedService, err := resolveClusterService(ctx)
	if err != nil {
		return err
	}

	client, err := bridgeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	logCfg, err := client.ResolveLogConfig(ctx, resolvedCluster, resolvedService)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Log group: %s\n", logCfg.LogGroup)
	if logCfg.StreamPrefix != "" {
		fmt.Fprintf(os.Stderr, "Stream prefix: %s\n", logCfg.StreamPrefix)
	}

	since, err := parseDuration(logsSince)
	if err != nil {
		return fmt.Errorf("invalid --since value %q: %w", logsSince, err)
	}
	startTime := time.Now().Add(-since)

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	var nextToken *string
	lastTimestamp := startTime

	for {
		events, token, err := client.FetchLogs(ctx, logCfg.LogGroup, logCfg.StreamPrefix, logsFilter, lastTimestamp, nextToken)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		for _, e := range events {
			ts := e.Timestamp.Local().Format("2006-01-02 15:04:05")
			fmt.Fprintf(os.Stdout, "%s  %s\n", ts, e.Message)
			if e.Timestamp.After(lastTimestamp) {
				lastTimestamp = e.Timestamp.Add(time.Millisecond)
			}
		}

		if token != nil && *token != "" {
			nextToken = token
			continue
		}

		if logsNoFollow {
			return nil
		}

		nextToken = nil
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
}

// resolveClusterService resolves cluster and service from flags or TUI.
func resolveClusterService(ctx context.Context) (string, string, error) {
	if cluster != "" && service != "" {
		return cluster, service, nil
	}

	client, err := bridgeaws.NewClient(ctx, profile, region)
	if err != nil {
		return "", "", err
	}

	model := tui.NewModel(client, "", "ecs")
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", "", fmt.Errorf("TUI error: %w", err)
	}

	m, ok := finalModel.(tui.Model)
	if !ok {
		return "", "", fmt.Errorf("unexpected TUI state")
	}
	if m.Cancelled() {
		return "", "", fmt.Errorf("cancelled")
	}

	sel := m.Selection()
	if sel.Cluster == "" || sel.Service == "" {
		return "", "", fmt.Errorf("no cluster/service selected")
	}

	return sel.Cluster, sel.Service, nil
}

// parseDuration parses shorthand duration strings like "5m", "1h", "24h", "7d".
func parseDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}

	if s[len(s)-1] == 'd' {
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}
