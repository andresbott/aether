// internal/scanner/scanner.go
package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

type ScanOptions struct {
	IsFull bool
	// Log receives task-scoped log lines (e.g. into the per-execution log). A nil
	// Log discards them, so callers outside the task runner can omit it.
	Log *slog.Logger
	// Progress receives scan progress (percentage + current-file stage). A nil
	// value disables reporting. tempo.Progress satisfies this interface.
	Progress ProgressReporter
}

// ProgressReporter receives scan progress. The task runner passes tempo's
// reporter (which satisfies it); the scanner never imports tempo. A scan counts
// each file twice — once for the tag read, once for the reconcile save — so the
// percentage spans both phases rather than stalling at 100% during the save.
// Implementations must be safe for concurrent use: the tag-read worker pool
// calls it from multiple goroutines.
type ProgressReporter interface {
	SetTotal(total int64)
	Inc(delta int64) int64
	SetStage(msg string)
}

type noopProgress struct{}

func (noopProgress) SetTotal(int64)  {}
func (noopProgress) Inc(int64) int64 { return 0 }
func (noopProgress) SetStage(string) {}

// relPath renders p relative to root for a progress stage line, falling back to
// the base name when it cannot (e.g. a different volume).
func relPath(root, p string) string {
	if rel, err := filepath.Rel(root, p); err == nil {
		return rel
	}
	return filepath.Base(p)
}

type ScanStats struct {
	TracksProcessed int
	TracksNew       int
	TracksUpdated   int
	// TracksSkipped counts paths a caller supplied that the scan folder does not
	// cover and that were therefore deliberately not indexed (outside the root,
	// not an audio extension, excluded, unreadable). Only RescanPaths can
	// produce these — it is handed an explicit path list — so a full Scan, whose
	// paths all come from its own walk, always leaves this zero. It exists so a
	// caller can tell "skipped by design" from "reconcile failed" instead of
	// inferring a shortfall from TracksProcessed.
	TracksSkipped int
	// TracksFailed counts tracks whose per-track reconcile transaction still
	// failed after its one retry (a lost write lock, a constraint violation, an
	// association-replace error). They are skipped so one bad file does not fail
	// the whole scan — but, unlike TracksSkipped (skipped by design) and Errors
	// (tag-read failures), these are tracks the scan meant to index and could
	// not, so a non-zero count is a real shortfall a caller should surface.
	TracksFailed int
	Errors       []error
}

type Scanner struct {
	cfg       Config
	store     *store.Store
	tagReader tags.Reader
}

func New(cfg Config, s *store.Store, tagReader tags.Reader) *Scanner {
	return &Scanner{cfg: cfg, store: s, tagReader: tagReader}
}

type tagResult struct {
	walk WalkResult
	meta tags.Metadata
	// audioHash is the file's metadata-invariant audio hash, or "" when it has
	// none. Read alongside the tags, because both are per-file work that wants
	// the worker pool rather than the reconcile transaction.
	audioHash string
}

// folderWalk is one scan folder plus the walk that cleared its guards. It exists
// so the guards can run for *every* folder before *any* folder is reconciled
// (see preflight) without walking the tree twice.
type folderWalk struct {
	folder    scanfolder.Folder
	walk      []WalkResult
	toProcess []WalkResult
}

func (s *Scanner) Scan(ctx context.Context, opts ScanOptions) (ScanStats, error) {
	scanStart := time.Now()
	stats := ScanStats{}

	log := opts.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	// Name order: it decides who owns a file reachable from two folders (the last
	// one to walk it), so it must be the same on every run.
	folders := s.cfg.Folders.All()
	if len(folders) == 0 {
		log.Info("no scan folders configured; nothing to scan")
		return stats, nil
	}

	// Phase 1: validate and walk everything. Nothing is written yet, so a guard
	// tripping here aborts the whole run atomically.
	walks, err := s.preflight(ctx, folders)
	if err != nil {
		return stats, err
	}

	prog := opts.Progress
	if prog == nil {
		prog = noopProgress{}
	}

	// Compute the work total up front so the percentage spans both passes: each
	// file to process counts twice — once for its tag read, once for its
	// reconcile save. Selecting toProcess here (not inside scanFolder) is what
	// makes the total known before the first save.
	var totalFiles int64
	for i := range walks {
		walks[i].toProcess = s.selectToProcess(walks[i].walk, opts.IsFull)
		totalFiles += int64(len(walks[i].toProcess))
		log.Info("folder scan planned",
			slog.String("scan_folder", walks[i].folder.Name),
			slog.Int("found", len(walks[i].walk)),
			slog.Int("to_process", len(walks[i].toProcess)))
	}
	log.Info("scan plan", slog.Int("scan_folders", len(walks)), slog.Int64("files_to_process", totalFiles))
	prog.SetTotal(totalFiles * 2)

	// Phase 2: reconcile.
	for i := range walks {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		before := stats
		log.Info("reconciling folder",
			slog.String("scan_folder", walks[i].folder.Name),
			slog.Int("files", len(walks[i].toProcess)))
		if err := s.scanFolder(ctx, walks[i], scanStart, log, prog, &stats); err != nil {
			return stats, err
		}
		log.Info("folder reconciled",
			slog.String("scan_folder", walks[i].folder.Name),
			slog.Int("processed", stats.TracksProcessed-before.TracksProcessed),
			slog.Int("new", stats.TracksNew-before.TracksNew),
			slog.Int("updated", stats.TracksUpdated-before.TracksUpdated),
			slog.Int("failed", stats.TracksFailed-before.TracksFailed))
	}

	if ctx.Err() == nil {
		prog.SetStage("Cleaning up…")
		log.Info("running cleanup")
		if err := s.store.Cleanup(ctx, scanStart); err != nil {
			return stats, err
		}
	}

	return stats, nil
}

