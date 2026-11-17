// Package gitcmd runs git commands.
package gitcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// timeout bounds every git command, so a stalled fetch or push (a dead
// connection) cannot hang the caller.
const timeout = 2 * time.Minute

// Run runs git in dir and returns its output without the trailing newline;
// on failure the error carries git's message, and the output is still
// returned (some commands, like merge-tree, report through it).
func Run(dir string, args ...string) (string, error) {
	return RunEnv(dir, nil, args...)
}

// RunEnv is Run with extra environment variables (KEY=value). git never
// prompts for credentials: a run without a terminal must fail, not hang.
func RunEnv(dir string, env []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), env...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	output := strings.TrimRight(string(out), "\n")
	if err != nil {
		return output, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

// ExitCode returns the exit code carried by an error from Run, or -1 when
// git did not run to an exit.
func ExitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}
