package subsonic

import (
	"time"

	"github.com/andresbott/aether/internal/model"
)

// starLookup answers "when was this starred?" for a batch of items already
// loaded by a handler. Every response that carries artists, albums or songs
// builds one, so a list endpoint costs one extra query per entity type instead
// of one per row.
//
// The Subsonic/OpenSubsonic entities AlbumID3, ArtistID3 and Child all define an
// optional "starred" timestamp; it must be omitted, not empty, when the item is
// not starred — clients test for presence.
type starLookup struct {
	artists map[uint]time.Time
	albums  map[uint]time.Time
	tracks  map[uint]time.Time
}

// starGetter is the subset of *store.Store starLookup needs, so the helpers stay
// testable without a full handler.
type starGetter interface {
	StarredAt(owner, itemType string, itemIDs []uint) (map[uint]time.Time, error)
}

// newStarLookup batches one query per non-empty id set. A lookup error is not
// fatal: the response is still correct Subsonic, just without star state, which
// beats failing an entire browse request over an annotation.
func newStarLookup(s starGetter, owner string, artistIDs, albumIDs, trackIDs []uint) *starLookup {
	l := &starLookup{
		artists: map[uint]time.Time{},
		albums:  map[uint]time.Time{},
		tracks:  map[uint]time.Time{},
	}
	if m, err := s.StarredAt(owner, "artist", artistIDs); err == nil {
		l.artists = m
	}
	if m, err := s.StarredAt(owner, "album", albumIDs); err == nil {
		l.albums = m
	}
	if m, err := s.StarredAt(owner, "track", trackIDs); err == nil {
		l.tracks = m
	}
	return l
}

// applyArtist, applyAlbum and applyTrack set "starred" on an already-built
// entity map, leaving the key absent when the item is not starred.
func (l *starLookup) applyArtist(m map[string]any, id uint) map[string]any {
	return setStarred(m, l.artists, id)
}

func (l *starLookup) applyAlbum(m map[string]any, id uint) map[string]any {
	return setStarred(m, l.albums, id)
}

func (l *starLookup) applyTrack(m map[string]any, id uint) map[string]any {
	return setStarred(m, l.tracks, id)
}

func setStarred(m map[string]any, stars map[uint]time.Time, id uint) map[string]any {
	if ts, ok := stars[id]; ok {
		m["starred"] = ts.Format(time.RFC3339)
	}
	return m
}

// starredSongList builds the Subsonic Child list for a flat track slice with
// star state applied — the shape every song-only list endpoint returns.
func starredSongList(s starGetter, owner string, tracks []model.Track) []map[string]any {
	stars := newStarLookup(s, owner, nil, nil, trackIDs(tracks))
	songs := make([]map[string]any, 0, len(tracks))
	for i := range tracks {
		t := tracks[i]
		songs = append(songs, stars.applyTrack(trackToChild(&t, t.Album), t.ID))
	}
	return songs
}

// albumIDs, trackIDs and artistIDs collect primary keys for a batched star
// lookup.
func albumIDs(albums []model.Album) []uint {
	ids := make([]uint, 0, len(albums))
	for i := range albums {
		ids = append(ids, albums[i].ID)
	}
	return ids
}

func trackIDs(tracks []model.Track) []uint {
	ids := make([]uint, 0, len(tracks))
	for i := range tracks {
		ids = append(ids, tracks[i].ID)
	}
	return ids
}

func trackPtrIDs(tracks []*model.Track) []uint {
	ids := make([]uint, 0, len(tracks))
	for _, t := range tracks {
		if t != nil {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

func artistIDs(artists []model.Artist) []uint {
	ids := make([]uint, 0, len(artists))
	for i := range artists {
		ids = append(ids, artists[i].ID)
	}
	return ids
}
