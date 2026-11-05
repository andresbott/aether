package store

import (
	"fmt"
	"sort"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/unidecode"
	"gorm.io/gorm/clause"
)

// GroupingsFilter narrows grouping-browsing endpoints.
type GroupingsFilter struct {
	Scope TrackScope
}

// GroupingWithCounts is a grouping plus the footprint the index shows. The
// counts honour the filter's scope: a grouping seen through a library counts
// only that library's albums and tracks.
type GroupingWithCounts struct {
	model.Grouping
	AlbumCount int
	SongCount  int
	Duration   int
	// Year is the earliest non-zero album year, 0 when none carries one.
	Year int
}

// groupingAlbumOrder is how a grouping lists its albums: oldest first, albums without
// a year last, then by name.
const groupingAlbumOrder = "CASE WHEN albums.year = 0 THEN 1 ELSE 0 END, albums.year ASC, albums.name_norm ASC"

// GetGroupings returns the grouping index: every grouping with at least one
// album (with a track in the scope, when there is one), by normalized name.
func (s *Store) GetGroupings(filter *GroupingsFilter) ([]GroupingWithCounts, error) {
	q := s.db.
		Table("groupings").
		Select(`groupings.*,
			COUNT(DISTINCT albums.id) AS album_count,
			COUNT(tracks.id) AS song_count,
			COALESCE(SUM(tracks.duration), 0) AS duration,
			COALESCE(MIN(CASE WHEN albums.year > 0 THEN albums.year END), 0) AS year`).
		Joins("JOIN albums ON albums.grouping_id = groupings.id").
		Joins("JOIN tracks ON tracks.album_id = albums.id")
	if filter != nil {
		q = scopeTracks(q, filter.Scope)
	}
	var out []GroupingWithCounts
	err := q.Group("groupings.id").Order("groupings.name_norm ASC").Scan(&out).Error
	return out, err
}

// GetGrouping returns one grouping and its albums in browsing order. A scope
// narrows the albums to those with a track in it; the grouping itself is
// returned even when the scope leaves it empty.
func (s *Store) GetGrouping(id uint, filter *GroupingsFilter) (*model.Grouping, []model.Album, error) {
	var g model.Grouping
	if err := s.db.First(&g, id).Error; err != nil {
		return nil, nil, notFound(err)
	}
	q := s.db.
		Preload("Artists").
		Preload("Genres").
		Where("albums.grouping_id = ?", id)
	if filter != nil {
		q = scopeByAlbum(q, filter.Scope, "albums.id")
	}
	var albums []model.Album
	if err := q.Order(groupingAlbumOrder).Find(&albums).Error; err != nil {
		return nil, nil, err
	}
	return &g, albums, nil
}

// GroupingCoverAlbum returns the album whose cover stands for the grouping —
// the first of its browsing order — or ErrNotFound when it has no albums.
func (s *Store) GroupingCoverAlbum(id uint) (*model.Album, error) {
	var a model.Album
	err := s.db.Where("albums.grouping_id = ?", id).Order(groupingAlbumOrder).First(&a).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

// AlbumGroupings maps album ids to their grouping; albums without one are
// absent.
func (s *Store) AlbumGroupings(albumIDs []uint) (map[uint]model.Grouping, error) {
	out := make(map[uint]model.Grouping)
	if len(albumIDs) == 0 {
		return out, nil
	}
	type row struct {
		AlbumID uint
		model.Grouping
	}
	for i := 0; i < len(albumIDs); i += chunkSize {
		end := min(i+chunkSize, len(albumIDs))
		var rows []row
		err := s.db.Table("albums").
			Select("albums.id AS album_id, groupings.*").
			Joins("JOIN groupings ON groupings.id = albums.grouping_id").
			Where("albums.id IN ?", albumIDs[i:end]).
			Scan(&rows).Error
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.AlbumID] = r.Grouping
		}
	}
	return out, nil
}

// ReconcileGroupings recomputes every album's grouping from its tracks'
// GROUPING tags and drops groupings no album uses any more.
//
// An album's grouping is the value most of its tracks carry (compared on the
// normalized name, so "Doctor Who" and "doctor who" agree); a tie goes to the
// alphabetically first name, so the answer does not depend on scan order. An
// album none of whose tracks carries the tag has no grouping — removing the
// tag un-groups it. The display name of a new grouping is the most common
// spelling among the tracks that chose it; an existing grouping keeps its name.
//
// It reads the tracks' stored tags rather than the scan's changes, so it is
// correct after an incremental scan that skipped unchanged files, and it is
// idempotent. Run it after every scan and targeted rescan.
func (s *Store) ReconcileGroupings() error {
	return s.Transaction(func(tx *Store) error {
		votes, err := tx.groupingVotes()
		if err != nil {
			return err
		}
		winners := pickGroupingWinners(votes)
		ids, err := tx.ensureGroupings(winners)
		if err != nil {
			return err
		}
		if err := tx.assignAlbumGroupings(winners, ids); err != nil {
			return err
		}
		if err := tx.db.Exec(`DELETE FROM groupings WHERE id NOT IN (SELECT DISTINCT grouping_id FROM albums WHERE grouping_id IS NOT NULL)`).Error; err != nil {
			return fmt.Errorf("delete empty groupings: %w", err)
		}
		return nil
	})
}

