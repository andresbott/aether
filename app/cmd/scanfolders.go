package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
)

// startupProbeTimeout bounds each scan folder's availability check at
// startup, so a dead mount costs the boot sequence this long at most, not
// forever.
const startupProbeTimeout = 3 * time.Second

// scanFolderSet builds the immutable scan-folder set from the config entries.
// Every structural rule lives in scanfolder.NewSet, so a typo in the file fails
// here — at load — with the folder named in the message.
func scanFolderSet(cfgs []ScanFolderCfg) (*scanfolder.Set, error) {
	folders := make([]scanfolder.Folder, 0, len(cfgs))
	for _, c := range cfgs {
		folders = append(folders, scanfolder.Folder{
			Name:            c.Name,
			Path:            c.Path,
			ExcludePatterns: c.ExcludePatterns,
			FollowSymlinks:  c.FollowSymlinks == nil || *c.FollowSymlinks,
		})
	}
	set, err := scanfolder.NewSet(folders)
	if err != nil {
		return nil, fmt.Errorf("config ScanFolders: %w", err)
	}
	return set, nil
}

// warnScanFolders logs the things an operator should hear about at startup and
// that must NOT stop the server:
//
//   - one Info line per configured scan folder (name, path, exclude-pattern
//     count, follow-symlinks), so a silently misread config is visible.
//   - a scan folder whose root cannot be scanned — missing, not a directory, a
//     symlink, or a mount that does not answer. A share that mounts late must
//     not keep the server down; scans refuse to run against it until it is
//     fixed.
//   - no scan folder configured at all: scans then do nothing, ever, until the
//     config names one, and the /rest media guard refuses every on-disk file, so
//     an index that is still populated lists but does not play.
//   - indexed tracks stamped with a scan folder that is no longer configured.
//     At the next scan they are re-stamped if a configured folder still reaches
//     their files (that is what a rename looks like), and removed otherwise —
//     together with their stars, playlist entries and play history. With no
//     scan folder configured at all neither happens, so the wording says that
//     instead of promising a removal that cannot occur. Saying so here is the
//     window in which a mistyped or commented-out entry can still be put back.
func warnScanFolders(l *slog.Logger, s *store.Store, folders *scanfolder.Set) {
	all := folders.All()
	for _, f := range all {
		l.Info("scan folder loaded",
			slog.String("component", "startup"),
			slog.String("scan_folder", f.Name), slog.String("path", f.Path),
			slog.Int("exclude_patterns", len(f.ExcludePatterns)),
			slog.Bool("follow_symlinks", f.FollowSymlinks))
		if err := f.AvailableWithin(startupProbeTimeout); err != nil {
			l.Warn("scan folder is not usable; scans will refuse to run until this is fixed",
				slog.String("component", "startup"),
				slog.String("scan_folder", f.Name), slog.String("error", err.Error()))
		}
	}
	if len(all) == 0 {
		l.Info("no scan folders configured; scans do nothing (nothing is indexed and nothing is removed) and no on-disk "+
			"media is served (streams, folder art and embedded covers are refused) until ScanFolders is set in the config file",
			slog.String("component", "startup"))
	}

	counts, err := s.TrackCountsByScanFolder()
	if err != nil {
		l.Warn("could not check indexed tracks against the configured scan folders",
			slog.String("component", "startup"), slog.String("error", err.Error()))
		return
	}
	for name, n := range counts {
		if name == "" {
			continue // never stamped: the next scan stamps or removes it
		}
		if _, ok := folders.ByName(name); ok {
			continue
		}
		if len(all) == 0 {
			l.Warn("indexed tracks carry a scan-folder name that is no longer configured; scans do nothing while no "+
				"folder is configured, so they are neither re-stamped nor removed — and they are listed but cannot be "+
				"played, because no on-disk media is served without a scan folder",
				slog.String("component", "startup"),
				slog.String("scan_folder", name), slog.Int64("tracks", n))
			continue
		}
		l.Warn("indexed tracks carry a scan-folder name that is no longer configured; at the next scan they are "+
			"re-stamped if a configured folder still reaches their files (that is what a rename looks like), and "+
			"removed otherwise, with their stars, playlist entries and play history",
			slog.String("component", "startup"),
			slog.String("scan_folder", name), slog.Int64("tracks", n))
	}
}
