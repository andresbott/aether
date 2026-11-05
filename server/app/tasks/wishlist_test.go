package tasks

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
)

const (
	wlArtist = "5b11f4ce-a62d-471e-81fc-a69a8278c7da"
	wlGroup  = "1b022e01-4da6-387b-8658-8678046e4cef"
	wlRec    = "9b1b4a2c-3c55-4c5e-8f0e-1a2b3c4d5e6f"
)

// mbTagReader tags every file as one Nirvana track carrying all three
// MusicBrainz ids a wishlist entry can match on.
type mbTagReader struct{}

func (mbTagReader) CanRead(p string) bool { return scanner.IsAudioFile(p) }
func (mbTagReader) Read(context.Context, string) (tags.Metadata, error) {
	return tags.Metadata{
		Title:            "Lithium",
		Artist:           []string{"Nirvana"},
		AlbumArtist:      []string{"Nirvana"},
		Album:            "Nevermind",
		TrackNumber:      5,
		Duration:         257,
		MBRecordingID:    wlRec,
		MBReleaseGroupID: wlGroup,
		MBArtistID:       []string{wlArtist},
		MBAlbumArtistID:  []string{wlArtist},
	}, nil
}

func TestScanMarksWishlistFound(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lithium.mp3"), []byte("not really audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := scanfolder.NewSet([]scanfolder.Folder{{Name: "Main", Path: root, FollowSymlinks: true}})
	if err != nil {
		t.Fatal(err)
	}
	st := newTestStore(t)
	items, err := st.AddWishlistItems([]model.WishlistItem{
		{Kind: model.WishlistArtist, MBID: wlArtist, Title: "Nirvana"},
		{Kind: model.WishlistAlbum, MBID: wlGroup, Title: "Nevermind"},
		{Kind: model.WishlistSong, MBID: wlRec, Title: "Lithium"},
	})
	if err != nil {
		t.Fatal(err)
	}

	scan := NewScanTaskFn(scanner.Config{Folders: set}, st, mbTagReader{}, true)
	if err := scan(context.Background(), slog.Default(), &progressSpy{}); err != nil {
		t.Fatalf("scan: %v", err)
	}

	var album model.Album
	if err := st.DB().First(&album).Error; err != nil {
		t.Fatal(err)
	}
	if album.MBReleaseGroupID != wlGroup {
		t.Fatalf("scan must store the release group id, got %q", album.MBReleaseGroupID)
	}
	for _, it := range items {
		got, err := st.GetWishlistItem(it.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.MatchID == 0 || got.MatchedAt == nil {
			t.Errorf("%s entry not found after the scan: %+v", got.Kind, got)
		}
		if got.Kind == model.WishlistSong && got.MatchAlbumID != album.ID {
			t.Errorf("song must point at its album %d, got %d", album.ID, got.MatchAlbumID)
		}
		if got.Kind == model.WishlistAlbum && got.MatchID != album.ID {
			t.Errorf("album entry points at %d, want %d", got.MatchID, album.ID)
		}
	}
}