// groupingCandidate is one normalized grouping name an album's tracks voted for.
type groupingCandidate struct {
	norm      string
	n         int
	spellings map[string]int
}

// groupingVotes tallies, per album, the tracks carrying each normalized grouping.
func (s *Store) groupingVotes() (map[uint]map[string]*groupingCandidate, error) {
	var rows []struct {
		AlbumID  uint
		Grouping string
		N        int
	}
	if err := s.db.Table("tracks").
		Select("album_id, grouping, COUNT(*) AS n").
		Where("grouping != ''").
		Group("album_id, grouping").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read track groupings: %w", err)
	}
	byAlbum := map[uint]map[string]*groupingCandidate{}
	for _, r := range rows {
		norm := unidecode.Normalize(r.Grouping)
		if norm == "" {
			continue
		}
		m := byAlbum[r.AlbumID]
		if m == nil {
			m = map[string]*groupingCandidate{}
			byAlbum[r.AlbumID] = m
		}
		c := m[norm]
		if c == nil {
			c = &groupingCandidate{norm: norm, spellings: map[string]int{}}
			m[norm] = c
		}
		c.n += r.N
		c.spellings[r.Grouping] += r.N
	}
	return byAlbum, nil
}

// pickGroupingWinners chooses each voted album's grouping: the most votes, ties
// to the alphabetically first normalized name.
func pickGroupingWinners(votes map[uint]map[string]*groupingCandidate) map[uint]*groupingCandidate {
	winners := make(map[uint]*groupingCandidate, len(votes))
	for albumID, m := range votes {
		var best *groupingCandidate
		for _, c := range m {
			if best == nil || c.n > best.n || (c.n == best.n && c.norm < best.norm) {
				best = c
			}
		}
		winners[albumID] = best
	}
	return winners
}

// ensureGroupings resolves (creating what is missing) the grouping row of every
// winning name once, returning their ids by normalized name.
func (s *Store) ensureGroupings(winners map[uint]*groupingCandidate) (map[string]uint, error) {
	display := map[string]string{}
	norms := make([]string, 0)
	for _, c := range winners {
		if _, seen := display[c.norm]; seen {
			continue
		}
		norms = append(norms, c.norm)
		display[c.norm] = mostCommon(c.spellings)
	}
	sort.Strings(norms)
	ids := make(map[string]uint, len(norms))
	for _, norm := range norms {
		g := model.Grouping{Name: display[norm], NameNorm: norm}
		if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&g).Error; err != nil {
			return nil, fmt.Errorf("create grouping %q: %w", norm, err)
		}
		// A conflict leaves the ID at 0, so always read the row back.
		var stored model.Grouping
		if err := s.db.Where("name_norm = ?", norm).First(&stored).Error; err != nil {
			return nil, fmt.Errorf("read grouping %q: %w", norm, err)
		}
		ids[norm] = stored.ID
	}
	return ids, nil
}

// assignAlbumGroupings writes only the albums whose grouping changed: winners
// are set, albums with no vote are cleared.
func (s *Store) assignAlbumGroupings(winners map[uint]*groupingCandidate, ids map[string]uint) error {
	var current []struct {
		ID         uint
		GroupingID *uint
	}
	if err := s.db.Table("albums").Select("id, grouping_id").Scan(&current).Error; err != nil {
		return fmt.Errorf("read album groupings: %w", err)
	}
	toSet := map[uint][]uint{} // grouping id -> album ids
	var toClear []uint
	for _, a := range current {
		w := winners[a.ID]
		switch {
		case w == nil && a.GroupingID != nil:
			toClear = append(toClear, a.ID)
		case w != nil && (a.GroupingID == nil || *a.GroupingID != ids[w.norm]):
			toSet[ids[w.norm]] = append(toSet[ids[w.norm]], a.ID)
		}
	}
	if err := execInChunks(s.db, toClear, `UPDATE albums SET grouping_id = NULL WHERE id IN ?`); err != nil {
		return fmt.Errorf("clear album groupings: %w", err)
	}
	for gid, albumIDs := range toSet {
		for i := 0; i < len(albumIDs); i += chunkSize {
			end := min(i+chunkSize, len(albumIDs))
			if err := s.db.Exec(`UPDATE albums SET grouping_id = ? WHERE id IN ?`, gid, albumIDs[i:end]).Error; err != nil {
				return fmt.Errorf("set album groupings: %w", err)
			}
		}
	}
	return nil
}

// mostCommon returns the spelling with the highest count, ties to the
// alphabetically first so the choice is deterministic.
func mostCommon(spellings map[string]int) string {
	best, bestN := "", -1
	for sp, n := range spellings {
		if n > bestN || (n == bestN && sp < best) {
			best, bestN = sp, n
		}
	}
	return best
}
