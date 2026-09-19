package cmd

import (
	"fmt"
	"log/slog"

	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/store"
)

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

// warnScanFolders logs the two things an operator should hear about at startup
// and that must NOT stop the server:
//
//   - a scan folder whose directory is not there. A share that mounts late must
//     not keep the server down; the scan preflight refuses to run against it, so
//     nothing is swept in the meantime.
//   - indexed tracks stamped with a scan folder that is no longer configured.
//     Nothing walks them any more, so the next scan removes them — together with
//     their stars, playlist entries and play history. Saying so here is the
//     window in which a mistyped or commented-out entry can still be put back.
func warnScanFolders(l *slog.Logger, s *store.Store, folders *scanfolder.Set) {
	for _, f := range folders.All() {
		if err := f.Available(); err != nil {
			l.Warn("scan folder directory is unavailable; scans will refuse to run until it is back",
				slog.String("component", "startup"),
				slog.String("scan_folder", f.Name), slog.String("error", err.Error()))
		}
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
		l.Warn("indexed tracks belong to a scan folder that is no longer configured; the next scan will remove them, "+
			"with their stars, playlist entries and play history",
			slog.String("component", "startup"),
			slog.String("scan_folder", name), slog.Int64("tracks", n))
	}
}
