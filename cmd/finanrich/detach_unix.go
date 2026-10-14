//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detach põe o navegador num grupo de processos próprio.
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
