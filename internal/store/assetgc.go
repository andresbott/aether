package store

import "github.com/andresbott/aether/internal/model"

// AssetIdentities is every entity that owns a cover, in the shape the
// orphan-cover GC needs to recompute its asset keys (see internal/assetkey).
// AssetIdentities loads only the identity columns each assetkey.*Of reads, not
// full rows, so the background sweep's memory stays bounded on a large library.
// The selected columns ARE the inputs to key derivation and must track
// internal/assetkey: albums key on (name_norm, album_artist_norm, mb_release_id),
// artists on (mb_artist_id, name_norm), genres on name, playlists on uuid, radios
// on stream_url.
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
	if err := s.db.Select("name_norm", "album_artist_norm", "mb_release_id").Find(&ids.Albums).Error; err != nil {
		return AssetIdentities{}, err
	}
	if err := s.db.Select("mb_artist_id", "name_norm").Find(&ids.Artists).Error; err != nil {
		return AssetIdentities{}, err
	}
	if err := s.db.Select("name").Find(&ids.Genres).Error; err != nil {
		return AssetIdentities{}, err
	}
	if err := s.db.Select("uuid").Find(&ids.Playlists).Error; err != nil {
		return AssetIdentities{}, err
	}
	if err := s.db.Select("stream_url").Find(&ids.Radios).Error; err != nil {
		return AssetIdentities{}, err
	}
	return ids, nil
}
