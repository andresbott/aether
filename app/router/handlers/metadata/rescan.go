package metadata

import (
	"context"
	"fmt"

	"github.com/andresbott/aether/internal/scanner"
)

// TrackRescanner re-reads the tags of specific files and updates the library
// index. Satisfied by *scanner.Scanner. It exists so the editor can make an
// edit visible in the music UI without waiting for a scan task. Shared by the
// tag and picture handlers (the identify handler never writes, so it has no
// rescanner).
type TrackRescanner interface {
	RescanPaths(ctx context.Context, libraryID uint, absPaths []string) (scanner.ScanStats, error)
}

// rescanStatus reports the outcome of the post-write re-index. A failure is
// never fatal — the write already landed on disk — but the recovery differs by
// write kind. A tag or embedded-picture write changes the audio file's mtime,
// so `filterChanged` (scanner.go) admits it and the next incremental scan
// catches up on its own. A folder-cover or artist-image write touches no audio
// mtime, so an incremental scan reconciles zero tracks in that directory and
// never re-runs cover detection: only a full scan (or an unrelated tag edit to
// a track in the same folder) repoints it. rescanFolderArt annotates the error
// for those writes so the toast does not falsely promise "the next scan".
type rescanStatus struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// folderArtRescanNote is appended to a failed folder-cover / artist-image
// rescan's error. These writes leave every audio mtime untouched, so an
// incremental scan will never revisit the folder to re-detect the cover
// (filterChanged keys on audio size/mtime); a full scan is required.
const folderArtRescanNote = " A full library scan is required to update the index — an incremental scan will not pick up a cover-art change on its own."

// rescanSaved re-indexes absPaths, returning nil when re-indexing is disabled
// (rescan is nil) or there is nothing to do (the response then carries no
// "rescan" field).
//
// OK is true only when every path the library covers was re-indexed. The
// re-index swallows per-file failures (a lost SQLite write lock skips one
// track, an unreadable file lands in ScanStats.Errors) and still returns a nil
// error, so inspecting err alone would promise "the index is current" for a run
// that indexed nothing — exactly the overlapping-scan case this feature exists
// for.
func rescanSaved(ctx context.Context, rescan TrackRescanner, libraryID uint, absPaths []string) *rescanStatus {
	if rescan == nil || len(absPaths) == 0 {
		return nil
	}
	stats, err := rescan.RescanPaths(ctx, libraryID, absPaths)
	if err != nil {
		return &rescanStatus{Error: err.Error()}
	}
	if msg := incompleteRescanMessage(stats, len(absPaths)); msg != "" {
		return &rescanStatus{Error: msg}
	}
	return &rescanStatus{OK: true}
}

// rescanFolderArt is rescanSaved for writes that only touch on-disk image files
// (folder covers, artist images), never an audio file's tags. Because these
// writes leave every audio mtime untouched, a failed synchronous re-index does
// NOT self-heal on the next incremental scan the way a tag write does — it needs
// a full scan. On failure it appends folderArtRescanNote so the message says so
// instead of implying the index will catch up on its own.
func rescanFolderArt(ctx context.Context, rescan TrackRescanner, libraryID uint, absPaths []string) *rescanStatus {
	rs := rescanSaved(ctx, rescan, libraryID, absPaths)
	if rs != nil && !rs.OK && rs.Error != "" {
		rs.Error += folderArtRescanNote
	}
	return rs
}

// incompleteRescanMessage describes a partial re-index ("" = every path the
// library covers made it). Per-file errors are summarised rather than
// concatenated: a folder-wide picture write can produce hundreds, and this text
// ends up in a toast.
//
// The shortfall is measured against the paths the scanner actually admitted
// (total minus the ones it deliberately skipped), never against total. The
// editor lists exactly the formats the scanner indexes (both gate on
// tags.Supported), but it still ignores the library's exclude patterns, so a
// perfectly correct save can hand RescanPaths a path the scanner skips as
// excluded. Comparing against total would warn the user about a save that
// worked.
func incompleteRescanMessage(stats scanner.ScanStats, total int) string {
	if len(stats.Errors) > 0 {
		if len(stats.Errors) == 1 {
			return stats.Errors[0].Error()
		}
		return fmt.Sprintf("%d files could not be re-indexed; first error: %v",
			len(stats.Errors), stats.Errors[0])
	}
	indexable := total - stats.TracksSkipped
	if stats.TracksProcessed < indexable {
		return fmt.Sprintf("only %d of %d files were re-indexed", stats.TracksProcessed, indexable)
	}
	return ""
}
