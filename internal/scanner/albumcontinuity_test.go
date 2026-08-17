// internal/scanner/albumcontinuity_test.go
package scanner_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

// retagReader is a tag reader whose album identity can be changed between
// scans, standing in for the metadata editor writing new tags to the files.
// perPath overrides the album name of individual files, for the cases where a
// batch disagrees with itself.
type retagReader struct {
	album       string
	albumArtist string
	mbReleaseID string
	perPath     map[string]string
}

func (r *retagReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }

func (r *retagReader) Read(_ context.Context, absPath string) (tags.Metadata, error) {
	album := r.album
	if v, ok := r.perPath[absPath]; ok {
		album = v
	}
	return tags.Metadata{
		Title:       filepath.Base(absPath),
		Artist:      []string{r.albumArtist},
		AlbumArtist: []string{r.albumArtist},
		Album:       album,
		MBReleaseID: r.mbReleaseID,
		Genre:       []string{"Rock"},
		Year:        2020,
		TrackNumber: 1,
		Duration:    180,
		Bitrate:     320,
	}, nil
}

// theOnlyAlbum fails unless the DB holds exactly one album row, and returns it.
func theOnlyAlbum(t *testing.T, st *store.Store) model.Album {
	t.Helper()
	var albums []model.Album
	if err := st.DB().Find(&albums).Error; err != nil {
		t.Fatal(err)
	}
	if len(albums) != 1 {
		t.Fatalf("expected exactly 1 album row, got %d: %+v", len(albums), albums)
	}
	return albums[0]
}

// The core promise: retagging every track of an album keeps the row, so
// everything keyed on albums.id survives — the manual cover in the asset store
// (handlers/subsonic/albums.go keys on it), stars, created_at and therefore the
// "newest" list and the discovery feed, plus every client-cached /album/:id.
func TestReconcileKeepsTheAlbumIDWhenTheWholeAlbumIsRetagged(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
		"Apocalyptica/Cult/03.mp3",
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocaliptica"}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

	// The editor fixes the misspelled album artist on every file of the album.
	reader.albumArtist = "Apocalyptica"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyAlbum(t, st)
	if after.ID != before.ID {
		t.Fatalf("album id changed on retag: was %d, now %d", before.ID, after.ID)
	}
	if after.AlbumArtistNorm != "apocalyptica" {
		t.Fatalf("expected the row to carry the new identity, got %q", after.AlbumArtistNorm)
	}

	var trackCount int64
	if err := st.DB().Model(&model.Track{}).Where("album_id = ?", after.ID).Count(&trackCount).Error; err != nil {
		t.Fatal(err)
	}
	if trackCount != 3 {
		t.Fatalf("expected all 3 tracks on the retagged row, got %d", trackCount)
	}
}

// The identify-album flow: the whole selection gains an MBReleaseID at once.
// This is the most common churn trigger in practice.
func TestReconcileKeepsTheAlbumIDWhenTheAlbumGainsAnMBID(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{
		"Apocalyptica/Cult/01.mp3",
		"Apocalyptica/Cult/02.mp3",
	})
	seedLibrary(t, st, dir, nil)

	reader := &retagReader{album: "Cult", albumArtist: "Apocalyptica"}
	s := scanner.New(scanner.Config{}, st, reader)
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}
	before := theOnlyAlbum(t, st)

	reader.mbReleaseID = "e1b2c3d4-0000-0000-0000-000000000001"
	if _, err := s.Scan(context.Background(), scanner.ScanOptions{IsFull: true}); err != nil {
		t.Fatal(err)
	}

	after := theOnlyAlbum(t, st)
	if after.ID != before.ID {
		t.Fatalf("album id changed when the album gained an MBID: was %d, now %d", before.ID, after.ID)
	}
	if after.MBReleaseID != reader.mbReleaseID {
		t.Fatalf("expected the row to carry the new MBID, got %q", after.MBReleaseID)
	}
}
