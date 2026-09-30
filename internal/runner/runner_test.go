package runner

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRun_Success(t *testing.T) {
	dir := t.TempDir()
	var r *Result
	if runtime.GOOS == "windows" {
		r = Run(context.Background(), dir, 10*time.Second, 8192, "cmd", "/c", "echo hello")
	} else {
		r = Run(context.Background(), dir, 10*time.Second, 8192, "echo", "hello")
	}

	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0; stderr: %s", r.ExitCode, r.Stderr)
	}
	if !strings.Contains(r.Stdout, "hello") {
		t.Errorf("Stdout = %q, want to contain 'hello'", r.Stdout)
	}
	if r.TimedOut {
		t.Error("unexpected timeout")
	}
}

func TestRun_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	var r *Result
	if runtime.GOOS == "windows" {
		r = Run(context.Background(), dir, 10*time.Second, 8192, "cmd", "/c", "exit 1")
	} else {
		r = Run(context.Background(), dir, 10*time.Second, 8192, "sh", "-c", "exit 1")
	}

	if r.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", r.ExitCode)
	}
}

func TestRun_Timeout(t *testing.T) {
	dir := t.TempDir()
	var r *Result
	if runtime.GOOS == "windows" {
		r = Run(context.Background(), dir, 500*time.Millisecond, 8192, "cmd", "/c", "ping -n 10 127.0.0.1 > nul")
	} else {
		r = Run(context.Background(), dir, 500*time.Millisecond, 8192, "sleep", "10")
	}

	if !r.TimedOut {
		t.Error("expected timeout, got none")
	}
}

func TestRun_OutputTruncation(t *testing.T) {
	dir := t.TempDir()
	maxBytes := 16
	var r *Result
	if runtime.GOOS == "windows" {
		// Generate output longer than maxBytes.
		r = Run(context.Background(), dir, 10*time.Second, maxBytes, "cmd", "/c", "echo AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	} else {
		r = Run(context.Background(), dir, 10*time.Second, maxBytes, "echo", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	}

	if len(r.Stdout) > maxBytes {
		t.Errorf("Stdout length = %d, want <= %d", len(r.Stdout), maxBytes)
	}
}

func TestRun_NoShellInjection(t *testing.T) {
	// This test verifies that shell metacharacters in arguments are treated literally.
	// The argument "; rm -rf /" should stay as a literal argument, not be interpreted.
	dir := t.TempDir()

	// The "echo" command on Unix will just print the argument literally.
	// If a shell were used, "; rm -rf /" would be dangerous.
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/c", "echo", "; rm -rf /"}
	} else {
		name = "echo"
		args = []string{"; rm -rf /"}
	}

	r := Run(context.Background(), dir, 10*time.Second, 8192, name, args...)

	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", r.ExitCode)
	}
	// The dangerous argument should appear literally in the output.
	if !strings.Contains(r.Stdout, "; rm -rf /") {
		t.Errorf("Stdout = %q, should contain the literal argument", r.Stdout)
	}
}

func TestRun_ExecutableNotFound(t *testing.T) {
	dir := t.TempDir()
	r := Run(context.Background(), dir, 10*time.Second, 8192, "nonexistent-command-xyz")

	if r.Err == nil {
		t.Error("expected error for missing executable, got nil")
	}
	if r.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 for not started", r.ExitCode)
	}
	// Verify it's an exec.Error (command not found).
	if _, ok := r.Err.(*exec.Error); !ok {
		t.Errorf("expected *exec.Error, got %T", r.Err)
	}
}
