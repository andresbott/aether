package store

import (
	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm/clause"
)

func (s *Store) FindOrCreateGenres(names []string) ([]*model.Genre, error) {
	genres := make([]*model.Genre, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		var genre model.Genre
		err := s.db.Where("name = ?", name).First(&genre).Error
		if err != nil {
			genre = model.Genre{Name: name}
			if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&genre).Error; err != nil {
				return nil, err
			}
			s.db.Where("name = ?", name).First(&genre)
		}
		genres = append(genres, &genre)
	}
	return genres, nil
}

func (s *Store) GetGenre(id uint) (*model.Genre, error) {
	var genre model.Genre
	if err := s.db.First(&genre, id).Error; err != nil {
		return nil, err
	}
	return &genre, nil
}

type GenreWithCounts struct {
	model.Genre
	SongCount  int
	AlbumCount int
}

func (s *Store) GetGenres() ([]GenreWithCounts, error) {
	var genres []GenreWithCounts
	err := s.db.
		Table("genres").
		Select(`genres.*,
			(SELECT COUNT(*) FROM track_genres WHERE track_genres.genre_id = genres.id) AS song_count,
			(SELECT COUNT(DISTINCT album_genres.album_id) FROM album_genres WHERE album_genres.genre_id = genres.id) AS album_count`).
		Order("genres.name ASC").
		Scan(&genres).Error
	return genres, err
}
