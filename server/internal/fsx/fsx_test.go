//go:build unix

package fsx

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// CreateTemp must request the conventional 0666 and let the process umask clamp
// it, unlike os.CreateTemp which hard-codes 0600 (a mode umask can never widen).
func TestCreateTempHonorsUmask(t *testing.T) {
	cases := []struct {
		umask int
		want  os.FileMode
	}{
		{0o002, 0o664}, // shared media group: readable AND writable
		{0o022, 0o644},
		{0o077, 0o600}, // umask still clamps: not a blind chmod 0666
	}
	for _, tc := range cases {
		dir := t.TempDir()
		old := syscall.Umask(tc.umask)
		f, err := CreateTemp(dir, "x-*.tmp")
		syscall.Umask(old)
		if err != nil {
			t.Fatalf("umask %#o: CreateTemp: %v", tc.umask, err)
		}
		name := f.Name()
		_ = f.Close()
		info, err := os.Stat(name)
		if err != nil {
			t.Fatalf("umask %#o: stat: %v", tc.umask, err)
		}
		if got := info.Mode().Perm(); got != tc.want {
			t.Errorf("umask %#o: mode = %#o, want %#o", tc.umask, got, tc.want)
		}
	}
}

// The atomic-rename callers depend on the os.CreateTemp contract: a unique name
// built from the "<prefix>-*.<suffix>" pattern.
func TestCreateTempAppliesPattern(t *testing.T) {
	dir := t.TempDir()
	f1, err := CreateTemp(dir, "asset-*.tmp")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer func() { _ = f1.Close() }()
	f2, err := CreateTemp(dir, "asset-*.tmp")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer func() { _ = f2.Close() }()

	base := filepath.Base(f1.Name())
	if !strings.HasPrefix(base, "asset-") || !strings.HasSuffix(base, ".tmp") {
		t.Errorf("name %q does not match pattern asset-*.tmp", base)
	}
	if f1.Name() == f2.Name() {
		t.Errorf("two temps share a name: %q", f1.Name())
	}
}
