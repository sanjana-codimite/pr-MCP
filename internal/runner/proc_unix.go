//go:build !windows

package runner

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcAttr configures the command to run in its own process group
// so that the entire group can be killed on timeout.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// Kill the entire process group.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// safeEnv returns a minimal environment for commands, inheriting only
// essential variables. This avoids leaking secrets from the parent environment.
func safeEnv() []string {
	env := []string{}
	for _, key := range []string{"PATH", "HOME", "USER", "GOPATH", "GOROOT", "TMPDIR", "LANG", "LC_ALL"} {
		if val := os.Getenv(key); val != "" {
			env = append(env, key+"="+val)
		}
	}
	return env
}
