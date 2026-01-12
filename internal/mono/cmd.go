package mono

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

const DefaultTimeout = 30 * time.Second

func Command(name string, args ...string) *Cmd {
	return &Cmd{
		name:    name,
		args:    args,
		timeout: DefaultTimeout,
	}
}

type Cmd struct {
	name    string
	args    []string
	dir     string
	timeout time.Duration
	stdout  io.Writer
	stderr  io.Writer
	env     []string
}

func (c *Cmd) Dir(dir string) *Cmd {
	c.dir = dir
	return c
}

func (c *Cmd) Timeout(d time.Duration) *Cmd {
	c.timeout = d
	return c
}

func (c *Cmd) Stdout(w io.Writer) *Cmd {
	c.stdout = w
	return c
}

func (c *Cmd) Stderr(w io.Writer) *Cmd {
	c.stderr = w
	return c
}

func (c *Cmd) Env(env []string) *Cmd {
	c.env = env
	return c
}

func (c *Cmd) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.name, c.args...)
	if c.dir != "" {
		cmd.Dir = c.dir
	}
	if c.stdout != nil {
		cmd.Stdout = c.stdout
	}
	if c.stderr != nil {
		cmd.Stderr = c.stderr
	}
	if c.env != nil {
		cmd.Env = c.env
	}

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("command timed out after %v", c.timeout)
	}
	return err
}

func (c *Cmd) Output() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.name, c.args...)
	if c.dir != "" {
		cmd.Dir = c.dir
	}
	if c.env != nil {
		cmd.Env = c.env
	}

	output, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", c.timeout)
	}
	return output, err
}

func (c *Cmd) CombinedOutput() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.name, c.args...)
	if c.dir != "" {
		cmd.Dir = c.dir
	}
	if c.env != nil {
		cmd.Env = c.env
	}

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", c.timeout)
	}
	return output, err
}

type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

func (c *Cmd) RunCapture() (*RunResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.name, c.args...)
	if c.dir != "" {
		cmd.Dir = c.dir
	}
	if c.env != nil {
		cmd.Env = c.env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", c.timeout)
	}

	result := &RunResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: 0,
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return result, err
}
