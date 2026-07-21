// Package fpcalc wraps the Chromaprint `fpcalc` command-line tool to compute
// acoustic fingerprints of audio files. The binary is an optional external
// dependency: call Available before constructing a Client to detect it.
package fpcalc

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// DefaultTimeout bounds a single fingerprint run; decoding a track takes on
// the order of a second, so anything past this indicates a stuck decoder.
const DefaultTimeout = 30 * time.Second

// Available reports whether the fpcalc binary can be found. An empty path
// checks the system PATH for "fpcalc".
func Available(path string) bool {
	if path == "" {
		path = "fpcalc"
	}
	_, err := exec.LookPath(path)
	return err == nil
}

// Result is the fingerprint of a single audio file.
type Result struct {
	// Duration of the audio in seconds, as reported by fpcalc.
	Duration float64 `json:"duration"`
	// Fingerprint is the compressed, base64-encoded Chromaprint fingerprint,
	// the format the AcoustID lookup API expects.
	Fingerprint string `json:"fingerprint"`
}

// Client runs fpcalc. The zero value uses "fpcalc" from PATH.
type Client struct {
	// Path to the fpcalc binary; empty means "fpcalc" resolved via PATH.
	Path string
	// Timeout per invocation; zero means DefaultTimeout.
	Timeout time.Duration
}

// New returns a Client using the given binary path ("" = PATH lookup).
func New(path string) *Client {
	return &Client{Path: path}
}

func (c *Client) binary() string {
	if c.Path == "" {
		return "fpcalc"
	}
	return c.Path
}

// Fingerprint computes the acoustic fingerprint of the audio file at absPath.
func (c *Client) Fingerprint(ctx context.Context, absPath string) (Result, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, //nolint:gosec // G204: args are passed directly without a shell; absPath is a library file
		c.binary(), "-json", absPath,
	).Output()
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("fpcalc: %w", ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return Result{}, fmt.Errorf("fpcalc exec: %w: %s", err, exitErr.Stderr)
		}
		return Result{}, fmt.Errorf("fpcalc exec: %w", err)
	}
	return parseOutput(out)
}

// parseOutput maps `fpcalc -json` output into a Result. Kept separate from
// the exec call so parsing can be unit-tested without the binary present.
func parseOutput(out []byte) (Result, error) {
	var r Result
	if err := json.Unmarshal(out, &r); err != nil {
		return Result{}, fmt.Errorf("fpcalc json: %w", err)
	}
	if r.Fingerprint == "" {
		return Result{}, fmt.Errorf("fpcalc: empty fingerprint in output")
	}
	return r, nil
}
