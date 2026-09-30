//go:build windows

package runner

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcAttr configures the command for Windows.
// On Windows, we use CREATE_NEW_PROCESS_GROUP to allow terminating
// the process tree on timeout.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// safeEnv returns a minimal environment for commands, inheriting only
// essential variables. This avoids leaking secrets from the parent environment.
func safeEnv() []string {
	env := []string{}
	for _, key := range []string{
		"PATH", "PATHEXT", "SYSTEMROOT", "COMSPEC", "TEMP", "TMP",
		"USERPROFILE", "HOMEDRIVE", "HOMEPATH",
		"GOPATH", "GOROOT",
		"PROGRAMFILES", "PROGRAMFILES(X86)", "APPDATA", "LOCALAPPDATA",
	} {
		if val := os.Getenv(key); val != "" {
			env = append(env, key+"="+val)
		}
	}
	return env
}
