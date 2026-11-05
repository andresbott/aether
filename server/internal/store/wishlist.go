package store

import (
	"errors"
	"strings"
	"time"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm/clause"
)

// ErrInvalidWishlistKind is returned for a kind outside model.WishlistKinds.
var ErrInvalidWishlistKind = errors.New("invalid wishlist kind")

// wishlistMatchChunk bounds the IN list of one catalog lookup, well under
// SQLite's bound-variable limit.
const wishlistMatchChunk = 500

// WishlistMatch is the catalog row an MBID resolves to: the artist, album or
// track id by kind, plus — for a song — the track's album.
type WishlistMatch struct {
	ID      uint
	AlbumID uint
}

// WishlistReconcileStats reports what one ReconcileWishlist pass changed.
type WishlistReconcileStats struct {
	// Matched counts entries that are in the catalog after the pass.
	Matched int
	// Found counts entries that were not matched before the pass and are now.
	Found int
	// Lost counts entries that were matched before the pass and are not now.
	Lost int
}

// ListWishlist returns every wishlist entry, newest first.
func (s *Store) ListWishlist() ([]model.WishlistItem, error) {
	var items []model.WishlistItem
	err := s.db.Order("created_at DESC, id DESC").Find(&items).Error
	return items, err
}

// GetWishlistItem returns one entry, ErrNotFound when there is none.
func (s *Store) GetWishlistItem(id uint) (*model.WishlistItem, error) {
	var it model.WishlistItem
	if err := s.db.First(&it, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &it, nil
}

// AddWishlistItems inserts the given entries and returns the stored rows in
// input order. Adding is idempotent: an entry whose (kind, mbid) is already on
// the list is returned as it is, its stored snapshot and note untouched. New
// entries are matched against the catalog straight away, so something already
// in the library shows as found without waiting for the next scan. An entry
// without an MBID (added by hand) is always inserted and never matched.
func (s *Store) AddWishlistItems(items []model.WishlistItem) ([]model.WishlistItem, error) {
	out := make([]model.WishlistItem, 0, len(items))
	err := s.Transaction(func(tx *Store) error {
		for _, in := range items {
			if !in.Kind.Valid() {
				return ErrInvalidWishlistKind
			}
			row := model.WishlistItem{
				Kind:           in.Kind,
				MBID:           strings.ToLower(strings.TrimSpace(in.MBID)),
				Title:          in.Title,
				Artist:         in.Artist,
				Album:          in.Album,
				Date:           in.Date,
				Type:           in.Type,
				Disambiguation: in.Disambiguation,
				Note:           in.Note,
			}
			res := tx.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				// Already listed: hand back the stored row.
				if err := tx.db.Where("kind = ? AND mbid = ?", row.Kind, row.MBID).First(&row).Error; err != nil {
					return notFound(err)
				}
				out = append(out, row)
				continue
			}
			if !row.Manual() {
				if err := tx.matchWishlistRows([]*model.WishlistItem{&row}); err != nil {
					return err
				}
			}
			out = append(out, row)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ErrWishlistNotManual is returned when an update would change the title or
// kind of a MusicBrainz pick: those are its MusicBrainz snapshot, and only an
// entry added by hand owns them.
var ErrWishlistNotManual = errors.New("only an entry added by hand can change its title or kind")

// WishlistUpdate is a partial update of an entry; a nil field is left as is.
// Note may change on any entry; Title and Kind only on one added by hand.
type WishlistUpdate struct {
	Note  *string
	Title *string
	Kind  *model.WishlistKind
}

// UpdateWishlistItem applies u to one entry and returns it as stored.
// ErrNotFound when there is none, ErrWishlistNotManual when u changes the
// title or kind of a MusicBrainz pick, ErrInvalidWishlistKind for an unknown
// kind.
func (s *Store) UpdateWishlistItem(id uint, u WishlistUpdate) (*model.WishlistItem, error) {
	var it model.WishlistItem
	if err := s.db.First(&it, id).Error; err != nil {
		return nil, notFound(err)
	}
	cols := map[string]any{}
	if u.Note != nil {
		cols["note"] = *u.Note
	}
	if (u.Title != nil || u.Kind != nil) && !it.Manual() {
		return nil, ErrWishlistNotManual
	}
	if u.Title != nil {
		cols["title"] = *u.Title
	}
	if u.Kind != nil && !u.Kind.Valid() {
		return nil, ErrInvalidWishlistKind
	}
	if u.Kind != nil {
		cols["kind"] = *u.Kind
	}
	if len(cols) == 0 {
		return &it, nil
	}
	res := s.db.Model(&it).Updates(cols)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		// Deleted between the read and the write.
		return nil, ErrNotFound
	}
	return &it, nil
}

// DeleteWishlistItem removes one entry, ErrNotFound when there is none.
func (s *Store) DeleteWishlistItem(id uint) error {
	res := s.db.Delete(&model.WishlistItem{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// WishlistIDsByMBID maps each of the given MBIDs that is on the wishlist as
// kind to its entry id.
func (s *Store) WishlistIDsByMBID(kind model.WishlistKind, mbids []string) (map[string]uint, error) {
	out := make(map[string]uint)
	for _, chunk := range chunkStrings(nonEmptyStrings(mbids), wishlistMatchChunk) {
		var rows []model.WishlistItem
		if err := s.db.Select("id", "mbid").Where("kind = ? AND mbid IN ?", kind, chunk).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.MBID] = r.ID
		}
	}
	return out, nil
}

// WishlistCatalogMatches resolves MBIDs of the given kind to the catalog rows
// carrying them: artists by mb_artist_id, albums by mb_release_group_id, songs
// by tracks.mb_recording_id. An MBID several rows share (two editions of one
// release group, the same recording on two albums) resolves to the oldest row,
// so the answer is stable across scans that do not touch it. MBIDs with no
// match are absent from the map.
//
// The catalog keeps MBIDs exactly as the tags spell them, and some taggers
// write them uppercase, so the match is case-insensitive (wishlist MBIDs are
// stored lowercase). Normalizing at scan time instead would re-key album and
// artist assets, which are keyed on those MBIDs. LOWER() forgoes the column
// index — a table scan once per scan pass, and per wishlist search, is cheap.
func (s *Store) WishlistCatalogMatches(kind model.WishlistKind, mbids []string) (map[string]WishlistMatch, error) {
	var table, column, albumColumn string
	switch kind {
	case model.WishlistArtist:
		table, column = "artists", "mb_artist_id"
	case model.WishlistAlbum:
		table, column = "albums", "mb_release_group_id"
	case model.WishlistSong:
		table, column, albumColumn = "tracks", "mb_recording_id", "album_id"
	default:
		return nil, ErrInvalidWishlistKind
	}
	cols := []string{"id", "LOWER(" + column + ") AS mbid"}
	if albumColumn != "" {
		cols = append(cols, albumColumn+" AS album_id")
	}

	out := make(map[string]WishlistMatch)
	lowered := nonEmptyStrings(mbids)
	for i, m := range lowered {
		lowered[i] = strings.ToLower(m)
	}
	for _, chunk := range chunkStrings(lowered, wishlistMatchChunk) {
		var rows []struct {
			ID      uint
			MBID    string `gorm:"column:mbid"`
			AlbumID uint
		}
		err := s.db.Table(table).Select(cols).
			Where("LOWER("+column+") IN ?", chunk).
			Order("id ASC").
			Scan(&rows).Error
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if _, seen := out[r.MBID]; seen {
				continue
			}
			out[r.MBID] = WishlistMatch{ID: r.ID, AlbumID: r.AlbumID}
		}
	}
	return out, nil
}

// ReconcileWishlist re-matches every wishlist entry against the catalog: an
// entry whose MBID is now in the library is marked found (pointing at the
// matching row), one whose match is gone is cleared. Run after every pass that
// writes the catalog, since catalog ids are not stable across scans.
func (s *Store) ReconcileWishlist() (WishlistReconcileStats, error) {
	var stats WishlistReconcileStats
	err := s.Transaction(func(tx *Store) error {
		var items []model.WishlistItem
		if err := tx.db.Find(&items).Error; err != nil {
			return err
		}
		ptrs := make([]*model.WishlistItem, len(items))
		before := make([]uint, len(items))
		for i := range items {
			ptrs[i] = &items[i]
			before[i] = items[i].MatchID
		}
		if err := tx.matchWishlistRows(ptrs); err != nil {
			return err
		}
		for i, it := range items {
			switch {
			case before[i] == 0 && it.MatchID != 0:
				stats.Found++
			case before[i] != 0 && it.MatchID == 0:
				stats.Lost++
			}
			if it.MatchID != 0 {
				stats.Matched++
			}
		}
		return nil
	})
	return stats, err
}

// matchWishlistRows resolves each row's MBID against the catalog, writes the
// rows whose match changed, and updates the given structs to the stored state.
func (s *Store) matchWishlistRows(rows []*model.WishlistItem) error {
	byKind := make(map[model.WishlistKind][]string)
	for _, r := range rows {
		byKind[r.Kind] = append(byKind[r.Kind], r.MBID)
	}
	matches := make(map[model.WishlistKind]map[string]WishlistMatch, len(byKind))
	for kind, mbids := range byKind {
		m, err := s.WishlistCatalogMatches(kind, mbids)
		if err != nil {
			return err
		}
		matches[kind] = m
	}

	now := time.Now()
	for _, r := range rows {
		m := matches[r.Kind][strings.ToLower(r.MBID)]
		if m.ID == r.MatchID && m.AlbumID == r.MatchAlbumID {
			continue
		}
		matchedAt := r.MatchedAt
		switch {
		case m.ID == 0:
			matchedAt = nil
		case r.MatchID == 0:
			// Newly found. A match that only moved to another row (catalog ids
			// churn across scans) keeps the time it was first found.
			matchedAt = &now
		}
		err := s.db.Model(&model.WishlistItem{}).Where("id = ?", r.ID).
			UpdateColumns(map[string]any{
				"match_id":       m.ID,
				"match_album_id": m.AlbumID,
				"matched_at":     matchedAt,
			}).Error
		if err != nil {
			return err
		}
		r.MatchID, r.MatchAlbumID, r.MatchedAt = m.ID, m.AlbumID, matchedAt
	}
	return nil
}

func nonEmptyStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func chunkStrings(in []string, size int) [][]string {
	var out [][]string
	for len(in) > size {
		out = append(out, in[:size])
		in = in[size:]
	}
	if len(in) > 0 {
		out = append(out, in)
	}
	return out
}
