package store

import (
	"errors"
	"strings"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/unidecode"
	"gorm.io/gorm"
)

func (s *Store) FindOrCreateArtists(names []string, mbids []string) (artists []*model.Artist, gained []*model.Artist, err error) {
	artists = make([]*model.Artist, 0, len(names))
	for i, name := range names {
		mbid := ""
		if i < len(mbids) {
			mbid = mbids[i]
		}
		norm := unidecode.Normalize(name)
		var artist model.Artist
		findErr := s.db.Where("name_norm = ?", norm).First(&artist).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			// A real DB failure must not be mistaken for "artist does not
			// exist" — creating a duplicate row on top of a transient error is
			// how a scan silently corrupts the artist index.
			return nil, nil, findErr
		}
		if findErr != nil {
			created, cErr := s.createArtist(name, norm, mbid)
			if cErr != nil {
				return nil, nil, cErr
			}
			artists = append(artists, created)
			continue
		}
		if mbid != "" && artist.MBArtistID != mbid {
			// Tag is source of truth: overwrite a differing (or previously
			// empty) MBID.
			oldMBID := artist.MBArtistID
			artist.MBArtistID = mbid
			if uErr := s.db.Model(&artist).Update("mb_artist_id", mbid).Error; uErr != nil {
				return nil, nil, uErr
			}
			// Report this artist as having gained an MBID only when the old
			// MBID was empty. An MBID change (old != "" && old != new) is
			// deliberately excluded: the MBID slot is shared with the auto-fetcher
			// and is content-addressed by the real-world artist. Moving stored
			// images from one MBID to another would misattribute the old artist's
			// portrait to the new artist, which is worse than stranding it.
			if oldMBID == "" {
				gained = append(gained, &artist)
			}
		}
		artists = append(artists, &artist)
	}
	return artists, gained, nil
}

// createArtist inserts a new artist row, recovering from the unique-index
// race a First-miss-then-Create by both a targeted RescanPaths and a
// scheduled scan could trigger. Kept as a cheap safety net: scan and reindex
// are serialized by the `library-writes` exclusion group, so they don't
// actually overlap. On that collision it re-reads and returns the winner's
// row rather than failing the whole track; any MBID divergence is transient
// and the next scan reconciles it.
func (s *Store) createArtist(name, norm, mbid string) (*model.Artist, error) {
	artist := model.Artist{Name: name, NameNorm: norm, MBArtistID: mbid}
	createErr := s.db.Create(&artist).Error
	if createErr == nil {
		return &artist, nil
	}
	if IsUniqueViolation(createErr) {
		var winner model.Artist
		if reErr := s.db.Where("name_norm = ?", norm).First(&winner).Error; reErr == nil {
			return &winner, nil
		}
	}
	return nil, createErr
}

// GetArtists returns the artist index: artists credited on at least one album
// (album_artists). Track-only credits — compilation contributors, featured
// guests — are deliberately excluded so no index entry ever shows zero albums;
// those artists stay reachable through search and song credits, and GetArtist
// resolves their appearances.
func (s *Store) GetArtists(filter *ArtistsFilter) ([]model.Artist, error) {
	q := s.db.Model(&model.Artist{}).
		Distinct().
		Joins("JOIN album_artists ON album_artists.artist_id = artists.id")
	if filter != nil && !filter.Scope.IsZero() {
		// Whether the scoped library hides its artists is the caller's call: a
		// scope carries no library identity (see the /rest artist index).
		q = scopeTracks(q.Joins("JOIN tracks ON tracks.album_id = album_artists.album_id"), filter.Scope)
	} else {
		// Unscoped: exclude artists that ONLY appear in hide-artists libraries.
		q = s.excludeHiddenArtists(q)
	}
	var artists []model.Artist
	err := q.Order("name_norm ASC").Find(&artists).Error
	return artists, err
}

func (s *Store) GetArtist(id uint) (*model.Artist, []model.Album, error) {
	var artist model.Artist
	if err := s.db.First(&artist, id).Error; err != nil {
		return nil, nil, notFound(err)
	}
	// Albums the artist owns (album_artists) plus albums they appear on via
	// track credits only — a guest artist's page must not come up empty.
	var albums []model.Album
	err := s.db.
		Preload("Artists").
		Preload("Genres").
		Where(`albums.id IN (SELECT album_id FROM album_artists WHERE artist_id = ?)
			OR albums.id IN (SELECT t.album_id FROM tracks t
				JOIN track_artists ta ON ta.track_id = t.id
				WHERE ta.artist_id = ?)`, id, id).
		Order("albums.year DESC, albums.name_norm ASC").
		Find(&albums).Error
	if err != nil {
		return nil, nil, err
	}
	return &artist, albums, nil
}

