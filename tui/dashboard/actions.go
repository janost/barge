package dashboard

import (
	osexec "os/exec"

	bargeexec "github.com/janost/barge/exec"
	tea "github.com/charmbracelet/bubbletea"
)

// apiResultMsg is sent when an API action completes.
type apiResultMsg struct {
	message string
	err     error
}

// NewAPIAction creates an action that calls an AWS API and returns a result message.
func NewAPIAction(name string, fn func() error, successMsg string) Action {
	return Action{
		Name: name,
		Run: func() tea.Cmd {
			return func() tea.Msg {
				err := fn()
				if err != nil {
					return apiResultMsg{"", err}
				}
				return apiResultMsg{successMsg, nil}
			}
		},
	}
}

// NewExecAction creates an action that runs a subprocess in a bordered terminal.
func NewExecAction(name, title string, cmd *osexec.Cmd) Action {
	return Action{
		Name: name,
		Run: func() tea.Cmd {
			return tea.Exec(&bargeexec.BorderedExec{Title: title, Cmd: cmd}, func(err error) tea.Msg {
				return processExitMsg{err}
			})
		},
	}
}

// SSMShellAction creates an SSM session action for an EC2 instance.
func SSMShellAction(instanceID, title string) Action {
	cmd := osexec.Command("aws", "ssm", "start-session", "--target", instanceID)
	return NewExecAction("SSM Shell", title, cmd)
}

// ECSExecAction creates an ECS Exec interactive shell action.
func ECSExecAction(cluster, taskID, container, title string) Action {
	cmd := osexec.Command("aws", "ecs", "execute-command",
		"--cluster", cluster,
		"--task", taskID,
		"--container", container,
		"--interactive",
		"--command", "/bin/sh")
	return NewExecAction("ECS Exec", title, cmd)
}
