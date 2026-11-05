package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

// albumWithTracks creates an album with one track per grouping tag given.
func albumWithTracks(t *testing.T, s *store.Store, name string, year int, scanFolder string, groupings ...string) model.Album {
	t.Helper()
	al := model.Album{Name: name, NameNorm: name, AlbumArtistNorm: "x", Year: year}
	if err := s.DB().Create(&al).Error; err != nil {
		t.Fatal(err)
	}
	for i, g := range groupings {
		tr := model.Track{
			AlbumID: al.ID, ScanFolder: scanFolder, Grouping: g, Duration: 100,
			Filename: name + string(rune('a'+i)), FilePath: "/" + scanFolder + "/" + name + string(rune('a'+i)),
		}
		if err := s.DB().Create(&tr).Error; err != nil {
			t.Fatal(err)
		}
	}
	return al
}

func groupingOf(t *testing.T, s *store.Store, albumID uint) string {
	t.Helper()
	var al model.Album
	if err := s.DB().First(&al, albumID).Error; err != nil {
		t.Fatal(err)
	}
	if al.GroupingID == nil {
		return ""
	}
	var g model.Grouping
	if err := s.DB().First(&g, *al.GroupingID).Error; err != nil {
		t.Fatal(err)
	}
	return g.Name
}

func TestReconcileGroupingsMajorityAndNormalization(t *testing.T) {
	s := testStore(t)
	a := albumWithTracks(t, s, "S1", 2005, "f", "Doctor Who", "doctor who", "Other")
	b := albumWithTracks(t, s, "Tie", 2006, "f", "Zeta", "Alpha")
	c := albumWithTracks(t, s, "Plain", 2007, "f", "", "")
	d := albumWithTracks(t, s, "Partly", 2008, "f", "", "Doctor Who")

	if err := s.ReconcileGroupings(); err != nil {
		t.Fatal(err)
	}
	if got := groupingOf(t, s, a.ID); got != "Doctor Who" && got != "doctor who" {
		t.Fatalf("majority grouping = %q", got)
	}
	if got := groupingOf(t, s, b.ID); got != "Alpha" {
		t.Fatalf("tie must go to the alphabetically first, got %q", got)
	}
	if got := groupingOf(t, s, c.ID); got != "" {
		t.Fatalf("untagged album must have no grouping, got %q", got)
	}
	// An untagged track does not outvote the tagged one.
	if got := groupingOf(t, s, d.ID); got != "Doctor Who" {
		t.Fatalf("partly tagged album grouping = %q", got)
	}
	// "Doctor Who" and "doctor who" are one grouping row.
	var n int64
	s.DB().Model(&model.Grouping{}).Where("name_norm = ?", "doctor who").Count(&n)
	if n != 1 {
		t.Fatalf("expected one doctor who grouping, got %d", n)
	}
}

func TestReconcileGroupingsFollowsRetagAndDropsEmpty(t *testing.T) {
	s := testStore(t)
	a := albumWithTracks(t, s, "S1", 2005, "f", "Old")
	if err := s.ReconcileGroupings(); err != nil {
		t.Fatal(err)
	}
	if groupingOf(t, s, a.ID) != "Old" {
		t.Fatal("expected Old")
	}

	// Renamed: the album moves, the empty grouping is deleted.
	s.DB().Model(&model.Track{}).Where("album_id = ?", a.ID).Update("grouping", "New")
	if err := s.ReconcileGroupings(); err != nil {
		t.Fatal(err)
	}
	if groupingOf(t, s, a.ID) != "New" {
		t.Fatal("expected the album to move to New")
	}
	var n int64
	s.DB().Model(&model.Grouping{}).Count(&n)
	if n != 1 {
		t.Fatalf("expected the emptied grouping to be deleted, %d left", n)
	}

	// Tag removed: the album is un-grouped and nothing is left.
	s.DB().Model(&model.Track{}).Where("album_id = ?", a.ID).Update("grouping", "")
	if err := s.ReconcileGroupings(); err != nil {
		t.Fatal(err)
	}
	if groupingOf(t, s, a.ID) != "" {
		t.Fatal("expected the album to be un-grouped")
	}
	s.DB().Model(&model.Grouping{}).Count(&n)
	if n != 0 {
		t.Fatalf("expected no groupings, %d left", n)
	}
}

func TestGroupingsIndexOrderAndScope(t *testing.T) {
	s := testStore(t)
	lib := model.Library{Name: "L1", Filters: scanFolderFilter("L1")}
	if err := s.DB().Create(&lib).Error; err != nil {
		t.Fatal(err)
	}
	albumWithTracks(t, s, "Part 3", 2010, "L1", "Saga")
	albumWithTracks(t, s, "Part 1", 2001, "L1", "Saga")
	albumWithTracks(t, s, "Undated", 0, "L1", "Saga")
	albumWithTracks(t, s, "Part 2", 2005, "L2", "Saga")
	albumWithTracks(t, s, "Other", 1999, "L2", "Other")
	if err := s.ReconcileGroupings(); err != nil {
		t.Fatal(err)
	}

	all, err := s.GetGroupings(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Name != "Other" || all[1].Name != "Saga" {
		t.Fatalf("index = %+v", all)
	}
	if all[1].AlbumCount != 4 || all[1].SongCount != 4 || all[1].Year != 2001 {
		t.Fatalf("Saga counts = %+v", all[1])
	}

	scoped, err := s.GetGroupings(&store.GroupingsFilter{Scope: store.LibraryScope(&lib)})
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 1 || scoped[0].Name != "Saga" || scoped[0].AlbumCount != 3 {
		t.Fatalf("scoped index = %+v", scoped)
	}
	checkGroupingAlbums(t, s, all[1].ID, &lib)
}

func checkGroupingAlbums(t *testing.T, s *store.Store, id uint, lib *model.Library) {
	t.Helper()
	g, albums, err := s.GetGrouping(id, &store.GroupingsFilter{Scope: store.LibraryScope(lib)})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range albums {
		names = append(names, a.Name)
	}
	want := []string{"Part 1", "Part 3", "Undated"}
	if g.Name != "Saga" || len(names) != 3 || names[0] != want[0] || names[1] != want[1] || names[2] != want[2] {
		t.Fatalf("albums = %v, want %v", names, want)
	}

	cover, err := s.GroupingCoverAlbum(g.ID)
	if err != nil || cover.Name != "Part 1" {
		t.Fatalf("cover album = %v, %v", cover, err)
	}
	if _, _, err := s.GetGrouping(9999, nil); err == nil {
		t.Fatal("expected not found")
	}
}