func (s *Store) GetArtistAlbumCounts(filter *ArtistsFilter) (map[uint]int, error) {
	type row struct {
		ArtistID uint
		Count    int
	}
	// Count both ownership credits (album_artists) and appearance credits
	// (track_artists → tracks → albums), matching what GetArtist returns.
	var rows []row
	credits := s.db.
		Table("album_artists").
		Select("album_artists.artist_id AS artist_id, album_artists.album_id AS album_id")
	appearances := s.db.
		Table("track_artists").
		Select("track_artists.artist_id AS artist_id, tracks.album_id AS album_id").
		Joins("JOIN tracks ON tracks.id = track_artists.track_id")
	if filter != nil && !filter.Scope.IsZero() {
		credits = scopeTracks(credits.Joins("JOIN tracks ON tracks.album_id = album_artists.album_id"), filter.Scope)
		appearances = scopeTracks(appearances, filter.Scope)
	}
	err := s.db.
		Table("(? UNION ?) AS credits", credits, appearances).
		Select("artist_id, COUNT(DISTINCT album_id) AS count").
		Group("artist_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]int, len(rows))
	for _, r := range rows {
		result[r.ArtistID] = r.Count
	}
	return result, nil
}

// SetArtistImagePath records the artist-folder image found on disk at scan time
// (empty string clears it).
func (s *Store) SetArtistImagePath(id uint, path string) error {
	return s.db.Model(&model.Artist{}).Where("id = ?", id).Update("image_path", path).Error
}

// SetArtistMBID sets the artist's MusicBrainz artist ID (empty string clears it).
func (s *Store) SetArtistMBID(id uint, mbid string) error {
	return s.db.Model(&model.Artist{}).Where("id = ?", id).
		Update("mb_artist_id", mbid).Error
}

func (s *Store) SearchArtists(query string, count, offset int, filter *SearchFilter) ([]model.Artist, error) {
	norm := unidecode.Normalize(query)
	q := s.db.Model(&model.Artist{}).Where("name_norm LIKE ?", "%"+norm+"%")
	if filter != nil && !filter.Scope.IsZero() {
		q = scopeTracks(q.
			Distinct().
			Joins("JOIN track_artists ON track_artists.artist_id = artists.id").
			Joins("JOIN tracks ON tracks.id = track_artists.track_id"), filter.Scope)
	}
	var artists []model.Artist
	err := q.Order("name_norm ASC").Limit(count).Offset(offset).Find(&artists).Error
	return artists, err
}

// excludeHiddenArtists drops artists whose entire presence (as track artist or
// album artist) falls inside libraries that hide their artists. An artist with
// at least one track outside every such library stays visible. No-op when no
// library hides its artists.
func (s *Store) excludeHiddenArtists(q *gorm.DB) *gorm.DB {
	hidden, err := s.hiddenArtistScopes()
	if err != nil || len(hidden) == 0 {
		return q
	}
	// A track counts as visible when it matches none of the hidden scopes.
	parts := make([]string, 0, len(hidden))
	var args []any
	for _, h := range hidden {
		sql, a := h.where("t")
		parts = append(parts, "NOT ("+sql+")")
		args = append(args, a...)
	}
	visible := strings.Join(parts, " AND ")
	// Check both track_artists (direct artist-track links) and album_artists
	// (artist → album → tracks). The predicate appears twice, so do its args.
	visiblePresence := `
		(EXISTS (
			SELECT 1 FROM track_artists ta
			JOIN tracks t ON ta.track_id = t.id
			WHERE ta.artist_id = artists.id AND ` + visible + `
		) OR EXISTS (
			SELECT 1 FROM album_artists aa
			JOIN tracks t ON aa.album_id = t.album_id
			WHERE aa.artist_id = artists.id AND ` + visible + `
		))
	`
	both := make([]any, 0, 2*len(args))
	both = append(both, args...)
	both = append(both, args...)
	return q.Where(visiblePresence, both...)
}

// hiddenArtistScopes returns the scope of every library that hides its artists.
func (s *Store) hiddenArtistScopes() ([]TrackScope, error) {
	var libs []model.Library
	if err := s.db.Where("hide_artists = ?", true).Find(&libs).Error; err != nil {
		return nil, err
	}
	scopes := make([]TrackScope, 0, len(libs))
	for i := range libs {
		scopes = append(scopes, LibraryScope(&libs[i]))
	}
	return scopes, nil
}
