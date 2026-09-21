//go:build !windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

func startDetached(self string, args []string, log *os.File) error {
	cmd := exec.Command(self, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
