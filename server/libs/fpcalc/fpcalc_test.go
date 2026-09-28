package fpcalc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseOutput(t *testing.T) {
	tcs := []struct {
		name    string
		in      string
		want    Result
		wantErr string
	}{
		{
			name: "valid output",
			in:   `{"duration": 641.33, "fingerprint": "AQADtNQYhYkYnGj8"}`,
			want: Result{Duration: 641.33, Fingerprint: "AQADtNQYhYkYnGj8"},
		},
		{
			name:    "empty fingerprint",
			in:      `{"duration": 10.0, "fingerprint": ""}`,
			wantErr: "empty fingerprint",
		},
		{
			name:    "invalid json",
			in:      `not json`,
			wantErr: "fpcalc json",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOutput([]byte(tc.in))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOutput: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// writeFakeFpcalc creates an executable script that echoes a fixed JSON
// payload, standing in for the real binary.
func writeFakeFpcalc(t *testing.T, dir, payload string) string {
	t.Helper()
	path := filepath.Join(dir, "fpcalc")
	script := "#!/bin/sh\necho '" + payload + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFingerprintWithFakeBinary(t *testing.T) {
	dir := t.TempDir()
	bin := writeFakeFpcalc(t, dir, `{"duration": 123.4, "fingerprint": "ABC123"}`)

	c := New(bin)
	got, err := c.Fingerprint(context.Background(), "/some/file.mp3")
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	want := Result{Duration: 123.4, Fingerprint: "ABC123"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestFingerprintMissingBinary(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "does-not-exist"))
	_, err := c.Fingerprint(context.Background(), "/some/file.mp3")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestFingerprintTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fpcalc")
	// exec replaces the shell so the context kill reaches sleep itself and the
	// output pipe closes immediately instead of after the full sleep.
	script := "#!/bin/sh\nexec sleep 5\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Client{Path: path, Timeout: 50 * time.Millisecond}
	_, err := c.Fingerprint(context.Background(), "/some/file.mp3")
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected deadline error, got %v", err)
	}
}

func TestAvailable(t *testing.T) {
	dir := t.TempDir()
	bin := writeFakeFpcalc(t, dir, `{}`)
	if !Available(bin) {
		t.Error("expected Available(true) for existing binary")
	}
	if Available(filepath.Join(dir, "missing")) {
		t.Error("expected Available(false) for missing binary")
	}
}
