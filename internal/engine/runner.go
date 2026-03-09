package engine

import (
	"fmt"
	"io"
	"os/exec"
)

// RunOptions configures how a shell command is executed.
type RunOptions struct {
	// Dir is the working directory for the command.
	Dir string

	// Env is the full environment for the command (use envloader.Merge to build).
	Env []string

	// Stdout and Stderr receive command output (typically the deploy logger).
	Stdout io.Writer
	Stderr io.Writer
}

// RunCommand executes a single shell command string via /bin/sh -c.
// Returns a non-nil error if the command exits with a non-zero status.
func RunCommand(cmd string, opts RunOptions) error {
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Dir = opts.Dir
	c.Env = opts.Env
	c.Stdout = opts.Stdout
	c.Stderr = opts.Stderr

	if err := c.Run(); err != nil {
		return fmt.Errorf("command %q failed: %w", cmd, err)
	}
	return nil
}

// RunCommands executes a sequence of commands using the same options.
// Stops immediately on the first failure, returning the error with context.
func RunCommands(cmds []string, opts RunOptions) error {
	for i, cmd := range cmds {
		if err := RunCommand(cmd, opts); err != nil {
			return fmt.Errorf("step %d/%d: %w", i+1, len(cmds), err)
		}
	}
	return nil
}
