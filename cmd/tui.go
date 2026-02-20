// cmd/tui.go
package cmd

import (
	"context"
	"fmt"

	bargeaws "github.com/janost/barge/aws"
	"github.com/janost/barge/tui/dashboard"
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

	client, err := bargeaws.NewClient(ctx, profile, region)
	if err != nil {
		return err
	}

	resource := dashboard.NewEC2Resource()
	model := dashboard.New(client, resource)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
