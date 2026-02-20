package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/janost/barge/config"
	"github.com/spf13/cobra"
)

var bookmarksCmd = &cobra.Command{
	Use:   "bookmarks",
	Short: "List saved bookmarks",
	RunE:  runBookmarks,
}

var bookmarksRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a bookmark",
	Args:  cobra.ExactArgs(1),
	RunE:  runBookmarksRm,
}

var bookmarksPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Remove all bookmarks",
	RunE:  runBookmarksPurge,
}

func init() {
	bookmarksCmd.AddCommand(bookmarksRmCmd)
	bookmarksCmd.AddCommand(bookmarksPurgeCmd)
	execCmd.AddCommand(bookmarksCmd)
}

func runBookmarks(cmd *cobra.Command, args []string) error {
	bookmarks, err := config.LoadBookmarks()
	if err != nil {
		return fmt.Errorf("loading bookmarks: %w", err)
	}
	if len(bookmarks) == 0 {
		fmt.Fprintln(os.Stderr, "No bookmarks saved. Use 'barge exec save <name>' to create one.")
		return nil
	}

	names := make([]string, 0, len(bookmarks))
	for name := range bookmarks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		bm := bookmarks[name]
		switch bm.Mode {
		case "ec2":
			fmt.Fprintf(os.Stdout, "  %-20s  ec2 > %s\n", name, bm.InstanceID)
		default:
			summary := bm.Cluster + " > " + bm.Service
			if bm.Container != "" {
				summary += " > " + bm.Container
			}
			fmt.Fprintf(os.Stdout, "  %-20s  ecs > %s\n", name, summary)
		}
	}
	return nil
}

func runBookmarksRm(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := config.RemoveBookmark(name); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Bookmark %q removed.\n", name)
	return nil
}

func runBookmarksPurge(cmd *cobra.Command, args []string) error {
	if err := config.PurgeBookmarks(); err != nil {
		return fmt.Errorf("purging bookmarks: %w", err)
	}
	fmt.Fprintln(os.Stderr, "All bookmarks removed.")
	return nil
}
