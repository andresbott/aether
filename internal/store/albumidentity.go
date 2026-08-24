package store

import (
	"errors"
	"strings"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

// chunkSize bounds the IN-clause of the store's bulk lookups: FilterChanged
// and BulkUpdateLastSeen (scan_helpers.go) and the album/track identity lookups
// all page long path/id slices through it.
const chunkSize = 500

// AlbumIdentity is the tuple that decides which album row a track belongs to,
// plus the display name written alongside it.
//
// Name is deliberately NOT part of the comparable Key: name_norm is what
// idx_album_identity covers, so a case-or-accent-only edit is the same album.
type AlbumIdentity struct {
	Name            string
	NameNorm        string
	AlbumArtistNorm string
	MBReleaseID     string
}

// AlbumIdentityKey is the comparable part of an AlbumIdentity — exactly the
// columns of the idx_album_identity unique index. Usable as a map key.
type AlbumIdentityKey struct {
	NameNorm        string
	AlbumArtistNorm string
	MBReleaseID     string
}

func (a AlbumIdentity) Key() AlbumIdentityKey {
	return AlbumIdentityKey{
		NameNorm:        a.NameNorm,
		AlbumArtistNorm: a.AlbumArtistNorm,
		MBReleaseID:     a.MBReleaseID,
	}
}

// TrackAlbumIDs maps each of paths the DB already knows to the album row it
// currently belongs to. Paths with no track row, and tracks with no album, are
// absent from the result rather than mapped to zero.
func (s *Store) TrackAlbumIDs(paths []string) (map[string]uint, error) {
	type row struct {
		FilePath string `gorm:"column:file_path"`
		AlbumID  uint   `gorm:"column:album_id"`
	}
	out := make(map[string]uint, len(paths))
	for i := 0; i < len(paths); i += chunkSize {
		end := i + chunkSize
		if end > len(paths) {
			end = len(paths)
		}
		var rows []row
		if err := s.db.Table("tracks").
			Select("file_path, album_id").
			Where("file_path IN ?", paths[i:end]).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			if r.AlbumID != 0 {
				out[r.FilePath] = r.AlbumID
			}
		}
	}
	return out, nil
}

// AlbumTrackCounts reports how many tracks each of ids currently holds. An
// album with no tracks is absent from the map.
func (s *Store) AlbumTrackCounts(ids []uint) (map[uint]int, error) {
	type row struct {
		AlbumID uint `gorm:"column:album_id"`
		N       int  `gorm:"column:n"`
	}
	out := make(map[uint]int, len(ids))
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		var rows []row
		if err := s.db.Table("tracks").
			Select("album_id, COUNT(*) AS n").
			Where("album_id IN ?", ids[i:end]).
			Group("album_id").
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.AlbumID] = r.N
		}
	}
	return out, nil
}

// AlbumIdentities reports the identity each of ids currently carries.
func (s *Store) AlbumIdentities(ids []uint) (map[uint]AlbumIdentity, error) {
	type row struct {
		ID              uint   `gorm:"column:id"`
		Name            string `gorm:"column:name"`
		NameNorm        string `gorm:"column:name_norm"`
		AlbumArtistNorm string `gorm:"column:album_artist_norm"`
		MBReleaseID     string `gorm:"column:mb_release_id"`
	}
	out := make(map[uint]AlbumIdentity, len(ids))
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		var rows []row
		if err := s.db.Table("albums").
			Select("id, name, name_norm, album_artist_norm, mb_release_id").
			Where("id IN ?", ids[i:end]).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.ID] = AlbumIdentity{
				Name:            r.Name,
				NameNorm:        r.NameNorm,
				AlbumArtistNorm: r.AlbumArtistNorm,
				MBReleaseID:     r.MBReleaseID,
			}
		}
	}
	return out, nil
}

// AlbumIDForIdentity returns the id of the album holding key, or 0 when the
// identity is free. Mirrors FindOrCreateAlbum's WHERE clause exactly.
func (s *Store) AlbumIDForIdentity(key AlbumIdentityKey) (uint, error) {
	var album model.Album
	err := s.db.Select("id").
		Where("name_norm = ? AND album_artist_norm = ? AND mb_release_id = ?",
			key.NameNorm, key.AlbumArtistNorm, key.MBReleaseID).
		First(&album).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return album.ID, nil
}

// RetagAlbum rewrites an album row's identity columns in place, keeping its id,
// its created_at and everything keyed on them: stars, the manual cover in the
// asset store, the "newest" ordering, and every client-side /album/:id.
//
// Writing an identity another row already holds violates idx_album_identity;
// the error is returned as-is so the caller can recognise it with
// IsUniqueViolation and fall back to matching instead of renaming.
func (s *Store) RetagAlbum(id uint, ident AlbumIdentity) error {
	res := s.db.Model(&model.Album{}).Where("id = ?", id).Updates(map[string]any{
		"name":              ident.Name,
		"name_norm":         ident.NameNorm,
		"album_artist_norm": ident.AlbumArtistNorm,
		"mb_release_id":     ident.MBReleaseID,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// IsUniqueViolation reports whether err is a unique-index conflict. The driver
// does not always map it to gorm.ErrDuplicatedKey, so the message is the
// fallback — same approach as handlers/libraries and handlers/users.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") || strings.Contains(msg, "duplicate")
}
