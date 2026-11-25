package providers

import (
	"os/exec"
	"syscall"
)

func hideSubscriptionConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
