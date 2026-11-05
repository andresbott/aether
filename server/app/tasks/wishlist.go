package tasks

import (
	"log/slog"

	"github.com/andresbott/aether/internal/store"
)

// reconcileWishlist re-matches the wishlist against the catalog a task just
// wrote. Catalog ids are not stable across a scan, so this runs after every
// pass that writes tracks. A failure is logged, never returned: the catalog
// write already succeeded and the next pass retries the match.
func reconcileWishlist(s *store.Store, log *slog.Logger) {
	stats, err := s.ReconcileWishlist()
	if err != nil {
		log.Warn("wishlist: could not match against the catalog", slog.String("error", err.Error()))
		return
	}
	if stats.Found > 0 || stats.Lost > 0 {
		log.Info("wishlist: matched against the catalog",
			slog.Int("found", stats.Found),
			slog.Int("lost", stats.Lost),
			slog.Int("in_library", stats.Matched))
	}
}
