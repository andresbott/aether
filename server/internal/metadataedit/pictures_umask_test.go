//go:build unix

package metadataedit_test

import (
	"os"
	"syscall"
	"testing"

	"github.com/andresbott/aether/internal/metadataedit"
)

// A folder cover written into a track folder must honor the deployment umask, so
// a shared media group can read and write it — not land owner-only 0600.
func TestFolderPicture_HonorsUmask(t *testing.T) {
	dir := t.TempDir()
	old := syscall.Umask(0o002)
	path, err := metadataedit.WriteFolderPicture(dir, "cover", "jpg", jpgBytes)
	syscall.Umask(old)
	if err != nil {
		t.Fatalf("WriteFolderPicture: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o664 {
		t.Errorf("cover mode = %#o, want 0664 (umask 002)", got)
	}
}
