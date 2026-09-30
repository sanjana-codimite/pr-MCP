// Package runner provides safe command execution with timeouts, output limits,
// and security controls.
//
// Safety rules (Section 10):
//   - No shell: uses exec.CommandContext with argument arrays directly.
//   - Working directory: always set to the configured repo_path.
//   - Timeouts: every command has a context-based timeout; the process group
//     is killed on expiry.
//   - Output limits: stdout/stderr are capped to max_output_bytes.
//   - No secrets: never logs environment variables.
//   - stderr only: all logging goes to stderr.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Result holds the output of a command execution.
type Result struct {
	// ExitCode is the process exit code (-1 if killed or not started).
	ExitCode int
	// Stdout contains the captured standard output (truncated to maxBytes).
	Stdout string
	// Stderr contains the captured standard error (truncated to maxBytes).
	Stderr string
	// Duration is how long the command ran.
	Duration time.Duration
	// TimedOut is true if the command was killed due to timeout.
	TimedOut bool
	// Err is the Go-level error from exec, if any (not the same as non-zero exit code).
	Err error
}

// Run executes a command safely with the given constraints.
//
// It enforces:
//   - No shell invocation (command is passed as name + args, never via sh -c).
//   - Working directory is set to workDir.
//   - Context-based timeout.
//   - Output capture capped at maxBytes.
//
// The command is never run through a shell. Arguments that contain shell
// metacharacters (e.g., "; rm -rf /") remain literal arguments.
func Run(ctx context.Context, workDir string, timeout time.Duration, maxBytes int, name string, args ...string) *Result {
	start := time.Now()
	result := &Result{ExitCode: -1}

	// Create a timeout context.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the command without a shell. This is critical for safety:
	// exec.CommandContext does NOT invoke a shell, so arguments like
	// "; rm -rf /" stay as literal strings passed to the executable.
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir

	// Set up process group killing on timeout (platform-specific).
	setProcAttr(cmd)

	// Capture stdout and stderr with size limits.
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdoutBuf, remaining: maxBytes}
	cmd.Stderr = &limitedWriter{w: &stderrBuf, remaining: maxBytes}

	// Don't inherit the parent's environment variables to avoid leaking secrets.
	// Use a minimal environment. The command inherits PATH and common vars.
	cmd.Env = safeEnv()

	err := cmd.Run()
	result.Duration = time.Since(start)
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.Err = fmt.Errorf("command timed out after %v", timeout)
		return result
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Err = err
		}
		return result
	}

	result.ExitCode = 0
	return result
}

// limitedWriter wraps a writer and stops writing after a byte limit.
type limitedWriter struct {
	w         io.Writer
	remaining int
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.remaining <= 0 {
		return len(p), nil // Discard but don't error.
	}
	if len(p) > lw.remaining {
		p = p[:lw.remaining]
	}
	n, err := lw.w.Write(p)
	lw.remaining -= n
	return n, err
}
