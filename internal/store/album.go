package store

import (
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/unidecode"
	"gorm.io/gorm"
)

func (s *Store) FindOrCreateAlbum(name, albumArtistNorm, mbReleaseID string) (*model.Album, error) {
	nameNorm := unidecode.Normalize(name)
	var album model.Album
	err := s.db.Where("name_norm = ? AND album_artist_norm = ? AND mb_release_id = ?", nameNorm, albumArtistNorm, mbReleaseID).First(&album).Error
	if err != nil {
		album = model.Album{
			Name:            name,
			NameNorm:        nameNorm,
			AlbumArtistNorm: albumArtistNorm,
			MBReleaseID:     mbReleaseID,
		}
		if err := s.db.Create(&album).Error; err != nil {
			return nil, err
		}
	}
	return &album, nil
}

func (s *Store) GetAlbum(id uint) (*model.Album, error) {
	var album model.Album
	err := s.db.
		Preload("Artists").
		Preload("Genres").
		Preload("Tracks", func(db *gorm.DB) *gorm.DB {
			return db.Order("tracks.disc_number ASC, tracks.track_number ASC")
		}).
		Preload("Tracks.Artists").
		Preload("Tracks.Genres").
		First(&album, id).Error
	if err != nil {
		return nil, err
	}
	return &album, nil
}

type AlbumListFilter struct {
	Genre     string
	FromYear  int
	ToYear    int
	LibraryID *uint
}

func (s *Store) GetAlbumList(listType string, size, offset int, filter *AlbumListFilter) ([]model.Album, error) {
	q := s.db.Model(&model.Album{}).Preload("Artists").Preload("Genres")

	if filter != nil && filter.LibraryID != nil {
		q = q.Where("EXISTS (SELECT 1 FROM tracks WHERE tracks.album_id = albums.id AND tracks.library_id = ?)", *filter.LibraryID)
	}

	switch listType {
	case "alphabeticalByName":
		q = q.Order("name_norm ASC")
	case "alphabeticalByArtist":
		q = q.Order("album_artist_norm ASC, name_norm ASC")
	case "newest":
		q = q.Order("created_at DESC")
	case "byYear":
		if filter != nil {
			if filter.FromYear > 0 {
				q = q.Where("year >= ?", filter.FromYear)
			}
			if filter.ToYear > 0 {
				q = q.Where("year <= ?", filter.ToYear)
			}
		}
		q = q.Order("year DESC")
	case "byGenre":
		if filter != nil && filter.Genre != "" {
			q = q.Joins("JOIN album_genres ON album_genres.album_id = albums.id").
				Joins("JOIN genres ON genres.id = album_genres.genre_id").
				Where("genres.name = ?", filter.Genre)
		}
		q = q.Order("name_norm ASC")
	case "random":
		q = q.Order("RANDOM()")
	case "starred":
		q = q.Joins("JOIN starred_items ON starred_items.item_id = albums.id AND starred_items.item_type = 'album'").
			Order("starred_items.created_at DESC")
	case "frequent":
		q = q.Joins("JOIN tracks ON tracks.album_id = albums.id").
			Joins("JOIN play_histories ON play_histories.track_id = tracks.id").
			Group("albums.id").
			Order("COUNT(play_histories.id) DESC")
	case "recent":
		q = q.Joins("JOIN tracks ON tracks.album_id = albums.id").
			Joins("JOIN play_histories ON play_histories.track_id = tracks.id").
			Group("albums.id").
			Order("MAX(play_histories.played_at) DESC")
	default:
		q = q.Order("name_norm ASC")
	}

	var albums []model.Album
	err := q.Limit(size).Offset(offset).Find(&albums).Error
	return albums, err
}

func (s *Store) SearchAlbums(query string, count, offset int, filter *SearchFilter) ([]model.Album, error) {
	norm := unidecode.Normalize(query)
	q := s.db.
		Preload("Artists").
		Where("name_norm LIKE ?", "%"+norm+"%")
	if filter != nil && filter.LibraryID != nil {
		q = q.Where("EXISTS (SELECT 1 FROM tracks WHERE tracks.album_id = albums.id AND tracks.library_id = ?)", *filter.LibraryID)
	}
	var albums []model.Album
	err := q.Order("name_norm ASC").Limit(count).Offset(offset).Find(&albums).Error
	return albums, err
}
