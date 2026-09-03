// internal/scanner/albumcontinuity_internal_test.go
//
// Unit tests for the two halves planAlbumContinuity is split into: the pure
// planner (planAlbumRetags), and the re-proof applyAlbumRetag performs before
// it writes. The end-to-end behaviour lives in albumcontinuity_test.go; these
// cover the branches a full scan cannot reach — chiefly a plan that has gone
// stale between the snapshot and the write, which is the risk the read/write
// split introduces.
package scanner

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func ident(nameNorm, artistNorm string) store.AlbumIdentity {
	return store.AlbumIdentity{Name: nameNorm, NameNorm: nameNorm, AlbumArtistNorm: artistNorm}
}

func TestPlanAlbumRetags(t *testing.T) {
	tcs := []struct {
		name string
		snap albumSnapshot
		want []albumRetagPlan
	}{
		{
			name: "nothing indexed yet",
			snap: albumSnapshot{
				want: map[string]store.AlbumIdentity{"/a/01.mp3": ident("cult", "apocalyptica")},
			},
		},
		{
			// The core case: every track of the album is in the batch and they
			// agree, so the row can be retagged and keep its id.
			name: "whole album retagged",
			snap: albumSnapshot{
				current: map[string]uint{"/a/01.mp3": 7, "/a/02.mp3": 7},
				want: map[string]store.AlbumIdentity{
					"/a/01.mp3": ident("cult", "apocalyptica"),
					"/a/02.mp3": ident("cult", "apocalyptica"),
				},
				counts: map[uint]int{7: 2},
				held:   map[uint]store.AlbumIdentity{7: ident("cult", "apocaliptica")},
			},
			want: []albumRetagPlan{{
				albumID:    7,
				trackCount: 2,
				oldIdent:   ident("cult", "apocaliptica"),
				newIdent:   ident("cult", "apocalyptica"),
			}},
		},
		{
			// The metadata editor's real shape: it rescans only the files it
			// wrote, so part of the album is missing from the batch. That is a
			// split, and the original row must be left alone.
			name: "split: part of the album is not in the batch",
			snap: albumSnapshot{
				current: map[string]uint{"/a/01.mp3": 7},
				want:    map[string]store.AlbumIdentity{"/a/01.mp3": ident("cult single", "apocalyptica")},
				counts:  map[uint]int{7: 3},
				held:    map[uint]store.AlbumIdentity{7: ident("cult", "apocalyptica")},
			},
		},
		{
			name: "split: the batch disagrees with itself",
			snap: albumSnapshot{
				current: map[string]uint{"/a/01.mp3": 7, "/a/02.mp3": 7},
				want: map[string]store.AlbumIdentity{
					"/a/01.mp3": ident("one", "apocalyptica"),
					"/a/02.mp3": ident("two", "apocalyptica"),
				},
				counts: map[uint]int{7: 2},
				held:   map[uint]store.AlbumIdentity{7: ident("cult", "apocalyptica")},
			},
		},
		{
			// The overwhelmingly common case on a normal rescan: nothing moved.
			name: "identity unchanged",
			snap: albumSnapshot{
				current: map[string]uint{"/a/01.mp3": 7},
				want:    map[string]store.AlbumIdentity{"/a/01.mp3": ident("cult", "apocalyptica")},
				counts:  map[uint]int{7: 1},
				held:    map[uint]store.AlbumIdentity{7: ident("cult", "apocalyptica")},
			},
		},
		{
			// A name-only edit that normalises to the same thing (case, accents)
			// is not a retag: name_norm is what the identity index covers.
			name: "display name differs but the key does not",
			snap: albumSnapshot{
				current: map[string]uint{"/a/01.mp3": 7},
				want: map[string]store.AlbumIdentity{
					"/a/01.mp3": {Name: "CULT", NameNorm: "cult", AlbumArtistNorm: "apocalyptica"},
				},
				counts: map[uint]int{7: 1},
				held:   map[uint]store.AlbumIdentity{7: ident("cult", "apocalyptica")},
			},
		},
		{
			// Three discs collapsing into one album: the row with the most
			// tracks survives, so the odds of keeping a cover or a star are as
			// high as they can be. One plan, not three.
			name: "merge: the largest album survives",
			snap: albumSnapshot{
				current: map[string]uint{
					"/cd1/01.mp3": 1, "/cd1/02.mp3": 1, "/cd1/03.mp3": 1,
					"/cd2/04.mp3": 2, "/cd2/05.mp3": 2,
					"/cd3/06.mp3": 3,
				},
				want: map[string]store.AlbumIdentity{
					"/cd1/01.mp3": ident("cult", "a"), "/cd1/02.mp3": ident("cult", "a"),
					"/cd1/03.mp3": ident("cult", "a"), "/cd2/04.mp3": ident("cult", "a"),
					"/cd2/05.mp3": ident("cult", "a"), "/cd3/06.mp3": ident("cult", "a"),
				},
				counts: map[uint]int{1: 3, 2: 2, 3: 1},
				held: map[uint]store.AlbumIdentity{
					1: ident("cult disc 1", "a"),
					2: ident("cult disc 2", "a"),
					3: ident("cult disc 3", "a"),
				},
			},
			want: []albumRetagPlan{{
				albumID:    1,
				trackCount: 3,
				mergedFrom: 2,
				oldIdent:   ident("cult disc 1", "a"),
				newIdent:   ident("cult", "a"),
			}},
		},
		{
			// Equal track counts: the lowest id wins, so the survivor never
			// depends on which tag reader finished first.
			name: "merge: equal counts tiebreak on the lowest id",
			snap: albumSnapshot{
				current: map[string]uint{"/cd2/01.mp3": 9, "/cd1/02.mp3": 4},
				want: map[string]store.AlbumIdentity{
					"/cd2/01.mp3": ident("cult", "a"),
					"/cd1/02.mp3": ident("cult", "a"),
				},
				counts: map[uint]int{9: 1, 4: 1},
				held: map[uint]store.AlbumIdentity{
					9: ident("cult disc 2", "a"),
					4: ident("cult disc 1", "a"),
				},
			},
			want: []albumRetagPlan{{
				albumID:    4,
				trackCount: 1,
				mergedFrom: 1,
				oldIdent:   ident("cult disc 1", "a"),
				newIdent:   ident("cult", "a"),
			}},
		},
		{
			// Two independent renames in one batch, emitted in target order so
			// the sequence of transactions is reproducible.
			name: "two albums retagged, ordered by target identity",
			snap: albumSnapshot{
				current: map[string]uint{"/z/01.mp3": 1, "/a/02.mp3": 2},
				want: map[string]store.AlbumIdentity{
					"/z/01.mp3": ident("zebra", "a"),
					"/a/02.mp3": ident("alpha", "a"),
				},
				counts: map[uint]int{1: 1, 2: 1},
				held: map[uint]store.AlbumIdentity{
					1: ident("zed", "a"),
					2: ident("al", "a"),
				},
			},
			want: []albumRetagPlan{
				{albumID: 2, trackCount: 1, oldIdent: ident("al", "a"), newIdent: ident("alpha", "a")},
				{albumID: 1, trackCount: 1, oldIdent: ident("zed", "a"), newIdent: ident("zebra", "a")},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := planAlbumRetags(tc.snap)
			if len(got) != len(tc.want) {
				t.Fatalf("expected %d plans, got %d: %+v", len(tc.want), len(got), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("plan %d:\n got %+v\nwant %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// planAlbumRetags must not depend on Go's map iteration order: the same
// snapshot has to produce the same survivor and the same order every time.
func TestPlanAlbumRetagsIsDeterministic(t *testing.T) {
	snap := albumSnapshot{
		current: map[string]uint{
			"/cd1/01.mp3": 5, "/cd1/02.mp3": 5,
			"/cd2/03.mp3": 6, "/cd2/04.mp3": 6,
			"/other/05.mp3": 7,
		},
		want: map[string]store.AlbumIdentity{
			"/cd1/01.mp3": ident("cult", "a"), "/cd1/02.mp3": ident("cult", "a"),
			"/cd2/03.mp3": ident("cult", "a"), "/cd2/04.mp3": ident("cult", "a"),
			"/other/05.mp3": ident("abbey road", "b"),
		},
		counts: map[uint]int{5: 2, 6: 2, 7: 1},
		held: map[uint]store.AlbumIdentity{
			5: ident("cult disc 1", "a"),
			6: ident("cult disc 2", "a"),
			7: ident("abbey rd", "b"),
		},
	}

	first := planAlbumRetags(snap)
	for i := 0; i < 50; i++ {
		got := planAlbumRetags(snap)
		if len(got) != len(first) {
			t.Fatalf("run %d: plan count changed: %d vs %d", i, len(got), len(first))
		}
		for j := range first {
			if got[j] != first[j] {
				t.Fatalf("run %d plan %d differs:\n got %+v\nwant %+v", i, j, got[j], first[j])
			}
		}
	}
}

// --- applyAlbumRetag: the re-proof that makes planning off a snapshot safe ---

func applyTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

// seedAlbum inserts an album carrying ident with trackCount tracks attached.
func seedAlbum(t *testing.T, st *store.Store, ident store.AlbumIdentity, trackCount int) uint {
	t.Helper()
	album := &model.Album{
		Name:            ident.Name,
		NameNorm:        ident.NameNorm,
		AlbumArtistNorm: ident.AlbumArtistNorm,
		MBReleaseID:     ident.MBReleaseID,
	}
	if err := st.DB().Create(album).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < trackCount; i++ {
		track := &model.Track{
			AlbumID:   album.ID,
			LibraryID: 1,
			Filename:  "t.mp3",
			FilePath:  t.Name() + "/" + ident.NameNorm + "/" + string(rune('a'+i)) + ".mp3",
		}
		if err := st.DB().Create(track).Error; err != nil {
			t.Fatal(err)
		}
	}
	return album.ID
}

func albumIdentity(t *testing.T, st *store.Store, id uint) store.AlbumIdentity {
	t.Helper()
	held, err := st.AlbumIdentities([]uint{id})
	if err != nil {
		t.Fatal(err)
	}
	return held[id]
}

func TestApplyAlbumRetagWritesAProvenPlan(t *testing.T) {
	st := applyTestStore(t)
	old := ident("cult", "apocaliptica")
	id := seedAlbum(t, st, old, 2)
	s := New(Config{}, st, nil)

	applied, err := s.applyAlbumRetag(albumRetagPlan{
		albumID:    id,
		trackCount: 2,
		oldIdent:   old,
		newIdent:   ident("cult", "apocalyptica"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("expected the retag to be applied")
	}
	if got := albumIdentity(t, st, id); got.AlbumArtistNorm != "apocalyptica" {
		t.Fatalf("expected the row to carry the new identity, got %q", got.AlbumArtistNorm)
	}
}

// The decisive test for the read/write split. The counts ARE the proof, and the
// snapshot they came from is history by the time we write: if the album gained
// or lost a track in between it has split, and renaming it anyway would merge
// two albums that must stay apart. Re-proving inside the writing transaction is
// what buys per-album transactions without that risk.
func TestApplyAlbumRetagDeclinesWhenTheTrackCountMoved(t *testing.T) {
	st := applyTestStore(t)
	old := ident("cult", "apocalyptica")
	id := seedAlbum(t, st, old, 3)
	s := New(Config{}, st, nil)

	// The plan was built when the album held 2 tracks; it now holds 3.
	applied, err := s.applyAlbumRetag(albumRetagPlan{
		albumID:    id,
		trackCount: 2,
		oldIdent:   old,
		newIdent:   ident("cult remastered", "apocalyptica"),
	})
	if err != nil {
		t.Fatalf("a stale plan is a decline, not an error: %v", err)
	}
	if applied {
		t.Fatal("expected the retag to be declined on a stale track count")
	}
	if got := albumIdentity(t, st, id); got.Key() != old.Key() {
		t.Fatalf("the row must be untouched, got %+v", got)
	}
}

// Something else moved the row between the snapshot and the write, so the plan
// describes an album that no longer exists in that shape.
func TestApplyAlbumRetagDeclinesWhenTheRowAlreadyMoved(t *testing.T) {
	st := applyTestStore(t)
	id := seedAlbum(t, st, ident("cult", "apocalyptica"), 1)
	s := New(Config{}, st, nil)

	applied, err := s.applyAlbumRetag(albumRetagPlan{
		albumID:    id,
		trackCount: 1,
		oldIdent:   ident("something else entirely", "apocalyptica"),
		newIdent:   ident("cult remastered", "apocalyptica"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("expected the retag to be declined: the row does not hold the planned identity")
	}
	if got := albumIdentity(t, st, id); got.NameNorm != "cult" {
		t.Fatalf("the row must be untouched, got %q", got.NameNorm)
	}
}

// Gone entirely — Cleanup removed it, or another pass merged it away.
func TestApplyAlbumRetagDeclinesWhenTheRowIsGone(t *testing.T) {
	st := applyTestStore(t)
	s := New(Config{}, st, nil)

	applied, err := s.applyAlbumRetag(albumRetagPlan{
		albumID:    4242,
		trackCount: 1,
		oldIdent:   ident("cult", "apocalyptica"),
		newIdent:   ident("cult remastered", "apocalyptica"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("expected the retag to be declined for a missing row")
	}
}

// The target identity is taken: that is a merge, and the tracks are repointed by
// FindOrCreateAlbum instead. Writing it would violate idx_album_identity.
func TestApplyAlbumRetagDeclinesWhenTheTargetIdentityIsTaken(t *testing.T) {
	st := applyTestStore(t)
	old := ident("reflections", "apocalyptica")
	target := ident("cult", "apocalyptica")
	id := seedAlbum(t, st, old, 1)
	seedAlbum(t, st, target, 1)
	s := New(Config{}, st, nil)

	applied, err := s.applyAlbumRetag(albumRetagPlan{
		albumID:    id,
		trackCount: 1,
		oldIdent:   old,
		newIdent:   target,
	})
	if err != nil {
		t.Fatalf("a taken identity is a decline, not an error: %v", err)
	}
	if applied {
		t.Fatal("expected the retag to be declined: the target identity is taken")
	}
	if got := albumIdentity(t, st, id); got.Key() != old.Key() {
		t.Fatalf("the row must be untouched, got %+v", got)
	}
}

// One album's failure must not cost the others theirs — the whole point of
// committing per album. The middle plan is unprovable (a stale count), and the
// two either side of it must still land.
func TestPlanAlbumContinuityAppliesTheOtherAlbumsWhenOneDeclines(t *testing.T) {
	st := applyTestStore(t)
	s := New(Config{}, st, nil)

	first := ident("alpha", "a")
	middle := ident("bravo", "a")
	last := ident("charlie", "a")
	firstID := seedAlbum(t, st, first, 1)
	middleID := seedAlbum(t, st, middle, 3)
	lastID := seedAlbum(t, st, last, 1)

	plans := []albumRetagPlan{
		{albumID: firstID, trackCount: 1, oldIdent: first, newIdent: ident("alpha ii", "a")},
		{albumID: middleID, trackCount: 2, oldIdent: middle, newIdent: ident("bravo ii", "a")}, // stale
		{albumID: lastID, trackCount: 1, oldIdent: last, newIdent: ident("charlie ii", "a")},
	}
	var appliedIDs []uint
	for _, plan := range plans {
		applied, err := s.applyAlbumRetag(plan)
		if err != nil {
			t.Fatalf("album %d: %v", plan.albumID, err)
		}
		if applied {
			appliedIDs = append(appliedIDs, plan.albumID)
		}
	}

	if len(appliedIDs) != 2 || appliedIDs[0] != firstID || appliedIDs[1] != lastID {
		t.Fatalf("expected albums %d and %d to be retagged, got %v", firstID, lastID, appliedIDs)
	}
	if got := albumIdentity(t, st, firstID); got.NameNorm != "alpha ii" {
		t.Fatalf("album before the declining one was rolled back: %q", got.NameNorm)
	}
	if got := albumIdentity(t, st, lastID); got.NameNorm != "charlie ii" {
		t.Fatalf("album after the declining one was not retagged: %q", got.NameNorm)
	}
	if got := albumIdentity(t, st, middleID); got.NameNorm != "bravo" {
		t.Fatalf("the declining album must be untouched, got %q", got.NameNorm)
	}
}
