package store

import (
	"sort"
	"strings"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/unidecode"
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

// genreCountsSelect is shared by GetGenres and SearchGenres so both report the
// same song/album counts for a genre. The counts are library-independent: a
// genre matched under a library filter still reports its whole footprint, as
// the genre pages do.
const genreCountsSelect = `genres.*,
	(SELECT COUNT(*) FROM track_genres WHERE track_genres.genre_id = genres.id) AS song_count,
	(SELECT COUNT(DISTINCT album_genres.album_id) FROM album_genres WHERE album_genres.genre_id = genres.id) AS album_count`

func (s *Store) GetGenres() ([]GenreWithCounts, error) {
	var genres []GenreWithCounts
	err := s.db.
		Table("genres").
		Select(genreCountsSelect).
		Order("genres.name ASC").
		Scan(&genres).Error
	return genres, err
}

// SearchGenres returns genres whose normalized name contains the query, using
// the same unidecode substring match as the artist/album/track searches. A
// LibraryID filter keeps only genres with at least one track in that library.
//
// Unlike the other searches this matches and sorts in Go rather than SQL: genres
// have no normalized column (see model.Genre), and there are few enough of them
// that GetGenres already loads the whole table on every genres-view load. Doing
// it here avoids a column that every rename would have to keep in step.
func (s *Store) SearchGenres(query string, count, offset int, filter *SearchFilter) ([]GenreWithCounts, error) {
	q := s.db.
		Table("genres").
		Select(genreCountsSelect)
	if filter != nil && filter.LibraryID != nil {
		q = q.Where(`EXISTS (SELECT 1 FROM track_genres
			JOIN tracks ON tracks.id = track_genres.track_id
			WHERE track_genres.genre_id = genres.id AND tracks.library_id = ?)`, *filter.LibraryID)
	}
	var all []GenreWithCounts
	if err := q.Scan(&all).Error; err != nil {
		return nil, err
	}

	norm := unidecode.Normalize(query)
	matches := make([]GenreWithCounts, 0, len(all))
	for _, g := range all {
		if strings.Contains(unidecode.Normalize(g.Name), norm) {
			matches = append(matches, g)
		}
	}
	// Order by the normalized name so paging is stable and matches the ordering
	// the other searches get from their name_norm index.
	sort.Slice(matches, func(i, j int) bool {
		return unidecode.Normalize(matches[i].Name) < unidecode.Normalize(matches[j].Name)
	})

	if offset >= len(matches) {
		return []GenreWithCounts{}, nil
	}
	matches = matches[offset:]
	if count >= 0 && count < len(matches) {
		matches = matches[:count]
	}
	return matches, nil
}
