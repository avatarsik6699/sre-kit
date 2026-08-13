// Package infrastructure spawns adapter subprocesses and plumbs their stdio, implementing
// adapterengine/application's Spawner/LineSource ports against os/exec.
package infrastructure

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"sre-kit/internal/adapterengine/application"
)

// ProcessSpawner implements application.Spawner against os/exec.
type ProcessSpawner struct{}

// NewProcessSpawner constructs a ProcessSpawner.
func NewProcessSpawner() *ProcessSpawner { return &ProcessSpawner{} }

// Subprocess implements application.LineSource: it scans NDJSON lines from the adapter's stdout
// and reports the exit error once the stream is drained.
type Subprocess struct {
	cmd     *exec.Cmd
	scanner *bufio.Scanner
	stderr  *bytes.Buffer
}

// Spawn starts command with args, writes config to its stdin, and returns a Subprocess ready to
// scan NDJSON lines from stdout. ctx cancellation kills the process (adapter timeout handling).
func (s *ProcessSpawner) Spawn(ctx context.Context, command string, args []string, config []byte) (application.LineSource, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = bytes.NewReader(config)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("adapterengine: stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("adapterengine: start %s: %w", command, err)
	}

	return &Subprocess{cmd: cmd, scanner: bufio.NewScanner(stdout), stderr: &stderr}, nil
}

func (p *Subprocess) Scan() bool   { return p.scanner.Scan() }
func (p *Subprocess) Text() string { return p.scanner.Text() }
func (p *Subprocess) Err() error   { return p.scanner.Err() }

// Wait waits for the subprocess to exit, returning a non-nil error (including stderr output) on a
// non-zero exit or timeout.
func (p *Subprocess) Wait() error {
	if err := p.cmd.Wait(); err != nil {
		if p.stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, p.stderr.String())
		}
		return err
	}
	return nil
}
