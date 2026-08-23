// Package fsx creates temp files that honor the process umask, so a deployment's
// umask and setgid directories govern the resulting permissions.
//
// It exists because os.CreateTemp always opens 0600 — a mode umask can only
// narrow, never widen — so a temp file, and the file an atomic temp+rename
// carries it to, can never become group-accessible regardless of the umask.
// Requesting the conventional 0666 instead hands control back to the umask and
// lets a setgid parent supply the group, exactly like cp or touch. This matters
// for files aether writes into the shared music library (e.g. folder cover art);
// files under the private DataDir deliberately keep the stdlib defaults.
package fsx

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// filePerm is the conventional broad file mode; the kernel clamps it by the
// process umask at creation time (and a setgid parent supplies the group).
const filePerm = 0o666

// CreateTemp behaves like os.CreateTemp but opens the file with mode 0666
// (subject to umask) instead of the standard library's hard-coded 0600, so the
// temp file — and the file an atomic temp+rename carries it to — participates in
// the deployment's umask/setgid policy.
func CreateTemp(dir, pattern string) (*os.File, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	prefix, suffix, err := prefixAndSuffix(pattern)
	if err != nil {
		return nil, &os.PathError{Op: "createtemp", Path: pattern, Err: err}
	}
	prefix = filepath.Join(dir, prefix)
	for try := 0; ; try++ {
		name := prefix + randomToken() + suffix
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, filePerm) //nolint:gosec // umask-governed temp opened O_EXCL; name is caller dir + random token (see package doc)
		if os.IsExist(err) {
			if try < 10000 {
				continue
			}
			return nil, &os.PathError{Op: "createtemp", Path: prefix + "*" + suffix, Err: os.ErrExist}
		}
		return f, err
	}
}

// prefixAndSuffix splits pattern on the last '*' (mirroring os.CreateTemp) and
// rejects a pattern containing a path separator.
func prefixAndSuffix(pattern string) (prefix, suffix string, err error) {
	for i := 0; i < len(pattern); i++ {
		if os.IsPathSeparator(pattern[i]) {
			return "", "", errors.New("pattern contains path separator")
		}
	}
	if i := strings.LastIndex(pattern, "*"); i >= 0 {
		prefix, suffix = pattern[:i], pattern[i+1:]
	} else {
		prefix = pattern
	}
	return prefix, suffix, nil
}

// randomToken returns a filename-safe random string for temp-file names.
func randomToken() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never fails on supported platforms
	return hex.EncodeToString(b[:])
}
