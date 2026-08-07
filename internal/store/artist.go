package store

import (
	"errors"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/unidecode"
	"gorm.io/gorm"
)

func (s *Store) FindOrCreateArtists(names []string, mbids []string) ([]*model.Artist, error) {
	artists := make([]*model.Artist, 0, len(names))
	for i, name := range names {
		mbid := ""
		if i < len(mbids) {
			mbid = mbids[i]
		}
		norm := unidecode.Normalize(name)
		var artist model.Artist
		err := s.db.Where("name_norm = ?", norm).First(&artist).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			// A real DB failure must not be mistaken for "artist does not
			// exist" — creating a duplicate row on top of a transient error is
			// how a scan silently corrupts the artist index.
			return nil, err
		}
		if err != nil {
			artist = model.Artist{Name: name, NameNorm: norm, MBArtistID: mbid}
			if err := s.db.Create(&artist).Error; err != nil {
				return nil, err
			}
		} else if mbid != "" && artist.MBArtistID != mbid {
			// Tag is source of truth: overwrite a differing (or previously
			// empty) MBID and reset the image-fetch timestamp so the artist
			// image is refetched for the corrected match.
			artist.MBArtistID = mbid
			artist.LastImageFetchAt = nil
			if err := s.db.Model(&artist).Updates(map[string]interface{}{
				"mb_artist_id":        mbid,
				"last_image_fetch_at": nil,
			}).Error; err != nil {
				return nil, err
			}
		}
		artists = append(artists, &artist)
	}
	return artists, nil
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
	if filter != nil && filter.LibraryID != nil {
		// Check if this specific library is hidden
		var lib model.Library
		if err := s.db.First(&lib, *filter.LibraryID).Error; err == nil && lib.HideArtists {
			// Return empty result for hidden libraries
			return []model.Artist{}, nil
		}
		q = q.
			Joins("JOIN tracks ON tracks.album_id = album_artists.album_id").
			Where("tracks.library_id = ?", *filter.LibraryID)
	} else {
		// No library filter: exclude artists that ONLY appear in hidden libraries
		q = s.excludeHiddenArtists(q)
	}
	var artists []model.Artist
	err := q.Order("name_norm ASC").Find(&artists).Error
	return artists, err
}

func (s *Store) GetArtist(id uint) (*model.Artist, []model.Album, error) {
	var artist model.Artist
	if err := s.db.First(&artist, id).Error; err != nil {
		return nil, nil, err
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
	if filter != nil && filter.LibraryID != nil {
		credits = credits.
			Joins("JOIN tracks ON tracks.album_id = album_artists.album_id").
			Where("tracks.library_id = ?", *filter.LibraryID)
		appearances = appearances.Where("tracks.library_id = ?", *filter.LibraryID)
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

func (s *Store) ArtistsWithMBID() ([]model.Artist, error) {
	var artists []model.Artist
	err := s.db.Where("mb_artist_id != ''").Find(&artists).Error
	return artists, err
}

// SetArtistImagePath records the artist-folder image found on disk at scan time
// (empty string clears it).
func (s *Store) SetArtistImagePath(id uint, path string) error {
	return s.db.Model(&model.Artist{}).Where("id = ?", id).Update("image_path", path).Error
}

func (s *Store) SetArtistImageFetchedAt(id uint, t time.Time) error {
	return s.db.Model(&model.Artist{}).Where("id = ?", id).Update("last_image_fetch_at", t).Error
}

// SetArtistMBID sets the artist's MusicBrainz artist ID (empty string
// clears it) and always resets LastImageFetchAt to nil, so a changed match
// is retried on the next fetch attempt instead of hitting the backoff.
func (s *Store) SetArtistMBID(id uint, mbid string) error {
	return s.db.Model(&model.Artist{}).Where("id = ?", id).
		Updates(map[string]interface{}{"mb_artist_id": mbid, "last_image_fetch_at": nil}).Error
}

func (s *Store) SearchArtists(query string, count, offset int, filter *SearchFilter) ([]model.Artist, error) {
	norm := unidecode.Normalize(query)
	q := s.db.Model(&model.Artist{}).Where("name_norm LIKE ?", "%"+norm+"%")
	if filter != nil && filter.LibraryID != nil {
		q = q.
			Distinct().
			Joins("JOIN track_artists ON track_artists.artist_id = artists.id").
			Joins("JOIN tracks ON tracks.id = track_artists.track_id").
			Where("tracks.library_id = ?", *filter.LibraryID)
	}
	var artists []model.Artist
	err := q.Order("name_norm ASC").Limit(count).Offset(offset).Find(&artists).Error
	return artists, err
}

// excludeHiddenArtists drops artists whose entire library presence (as track
// artist or album artist) sits in libraries with hide_artists = true. An
// artist with at least one track in a visible library stays visible.
// No-op when no library is hidden.
func (s *Store) excludeHiddenArtists(q *gorm.DB) *gorm.DB {
	var hidden []uint
	if err := s.db.Model(&model.Library{}).
		Where("hide_artists = ?", true).
		Pluck("id", &hidden).Error; err != nil || len(hidden) == 0 {
		return q
	}
	// Artist is visible if it has at least one track in a visible library.
	// We exclude artists that ONLY have tracks in hidden libraries.
	// Check both track_artists (direct artist-track links) and album_artists
	// (artist → album → tracks).
	visiblePresence := `
		(EXISTS (
			SELECT 1 FROM track_artists ta
			JOIN tracks t ON ta.track_id = t.id
			WHERE ta.artist_id = artists.id AND t.library_id NOT IN (?)
		) OR EXISTS (
			SELECT 1 FROM album_artists aa
			JOIN tracks t ON aa.album_id = t.album_id
			WHERE aa.artist_id = artists.id AND t.library_id NOT IN (?)
		))
	`
	return q.Where(visiblePresence, hidden, hidden)
}
