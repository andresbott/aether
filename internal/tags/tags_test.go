// internal/tags/tags_test.go
package tags_test

import (
	"testing"

	"github.com/andresbott/aether/internal/tags"
)

type mockReader struct {
	canRead bool
	meta    tags.Metadata
	err     error
}

func (m mockReader) CanRead(string) bool                { return m.canRead }
func (m mockReader) Read(string) (tags.Metadata, error) { return m.meta, m.err }

func TestFallbackReader_UsesPrimary(t *testing.T) {
	primary := mockReader{canRead: true, meta: tags.Metadata{Title: "from-primary"}}
	fallback := mockReader{canRead: true, meta: tags.Metadata{Title: "from-fallback"}}
	r := tags.NewFallbackReader(primary, fallback)

	if !r.CanRead("test.mp3") {
		t.Fatal("expected CanRead true")
	}
	m, err := r.Read("test.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "from-primary" {
		t.Fatalf("expected primary, got %q", m.Title)
	}
}

func TestFallbackReader_FallsBack(t *testing.T) {
	primary := mockReader{canRead: false}
	fallback := mockReader{canRead: true, meta: tags.Metadata{Title: "from-fallback"}}
	r := tags.NewFallbackReader(primary, fallback)

	if !r.CanRead("test.mka") {
		t.Fatal("expected CanRead true from fallback")
	}
	m, err := r.Read("test.mka")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "from-fallback" {
		t.Fatalf("expected fallback, got %q", m.Title)
	}
}

func TestFallbackReader_PrimaryErrorFallsBack(t *testing.T) {
	primary := mockReader{canRead: true, err: tags.ErrUnsupported}
	fallback := mockReader{canRead: true, meta: tags.Metadata{Title: "from-fallback"}}
	r := tags.NewFallbackReader(primary, fallback)

	m, err := r.Read("test.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "from-fallback" {
		t.Fatalf("expected fallback after primary error, got %q", m.Title)
	}
}

func TestFallbackReader_BothFail(t *testing.T) {
	primary := mockReader{canRead: false}
	fallback := mockReader{canRead: false}
	r := tags.NewFallbackReader(primary, fallback)

	if r.CanRead("test.xyz") {
		t.Fatal("expected CanRead false")
	}
	_, err := r.Read("test.xyz")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMetadataHasMBArtistIDField(t *testing.T) {
	var m tags.Metadata
	m.MBArtistID = []string{"a"}
	m.MBAlbumArtistID = []string{"b"}
	if m.MBArtistID[0] != "a" || m.MBAlbumArtistID[0] != "b" {
		t.Fatal("MBArtistID / MBAlbumArtistID fields missing")
	}
}
