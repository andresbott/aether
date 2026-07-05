package store

import (
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/unidecode"
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
		if err != nil {
			artist = model.Artist{Name: name, NameNorm: norm, MBArtistID: mbid}
			if err := s.db.Create(&artist).Error; err != nil {
				return nil, err
			}
		} else if artist.MBArtistID == "" && mbid != "" {
			artist.MBArtistID = mbid
			if err := s.db.Model(&artist).Update("mb_artist_id", mbid).Error; err != nil {
				return nil, err
			}
		}
		artists = append(artists, &artist)
	}
	return artists, nil
}

func (s *Store) GetArtists(filter *ArtistsFilter) ([]model.Artist, error) {
	q := s.db.Model(&model.Artist{})
	if filter != nil && filter.LibraryID != nil {
		q = q.
			Distinct().
			Joins("JOIN track_artists ON track_artists.artist_id = artists.id").
			Joins("JOIN tracks ON tracks.id = track_artists.track_id").
			Where("tracks.library_id = ?", *filter.LibraryID)
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
	var albums []model.Album
	err := s.db.
		Preload("Artists").
		Preload("Genres").
		Joins("JOIN album_artists ON album_artists.album_id = albums.id").
		Where("album_artists.artist_id = ?", id).
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
	var rows []row
	q := s.db.
		Table("album_artists").
		Select("album_artists.artist_id AS artist_id, COUNT(DISTINCT album_artists.album_id) AS count")
	if filter != nil && filter.LibraryID != nil {
		q = q.
			Joins("JOIN tracks ON tracks.album_id = album_artists.album_id").
			Where("tracks.library_id = ?", *filter.LibraryID)
	}
	err := q.Group("album_artists.artist_id").Find(&rows).Error
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

func (s *Store) SetArtistImageFetchedAt(id uint, t time.Time) error {
	return s.db.Model(&model.Artist{}).Where("id = ?", id).Update("last_image_fetch_at", t).Error
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
