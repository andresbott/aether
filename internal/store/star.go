package store

import (
	"time"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm/clause"
)

func (s *Store) Star(owner, itemType string, itemID uint) error {
	item := model.StarredItem{Owner: owner, ItemType: itemType, ItemID: itemID}
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error
}

func (s *Store) Unstar(owner, itemType string, itemID uint) error {
	return s.db.Where("owner = ? AND item_type = ? AND item_id = ?", owner, itemType, itemID).Delete(&model.StarredItem{}).Error
}

type StarredResult struct {
	Artists   []model.Artist
	Albums    []model.Album
	Tracks    []model.Track
	Playlists []model.Playlist
}

func (s *Store) GetStarred(owner string, filter *StarredFilter) (*StarredResult, error) {
	result := &StarredResult{}

	var err error
	result.Artists, err = s.starredArtists(owner, filter)
	if err != nil {
		return nil, err
	}

	result.Albums, err = s.starredAlbums(owner, filter)
	if err != nil {
		return nil, err
	}

	result.Tracks, err = s.starredTracks(owner, filter)
	if err != nil {
		return nil, err
	}

	result.Playlists, err = s.starredPlaylists(owner)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Store) starredArtists(owner string, filter *StarredFilter) ([]model.Artist, error) {
	var artistIDs []uint
	if err := s.db.Model(&model.StarredItem{}).Where("owner = ? AND item_type = 'artist'", owner).Pluck("item_id", &artistIDs).Error; err != nil {
		return nil, err
	}
	if len(artistIDs) == 0 {
		return []model.Artist{}, nil
	}

	q := s.db.Model(&model.Artist{}).Where("artists.id IN ?", artistIDs)
	if filter != nil && filter.LibraryID != nil {
		q = q.
			Distinct().
			Joins("JOIN track_artists ON track_artists.artist_id = artists.id").
			Joins("JOIN tracks ON tracks.id = track_artists.track_id").
			Where("tracks.library_id = ?", *filter.LibraryID)
	}

	var artists []model.Artist
	// Same name_norm ASC order as GetArtists: the favorites list is rendered by
	// the same views as the full library, whose alphabet rail assumes it.
	if err := q.Order("name_norm ASC").Find(&artists).Error; err != nil {
		return nil, err
	}
	return artists, nil
}

func (s *Store) starredAlbums(owner string, filter *StarredFilter) ([]model.Album, error) {
	var albumIDs []uint
	if err := s.db.Model(&model.StarredItem{}).Where("owner = ? AND item_type = 'album'", owner).Pluck("item_id", &albumIDs).Error; err != nil {
		return nil, err
	}
	if len(albumIDs) == 0 {
		return []model.Album{}, nil
	}

	q := s.db.Preload("Artists").Where("albums.id IN ?", albumIDs)
	if filter != nil && filter.LibraryID != nil {
		q = q.Where("EXISTS (SELECT 1 FROM tracks WHERE tracks.album_id = albums.id AND tracks.library_id = ?)", *filter.LibraryID)
	}

	var albums []model.Album
	// Same name_norm ASC order as GetAlbumList's alphabeticalByName, for the same
	// reason as starredArtists.
	if err := q.Order("name_norm ASC").Find(&albums).Error; err != nil {
		return nil, err
	}
	return albums, nil
}

func (s *Store) starredTracks(owner string, filter *StarredFilter) ([]model.Track, error) {
	var trackIDs []uint
	if err := s.db.Model(&model.StarredItem{}).Where("owner = ? AND item_type = 'track'", owner).Pluck("item_id", &trackIDs).Error; err != nil {
		return nil, err
	}
	if len(trackIDs) == 0 {
		return []model.Track{}, nil
	}

	q := s.db.Preload("Album").Preload("Album.Artists").Preload("Artists").Preload("Genres").Where("tracks.id IN ?", trackIDs)
	if filter != nil && filter.LibraryID != nil {
		q = q.Where("tracks.library_id = ?", *filter.LibraryID)
	}

	var tracks []model.Track
	if err := q.Find(&tracks).Error; err != nil {
		return nil, err
	}
	return tracks, nil
}

func (s *Store) starredPlaylists(owner string) ([]model.Playlist, error) {
	// Playlists are not scoped to a library — a playlist can hold tracks from
	// several — so StarredFilter.LibraryID deliberately does not apply here.
	var playlistStars []model.StarredItem
	if err := s.db.Where("owner = ? AND item_type = 'playlist'", owner).
		Order("created_at DESC").
		Find(&playlistStars).Error; err != nil {
		return nil, err
	}
	if len(playlistStars) == 0 {
		return []model.Playlist{}, nil
	}

	playlistIDs := make([]uint, 0, len(playlistStars))
	for _, st := range playlistStars {
		playlistIDs = append(playlistIDs, st.ItemID)
	}

	var playlists []model.Playlist
	if err := s.db.Where("id IN ?", playlistIDs).Find(&playlists).Error; err != nil {
		return nil, err
	}

	byID := make(map[uint]model.Playlist, len(playlists))
	for _, pl := range playlists {
		byID[pl.ID] = pl
	}

	// Re-apply the star order the IN query lost, skipping stars whose
	// playlist no longer exists.
	result := make([]model.Playlist, 0, len(playlists))
	for _, id := range playlistIDs {
		if pl, ok := byID[id]; ok {
			result = append(result, pl)
		}
	}
	return result, nil
}

// StarredAt returns when each of the given items of itemType was starred, in one
// query. Items that are not starred are absent from the map. Ids are only unique
// per type, so itemType is part of the lookup — never drop it. The lookup is
// keyed by owner AND type AND id.
func (s *Store) StarredAt(owner, itemType string, itemIDs []uint) (map[uint]time.Time, error) {
	out := map[uint]time.Time{}
	if len(itemIDs) == 0 {
		return out, nil
	}
	var stars []model.StarredItem
	if err := s.db.
		Where("owner = ? AND item_type = ? AND item_id IN ?", owner, itemType, itemIDs).
		Find(&stars).Error; err != nil {
		return nil, err
	}
	for _, st := range stars {
		out[st.ItemID] = st.CreatedAt
	}
	return out, nil
}
