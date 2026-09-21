//go:build windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

const detachedProcess = 0x00000008

func startDetached(self string, args []string, log *os.File) error {
	cmd := exec.Command(self, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}
	return cmd.Start()
}
