package cmd

import (
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	profile   string
	region    string
	cluster   string
	service   string
	task      string
	container string
	command   string
	instance  string
	dryRun    bool
)

func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

var rootCmd = &cobra.Command{
	Use:     "bridge",
	Short:   "AWS ECS and EC2 management tool",
	Long:    "Bridge simplifies managing AWS ECS tasks and EC2 instances — connect, inspect, tail logs, and more.",
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "AWS profile name")
	rootCmd.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS region")
}

func Execute() error {
	return rootCmd.Execute()
}
