// internal/scanner/albumidentity_test.go
package scanner_test

import (
	"testing"

	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

func TestAlbumIdentityOfAppliesTheScannerFallbacks(t *testing.T) {
	tests := []struct {
		name string
		meta tags.Metadata
		want store.AlbumIdentity
	}{
		{
			name: "plain album",
			meta: tags.Metadata{Album: "Cult", Artist: []string{"Apocalyptica"}, AlbumArtist: []string{"Apocalyptica"}},
			want: store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "apocalyptica"},
		},
		{
			name: "empty album name falls back",
			meta: tags.Metadata{Artist: []string{"Apocalyptica"}, AlbumArtist: []string{"Apocalyptica"}},
			want: store.AlbumIdentity{Name: "Unknown Album", NameNorm: "unknown album", AlbumArtistNorm: "apocalyptica"},
		},
		{
			name: "no album artist falls back to the track artist",
			meta: tags.Metadata{Album: "Cult", Artist: []string{"Apocalyptica"}},
			want: store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "apocalyptica"},
		},
		{
			name: "no album artist on a compilation falls back to Various Artists",
			meta: tags.Metadata{Album: "Mix", Artist: []string{"Someone"}, Compilation: true},
			want: store.AlbumIdentity{Name: "Mix", NameNorm: "mix", AlbumArtistNorm: "various artists"},
		},
		{
			name: "no artist at all falls back to Unknown Artist",
			meta: tags.Metadata{Album: "Cult"},
			want: store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "unknown artist"},
		},
		{
			name: "blank tag values are dropped before the fallback",
			meta: tags.Metadata{Album: "Cult", Artist: []string{"  "}, AlbumArtist: []string{""}},
			want: store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "unknown artist"},
		},
		{
			name: "the first album artist decides the identity",
			meta: tags.Metadata{Album: "Cult", AlbumArtist: []string{"Apocalyptica", "Nina Hagen"}},
			want: store.AlbumIdentity{Name: "Cult", NameNorm: "cult", AlbumArtistNorm: "apocalyptica"},
		},
		{
			name: "accents are transliterated, mbid is carried verbatim",
			meta: tags.Metadata{Album: "Ámbar", AlbumArtist: []string{"Björk"}, MBReleaseID: "REL-1"},
			want: store.AlbumIdentity{Name: "Ámbar", NameNorm: "ambar", AlbumArtistNorm: "bjork", MBReleaseID: "REL-1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := scanner.AlbumIdentityOf(tc.meta)
			if got != tc.want {
				t.Fatalf("expected %+v, got %+v", tc.want, got)
			}
			if got.Key() != tc.want.Key() {
				t.Fatalf("key mismatch: expected %+v, got %+v", tc.want.Key(), got.Key())
			}
		})
	}
}

func TestAlbumArtistNamesKeepsEveryCredit(t *testing.T) {
	meta := tags.Metadata{AlbumArtist: []string{"Apocalyptica", "Nina Hagen"}}
	got := scanner.AlbumArtistNames(meta)
	if len(got) != 2 || got[0] != "Apocalyptica" || got[1] != "Nina Hagen" {
		t.Fatalf("expected both album artists, got %v", got)
	}
}

func TestTrackArtistNamesFallsBackToUnknown(t *testing.T) {
	got := scanner.TrackArtistNames(tags.Metadata{})
	if len(got) != 1 || got[0] != "Unknown Artist" {
		t.Fatalf("expected [Unknown Artist], got %v", got)
	}
}
