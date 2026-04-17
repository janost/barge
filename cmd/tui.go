// cmd/tui.go
package cmd

import (
	"context"
	"fmt"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/janost/bridge/config"
	"github.com/janost/bridge/tui/dashboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive resource browser (k9s-style dashboard)",
	Long:  "Launch an interactive TUI dashboard for browsing and managing AWS resources. Navigate with arrow keys, press Enter for actions.",
	RunE:  runTUICmd,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUICmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, err := bridgeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	cfg := config.Load()
	resource := dashboard.NewResourcePicker()
	model := dashboard.New(client, resource, cfg)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
