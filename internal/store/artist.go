package store

import (
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/unidecode"
)

func (s *Store) FindOrCreateArtists(names []string) ([]*model.Artist, error) {
	artists := make([]*model.Artist, 0, len(names))
	for _, name := range names {
		norm := unidecode.Normalize(name)
		var artist model.Artist
		err := s.db.Where("name_norm = ?", norm).First(&artist).Error
		if err != nil {
			artist = model.Artist{Name: name, NameNorm: norm}
			if err := s.db.Create(&artist).Error; err != nil {
				return nil, err
			}
		}
		artists = append(artists, &artist)
	}
	return artists, nil
}

func (s *Store) GetArtists() ([]model.Artist, error) {
	var artists []model.Artist
	err := s.db.Order("name_norm ASC").Find(&artists).Error
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

func (s *Store) GetArtistAlbumCounts() (map[uint]int, error) {
	type row struct {
		ArtistID uint
		Count    int
	}
	var rows []row
	err := s.db.
		Table("album_artists").
		Select("artist_id, COUNT(*) as count").
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

func (s *Store) SearchArtists(query string, count, offset int) ([]model.Artist, error) {
	var artists []model.Artist
	norm := unidecode.Normalize(query)
	err := s.db.
		Where("name_norm LIKE ?", "%"+norm+"%").
		Order("name_norm ASC").
		Limit(count).
		Offset(offset).
		Find(&artists).Error
	return artists, err
}