// preflight runs both sweep guards and the walk for every scan folder before
// Scan reconciles the first one, and returns the walk results so phase 2 does
// not repeat the I/O (walking twice would also risk seeing two different
// trees).
//
// The two-phase split is what makes an aborted run harmless. Scan returns on the
// first scan folder that fails a guard, but planTrackContinuity's candidate pool
// is deliberately not scan-folder-scoped — a move between two collections has to
// keep its row — so an unavailable scan folder that is merely *later* in
// the set's name order used to have all of its rows stat ENOENT and land in
// `vanished` while an earlier scan folder was still being reconciled. A single
// byte-identical new file there was enough to re-link an unreachable scan
// folder's row, moving its stars, playlist memberships and history onto a file
// it has nothing to do with, and the guard then failed the scan too late to undo
// any of it. Validating first makes the abort happen before the first write.
//
// Still not covered, and not coverable here: an unreadable or unmounted *subtree*
// inside a root that is present (a per-directory mount that is gone leaves an
// empty mountpoint directory behind). Its files stat ENOENT exactly like deleted
// ones, so those rows can be swept — and, since the fingerprint cannot tell the
// difference either, re-linked onto a byte-identical new file. Requiring a
// vanished row's parent directory to still exist would break the primary use
// case, because reorganising a scan folder moves whole directories. The
// narrowing to fs.ErrNotExist in planTrackContinuity is the only defence, and it
// only helps when the failure is a permission error rather than an empty
// mountpoint.
func (s *Scanner) preflight(ctx context.Context, folders []scanfolder.Folder) ([]folderWalk, error) {
	out := make([]folderWalk, 0, len(folders))
	for _, folder := range folders {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Walk swallows the root's own stat error (walk.go), so without this an
		// unmounted folder scans "successfully" with zero results and Cleanup
		// deletes everything in it.
		if err := folder.Available(); err != nil {
			return nil, fmt.Errorf("scan folder %q: %w", folder.Name, err)
		}

		excludes, err := folder.Excludes()
		if err != nil {
			return nil, err
		}

		walkResults, err := Walk(folder, excludes)
		if err != nil {
			return nil, err
		}

		// An absent tree is not an empty tree. Walk swallows every error, including
		// the root's, so a share that is present but unpopulated (a bare mountpoint)
		// looks exactly like a folder the user emptied — and Cleanup would delete
		// every track of it, with the playlists, stars, play history and queue
		// entries attached to them. The DB still holding tracks is the only evidence
		// available, so it decides.
		if err := s.checkEmptyScanWithIndexedTracks(folder, walkResults, len(folders) == 1); err != nil {
			return nil, err
		}

		out = append(out, folderWalk{folder: folder, walk: walkResults})
	}
	return out, nil
}

