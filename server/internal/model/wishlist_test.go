package model_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestWishlistKindValid(t *testing.T) {
	for _, k := range model.WishlistKinds {
		if !k.Valid() {
			t.Errorf("%q must be valid", k)
		}
	}
	for _, k := range []model.WishlistKind{"", "playlist", "Album"} {
		if k.Valid() {
			t.Errorf("%q must not be valid", k)
		}
	}
}

// The (kind, mbid) pair is the entry's identity: the same MBID may be listed
// once per kind, never twice under one.
func TestWishlistItemIdentityIndex(t *testing.T) {
	db := testDB(t)
	const mbid = "1b022e01-4da6-387b-8658-8678046e4cef"
	if err := db.Create(&model.WishlistItem{Kind: model.WishlistAlbum, MBID: mbid, Title: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.WishlistItem{Kind: model.WishlistSong, MBID: mbid, Title: "b"}).Error; err != nil {
		t.Fatalf("another kind must be allowed: %v", err)
	}
	if err := db.Create(&model.WishlistItem{Kind: model.WishlistAlbum, MBID: mbid, Title: "c"}).Error; err == nil {
		t.Fatal("a duplicate (kind, mbid) must be refused")
	}
	// Entries added by hand have no MBID and never collide.
	for _, title := range []string{"bootleg", "another bootleg"} {
		it := model.WishlistItem{Kind: model.WishlistAlbum, Title: title}
		if err := db.Create(&it).Error; err != nil {
			t.Fatalf("a manual entry must be allowed: %v", err)
		}
		if !it.Manual() {
			t.Fatal("an entry without an MBID is manual")
		}
	}
}
