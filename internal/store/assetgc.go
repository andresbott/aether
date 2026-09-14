package store

import "github.com/andresbott/aether/internal/model"

// AssetIdentities is every entity that owns a cover, in the shape the
// orphan-cover GC needs to recompute its asset keys (see internal/assetkey). It
// loads full rows rather than narrow projections: the prune task runs
// occasionally in the background, not on a request path, so the extra columns
// are cheap and the code stays free of column-name coupling.
type AssetIdentities struct {
	Albums    []model.Album
	Artists   []model.Artist
	Genres    []model.Genre
	Playlists []model.Playlist
	Radios    []model.InternetRadioStation
}

// AssetIdentities returns every album, artist, genre, playlist and radio
// station so the prune task can build the set of live asset keys and drop cover
// derivatives that belong to none of them.
func (s *Store) AssetIdentities() (AssetIdentities, error) {
	var ids AssetIdentities
	if err := s.db.Find(&ids.Albums).Error; err != nil {
		return AssetIdentities{}, err
	}
	if err := s.db.Find(&ids.Artists).Error; err != nil {
		return AssetIdentities{}, err
	}
	if err := s.db.Find(&ids.Genres).Error; err != nil {
		return AssetIdentities{}, err
	}
	if err := s.db.Find(&ids.Playlists).Error; err != nil {
		return AssetIdentities{}, err
	}
	if err := s.db.Find(&ids.Radios).Error; err != nil {
		return AssetIdentities{}, err
	}
	return ids, nil
}
