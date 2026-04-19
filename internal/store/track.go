package store

import (
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/unidecode"
)

func (s *Store) UpsertTrack(track *model.Track, artists []*model.Artist, genres []*model.Genre) error {
	if err := s.db.Save(track).Error; err != nil {
		return err
	}
	if err := s.db.Model(track).Association("Artists").Replace(artists); err != nil {
		return err
	}
	if err := s.db.Model(track).Association("Genres").Replace(genres); err != nil {
		return err
	}
	return nil
}

func (s *Store) GetSong(id uint) (*model.Track, error) {
	var track model.Track
	err := s.db.
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("Genres").
		First(&track, id).Error
	if err != nil {
		return nil, err
	}
	return &track, nil
}

type RandomSongsFilter struct {
	Genre    string
	FromYear int
	ToYear   int
}

func (s *Store) GetRandomSongs(size int, filter *RandomSongsFilter) ([]model.Track, error) {
	q := s.db.Model(&model.Track{}).
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("Genres")

	if filter != nil {
		if filter.Genre != "" {
			q = q.Joins("JOIN track_genres ON track_genres.track_id = tracks.id").
				Joins("JOIN genres ON genres.id = track_genres.genre_id").
				Where("genres.name = ?", filter.Genre)
		}
		if filter.FromYear > 0 {
			q = q.Where("tracks.year >= ?", filter.FromYear)
		}
		if filter.ToYear > 0 {
			q = q.Where("tracks.year <= ?", filter.ToYear)
		}
	}

	var tracks []model.Track
	err := q.Order("RANDOM()").Limit(size).Find(&tracks).Error
	return tracks, err
}

func (s *Store) GetSongsByGenre(genre string, count, offset int) ([]model.Track, error) {
	var tracks []model.Track
	err := s.db.
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("Genres").
		Joins("JOIN track_genres ON track_genres.track_id = tracks.id").
		Joins("JOIN genres ON genres.id = track_genres.genre_id").
		Where("genres.name = ?", genre).
		Limit(count).
		Offset(offset).
		Find(&tracks).Error
	return tracks, err
}

func (s *Store) SearchSongs(query string, count, offset int) ([]model.Track, error) {
	norm := unidecode.Normalize(query)
	var tracks []model.Track
	err := s.db.
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("Genres").
		Where("title_norm LIKE ?", "%"+norm+"%").
		Order("title_norm ASC").
		Limit(count).
		Offset(offset).
		Find(&tracks).Error
	return tracks, err
}

func (s *Store) GetTrackFilePath(id uint) (string, error) {
	var track model.Track
	err := s.db.Select("file_path").First(&track, id).Error
	if err != nil {
		return "", err
	}
	return track.FilePath, nil
}

func (s *Store) GetCoverTrackPath(albumID uint) (string, error) {
	var track model.Track
	err := s.db.
		Select("file_path").
		Where("album_id = ? AND has_embedded_cover = ?", albumID, true).
		First(&track).Error
	if err != nil {
		return "", err
	}
	return track.FilePath, nil
}