// scanFolder is phase 2: mark, read tags and reconcile one scan folder that
// preflight has already validated and walked.
func (s *Scanner) scanFolder(ctx context.Context, fw folderWalk, scanStart time.Time, log *slog.Logger, prog ProgressReporter, stats *ScanStats) error {
	folder, walkResults := fw.folder, fw.walk

	allPaths := make([]string, len(walkResults))
	for i, wr := range walkResults {
		allPaths[i] = wr.FilePath
	}
	if err := s.store.BulkMarkSeen(allPaths, folder.Name, scanStart); err != nil {
		return err
	}

	toProcess := fw.toProcess

	workers := s.cfg.TagReadWorkers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	tagResults := make([]tagResult, 0, len(toProcess))
	var mu sync.Mutex
	var wg sync.WaitGroup
	ch := make(chan WalkResult, workers*2)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for wr := range ch {
				if ctx.Err() != nil {
					return
				}
				prog.SetStage("Extracting metadata: " + relPath(folder.Path, wr.FilePath))
				prog.Inc(1)
				log.Info("scanning song", slog.String("file", relPath(folder.Path, wr.FilePath)))
				// No separate tagReader.CanRead gate: Walk only admits IsAudioFile
				// paths, IsAudioFile is tags.Supported, and every supported format is
				// readable by some reader (enforced by tags.TestSupportedIsReadable),
				// so admission asks one question, not two — the same reasoning as
				// WalkWouldEmit.
				meta, err := s.tagReader.Read(ctx, wr.FilePath)
				if err != nil {
					mu.Lock()
					stats.Errors = append(stats.Errors, err)
					mu.Unlock()
					continue
				}
				// Hashed in the worker, next to the tag read: it is bounded
				// per-file I/O (audiohash reads at most 256 KiB of payload) and
				// belongs on the pool rather than in reconcile's per-track
				// transaction. Only files this pass reads get one, so an
				// incremental scan hashes exactly what changed and a steady
				// state hashes nothing.
				hash := audioHashOf(wr.FilePath)
				mu.Lock()
				tagResults = append(tagResults, tagResult{walk: wr, meta: meta, audioHash: hash})
				mu.Unlock()
			}
		}()
	}

	for _, wr := range toProcess {
		if ctx.Err() != nil {
			break
		}
		ch <- wr
	}
	close(ch)
	wg.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	rec, err := s.reconcile(ctx, folder.Path, tagResults, scanStart, log, prog)
	if err != nil {
		return err
	}
	stats.TracksProcessed += rec.Processed
	stats.TracksNew += rec.New
	stats.TracksUpdated += rec.Updated
	stats.TracksFailed += rec.Failed

	return nil
}

// selectToProcess is the set of files scanFolder will read and reconcile: the
// whole walk for a full scan, or only the changed files for an incremental one.
// Hoisted out of scanFolder so Scan can total it before any folder is saved.
func (s *Scanner) selectToProcess(walkResults []WalkResult, isFull bool) []WalkResult {
	if isFull {
		return walkResults
	}
	return s.filterChanged(walkResults)
}

func (s *Scanner) filterChanged(results []WalkResult) []WalkResult {
	paths := make([]string, len(results))
	for i, wr := range results {
		paths[i] = wr.FilePath
	}
	modMap, err := s.store.FilterChanged(paths)
	if err != nil {
		slog.Warn("filterChanged store error; falling back to full rescan", "err", err)
		return results
	}
	var out []WalkResult
	for _, wr := range results {
		dbMod, found := modMap[wr.FilePath]
		if !found || dbMod.Before(wr.ModTime) {
			out = append(out, wr)
		}
	}
	return out
}

// checkEmptyScanWithIndexedTracks refuses to continue when a folder's walk found
// zero files while the database still holds tracks for it, as this indicates an
// unmounted share or permission issue rather than a genuinely emptied folder.
// onlyFolder says whether this is the only scan folder configured, which changes
// the remedy the error message can honestly offer (see below).
func (s *Scanner) checkEmptyScanWithIndexedTracks(folder scanfolder.Folder, walkResults []WalkResult, onlyFolder bool) error {
	if len(walkResults) > 0 {
		return nil
	}
	indexed, err := s.store.CountTracksInScanFolder(folder.Name, folder.Path)
	if err != nil {
		return fmt.Errorf("scan folder %q: count indexed tracks: %w", folder.Name, err)
	}
	if indexed == 0 {
		return nil
	}
	if onlyFolder {
		// The usual remedy ("remove it, the next scan removes the tracks") does not
		// hold here: with no scan folders left configured, Scan returns before
		// Cleanup ever runs (see Scan), so removing this entry would not itself
		// remove anything.
		return fmt.Errorf("scan folder %q: no audio files under %q but %d tracks are indexed; "+
			"refusing to delete them — check that the path is mounted; it is the only scan folder configured, so "+
			"removing it from ScanFolders would leave none, and with no scan folders configured a scan does "+
			"nothing at all: those %d tracks would stay indexed until another scan folder is configured and scanned",
			folder.Name, folder.Path, indexed, indexed)
	}
	// The remedy has a price and has to say so: removing the folder from the
	// config is exactly the hard-delete this guard just refused to perform.
	return fmt.Errorf("scan folder %q: no audio files under %q but %d tracks are indexed; "+
		"refusing to delete them — check that the path is mounted; if the folder really is gone, "+
		"remove it from ScanFolders in the config file and restart: the next scan then removes those %d tracks "+
		"and everything attached to them (playlist entries, stars, play history)",
		folder.Name, folder.Path, indexed, indexed)
}
