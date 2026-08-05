package store

import (
	"path/filepath"
	"strings"

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

// GetAlbumByTrackPath resolves the album a track belongs to from its absolute
// file path. Used by the metadata editor, which works by file path, to find the
// DB album id for cover storage/serving.
func (s *Store) GetAlbumByTrackPath(absPath string) (*model.Album, error) {
	var track model.Track
	if err := s.db.Select("album_id").Where("file_path = ?", absPath).First(&track).Error; err != nil {
		return nil, err
	}
	return s.GetAlbum(track.AlbumID)
}

// SetAlbumCoverPath sets the album's external cover file path.
func (s *Store) SetAlbumCoverPath(albumID uint, path string) error {
	return s.db.Model(&model.Album{}).Where("id = ?", albumID).Update("cover_path", path).Error
}

// GetAlbumByTrackDir resolves the album of any track located directly in absDir.
// Used by the metadata editor to show the same cover the app serves for a folder,
// without depending on tag-reading. Returns gorm.ErrRecordNotFound when the folder
// has no scanned track.
func (s *Store) GetAlbumByTrackDir(absDir string) (*model.Album, error) {
	var track model.Track
	// Trailing separator via Join keeps "album/" from also matching "album2/".
	like := filepath.Join(absDir, "%")
	if err := s.db.Select("album_id").Where("file_path LIKE ?", like).First(&track).Error; err != nil {
		return nil, err
	}
	var album model.Album
	if err := s.db.First(&album, track.AlbumID).Error; err != nil {
		return nil, err
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
	Owner     string
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
			Where("starred_items.owner = ?", ownerOrAdmin(filter)).
			Order("starred_items.created_at DESC")
	case "frequent":
		q = q.Joins("JOIN tracks ON tracks.album_id = albums.id").
			Joins("JOIN play_histories ON play_histories.track_id = tracks.id").
			Where("play_histories.owner = ?", ownerOrAdmin(filter)).
			Group("albums.id").
			Order("COUNT(play_histories.id) DESC")
	case "recent":
		q = q.Joins("JOIN tracks ON tracks.album_id = albums.id").
			Joins("JOIN play_histories ON play_histories.track_id = tracks.id").
			Where("play_histories.owner = ?", ownerOrAdmin(filter)).
			Group("albums.id").
			Order("MAX(play_histories.played_at) DESC")
	default:
		q = q.Order("name_norm ASC")
	}

	var albums []model.Album
	err := q.Limit(size).Offset(offset).Find(&albums).Error
	return albums, err
}

// AlbumLetter is one bucket of the alphabetical album index: the first-letter
// label, the offset of its first album in alphabeticalByName order, and how
// many albums fall under it.
type AlbumLetter struct {
	Letter string // "#" or "A".."Z"
	Offset int
	Count  int
}

// GetAlbumLetterIndex returns per-letter offsets/counts for the alphabeticalByName
// album ordering (same LibraryID filter and name_norm ASC order as GetAlbumList),
// plus the total album count. Non-alphabetic first chars bucket under "#".
func (s *Store) GetAlbumLetterIndex(filter *AlbumListFilter) ([]AlbumLetter, int, error) {
	q := s.db.Model(&model.Album{})
	if filter != nil && filter.LibraryID != nil {
		q = q.Where("EXISTS (SELECT 1 FROM tracks WHERE tracks.album_id = albums.id AND tracks.library_id = ?)", *filter.LibraryID)
	}

	type row struct {
		C string
		N int
	}
	var rows []row
	if err := q.
		Select("SUBSTR(name_norm, 1, 1) AS c, COUNT(*) AS n").
		Group("c").
		Order("c ASC").
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	var letters []AlbumLetter
	byLetter := map[string]int{} // letter -> index in letters
	running := 0
	for _, r := range rows {
		letter := "#"
		if len(r.C) == 1 && r.C >= "a" && r.C <= "z" {
			letter = strings.ToUpper(r.C)
		}
		if idx, ok := byLetter[letter]; ok {
			letters[idx].Count += r.N
		} else {
			byLetter[letter] = len(letters)
			letters = append(letters, AlbumLetter{Letter: letter, Offset: running, Count: r.N})
		}
		running += r.N
	}
	return letters, running, nil
}

// AlbumTrackStat holds aggregate track figures for one album.
type AlbumTrackStat struct {
	AlbumID  uint
	Count    int
	Duration int
}

// AlbumTrackStats returns song count and total duration per album for the given
// album IDs, in one grouped query. Albums with no tracks are absent from the map.
func (s *Store) AlbumTrackStats(albumIDs []uint) (map[uint]AlbumTrackStat, error) {
	out := map[uint]AlbumTrackStat{}
	if len(albumIDs) == 0 {
		return out, nil
	}
	var rows []AlbumTrackStat
	if err := s.db.Model(&model.Track{}).
		Select("album_id AS album_id, COUNT(*) AS count, COALESCE(SUM(duration), 0) AS duration").
		Where("album_id IN ?", albumIDs).
		Group("album_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.AlbumID] = r
	}
	return out, nil
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

// ownerOrAdmin returns the filter's owner, defaulting to "admin" — the fixed
// single-user owner used when no identity layer is active.
func ownerOrAdmin(filter *AlbumListFilter) string {
	if filter != nil && filter.Owner != "" {
		return filter.Owner
	}
	return "admin"
}
