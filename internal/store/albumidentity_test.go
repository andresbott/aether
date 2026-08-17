package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

// seedAlbumWithTracks creates one album row plus n tracks pointing at it and
// returns the album id and the track file paths.
func seedAlbumWithTracks(t *testing.T, s *store.Store, ident store.AlbumIdentity, paths []string) uint {
	t.Helper()
	album := model.Album{
		Name:            ident.Name,
		NameNorm:        ident.NameNorm,
		AlbumArtistNorm: ident.AlbumArtistNorm,
		MBReleaseID:     ident.MBReleaseID,
	}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		track := model.Track{AlbumID: album.ID, Filename: p, FilePath: p}
		if err := s.DB().Create(&track).Error; err != nil {
			t.Fatal(err)
		}
	}
	return album.ID
}

func TestAlbumIdentityKeyIgnoresTheDisplayName(t *testing.T) {
	a := store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "apocalyptica"}
	b := store.AlbumIdentity{Name: "CULT", NameNorm: "cult", AlbumArtistNorm: "apocalyptica"}
	if a.Key() != b.Key() {
		t.Fatal("a case-only difference in the display name must not be a different identity")
	}
}

func TestTrackAlbumIDs(t *testing.T) {
	s := testStore(t)
	id := seedAlbumWithTracks(t, s, store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "apocalyptica"},
		[]string{"/music/01.mp3", "/music/02.mp3"})

	got, err := s.TrackAlbumIDs([]string{"/music/01.mp3", "/music/02.mp3", "/music/nope.mp3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 known paths, got %d: %v", len(got), got)
	}
	if got["/music/01.mp3"] != id || got["/music/02.mp3"] != id {
		t.Fatalf("expected both tracks to map to album %d, got %v", id, got)
	}
	if _, ok := got["/music/nope.mp3"]; ok {
		t.Fatal("an unknown path must be absent, not zero")
	}
}

func TestAlbumTrackCounts(t *testing.T) {
	s := testStore(t)
	big := seedAlbumWithTracks(t, s, store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "a"},
		[]string{"/music/01.mp3", "/music/02.mp3", "/music/03.mp3"})
	small := seedAlbumWithTracks(t, s, store.AlbumIdentity{Name: "Reflections", NameNorm: "reflections", AlbumArtistNorm: "a"},
		[]string{"/music/04.mp3"})

	got, err := s.AlbumTrackCounts([]uint{big, small, 9999})
	if err != nil {
		t.Fatal(err)
	}
	if got[big] != 3 || got[small] != 1 {
		t.Fatalf("unexpected counts: %v", got)
	}
	if _, ok := got[9999]; ok {
		t.Fatal("an album with no tracks must be absent from the map")
	}
}

func TestAlbumIdentities(t *testing.T) {
	s := testStore(t)
	want := store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "apocalyptica", MBReleaseID: "rel-1"}
	id := seedAlbumWithTracks(t, s, want, []string{"/music/01.mp3"})

	got, err := s.AlbumIdentities([]uint{id})
	if err != nil {
		t.Fatal(err)
	}
	if got[id] != want {
		t.Fatalf("expected %+v, got %+v", want, got[id])
	}
}

func TestAlbumIDForIdentity(t *testing.T) {
	s := testStore(t)
	ident := store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "apocalyptica"}
	id := seedAlbumWithTracks(t, s, ident, []string{"/music/01.mp3"})

	got, err := s.AlbumIDForIdentity(ident.Key())
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("expected album %d, got %d", id, got)
	}

	free, err := s.AlbumIDForIdentity(store.AlbumIdentityKey{NameNorm: "nothing", AlbumArtistNorm: "here"})
	if err != nil {
		t.Fatal(err)
	}
	if free != 0 {
		t.Fatalf("expected 0 for an unheld identity, got %d", free)
	}
}
