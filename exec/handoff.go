package exec

import (
	"fmt"
	"os"
	osexec "os/exec"
	"runtime"
	"syscall"
)

// CheckAWSCLI verifies the aws CLI is available in PATH.
func CheckAWSCLI() error {
	_, err := osexec.LookPath("aws")
	if err != nil {
		return fmt.Errorf("aws CLI not found in PATH; install it from https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html")
	}
	return nil
}

// Exec replaces the current process with `aws ecs execute-command`.
// On Unix this uses syscall.Exec for clean process replacement.
// On other platforms it falls back to os/exec.
func Exec(cluster, taskID, container, command string) error {
	awsBin, err := osexec.LookPath("aws")
	if err != nil {
		return fmt.Errorf("aws CLI not found: %w", err)
	}

	args := []string{
		"aws", "ecs", "execute-command",
		"--cluster", cluster,
		"--task", taskID,
		"--container", container,
		"--command", command,
		"--interactive",
	}

	if runtime.GOOS != "windows" {
		return syscall.Exec(awsBin, args, os.Environ())
	}

	// Fallback for non-Unix
	cmd := osexec.Command(awsBin, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
