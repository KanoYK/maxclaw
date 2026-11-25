//go:build !windows

package providers

import "os/exec"

func hideSubscriptionConsole(*exec.Cmd) {}
