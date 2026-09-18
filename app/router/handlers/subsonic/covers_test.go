package subsonic

import (
	"testing"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
)

func TestUpdateAlbumStoresGeneratedCover(t *testing.T) {
	s := testStore(t)
	album := model.Album{Name: "In Rainbows", NameNorm: "in rainbows", AlbumArtistNorm: "radiohead"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	// Ensure "bauhaus" is in the Available set:
	if err := s.SetCoverSettings(model.CoverSettings{
		DefaultStyles:   []string{"bauhaus"},
		AvailableStyles: []string{"bauhaus"},
	}); err != nil {
		t.Fatal(err)
	}

	srv, as := newRadioServer(t, s)
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{
		"id":                encodeAlbumID(album.ID),
		"generateStyle":     "bauhaus",
		"generateVariation": "2",
	}, nil, "")
	if status, code := postAlbum(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	if _, ok := as.Get(assetstore.KindAlbum, assetkey.AlbumOf(&album)); !ok {
		t.Fatal("expected a stored manual cover after generate")
	}
}

func TestUpdateAlbumRejectsUnavailableStyle(t *testing.T) {
	s := testStore(t)
	album := model.Album{Name: "The Bends", NameNorm: "the bends", AlbumArtistNorm: "radiohead"}
	if err := s.DB().Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	// Only "rings" is available, not "bauhaus":
	if err := s.SetCoverSettings(model.CoverSettings{
		DefaultStyles:   []string{"rings"},
		AvailableStyles: []string{"rings"},
	}); err != nil {
		t.Fatal(err)
	}

	srv, _ := newRadioServer(t, s)
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{
		"id":                encodeAlbumID(album.ID),
		"generateStyle":     "bauhaus",
		"generateVariation": "2",
	}, nil, "")
	if status, _ := postAlbum(t, srv.URL, body, ct); status != "failed" {
		t.Fatalf("expected failed for unavailable style, got %s", status)
	}
}
