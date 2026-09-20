package store_test

import (
	"slices"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

// seedScopeCatalog builds six tracks, named a–f in their titles, that between
// them exercise every clause kind:
//
//	a  lp       Music  /music/a/01.flac         flac  Rock
//	b  lp       Music  /music/ab/02.mp3         mp3   Rock, Live
//	c  single   Music  /music/a/sub/03.mp3      mp3   Jazz
//	d  comp     Books  /books/04.m4b            m4b   —
//	e  flagged  Books  /Music/a/05.mp3          mp3   Jazz
//	f  untyped  Music  /music/100%_pure/06.ogg  ogg   Live
//
// Albums: lp = ["Album"]; single = ["single"] (lowercase, as a Vorbis file may
// store it); comp = ["Album","Compilation"]; flagged = no types but the
// compilation flag; untyped = an explicitly empty type list.
func seedScopeCatalog(t *testing.T) *store.Store {
	t.Helper()
	s := testStore(t)
	db := s.DB()

	rock := &model.Genre{Name: "Rock"}
	jazz := &model.Genre{Name: "Jazz"}
	live := &model.Genre{Name: "Live"}
	for _, g := range []*model.Genre{rock, jazz, live} {
		if err := db.Create(g).Error; err != nil {
			t.Fatal(err)
		}
	}

	album := func(name string, types []string, compilation bool) model.Album {
		al := model.Album{Name: name, NameNorm: name, AlbumArtistNorm: "x", ReleaseTypes: types, Compilation: compilation}
		if err := db.Create(&al).Error; err != nil {
			t.Fatal(err)
		}
		return al
	}
	lp := album("lp", []string{"Album"}, false)
	single := album("single", []string{"single"}, false)
	comp := album("comp", []string{"Album", "Compilation"}, false)
	flagged := album("flagged", nil, true)
	untyped := album("untyped", []string{}, false)

	track := func(name string, al model.Album, folder, path, suffix string, genres ...*model.Genre) {
		tr := model.Track{
			Title: name, TitleNorm: name, AlbumID: al.ID,
			ScanFolder: folder, FilePath: path, Filename: path, Suffix: suffix,
		}
		if err := db.Create(&tr).Error; err != nil {
			t.Fatal(err)
		}
		if len(genres) > 0 {
			if err := db.Model(&tr).Association("Genres").Replace(genres); err != nil {
				t.Fatal(err)
			}
		}
	}
	track("a", lp, "Music", "/music/a/01.flac", "flac", rock)
	track("b", lp, "Music", "/music/ab/02.mp3", "mp3", rock, live)
	track("c", single, "Music", "/music/a/sub/03.mp3", "mp3", jazz)
	track("d", comp, "Books", "/books/04.m4b", "m4b")
	track("e", flagged, "Books", "/Music/a/05.mp3", "mp3", jazz)
	track("f", untyped, "Music", "/music/100%_pure/06.ogg", "ogg", live)
	return s
}

// tracksIn returns the titles of the tracks a scope matches, in title order.
// It goes through SearchSongs so the assertion covers a real scoped query.
func tracksIn(t *testing.T, s *store.Store, sc store.TrackScope) []string {
	t.Helper()
	tracks, err := s.SearchSongs("", 100, 0, &store.SearchFilter{Scope: sc})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(tracks))
	for _, tr := range tracks {
		out = append(out, tr.Title)
	}
	return out
}

// libFilter builds one library filter. Not named "filter": the ported tests in
// this package hold local variables of that name.
func libFilter(field model.LibraryFilterField, values ...string) model.LibraryFilter {
	return model.LibraryFilter{Field: field, Values: values}
}

func TestScopeOfClauses(t *testing.T) {
	s := seedScopeCatalog(t)
	all := []string{"a", "b", "c", "d", "e", "f"}

	cases := []struct {
		name    string
		filters []model.LibraryFilter
		want    []string
	}{
		{"no filters match everything", nil, all},

		{"scan folder", []model.LibraryFilter{libFilter(model.FilterScanFolder, "Music")}, []string{"a", "b", "c", "f"}},
		{"scan folder values are OR-ed", []model.LibraryFilter{libFilter(model.FilterScanFolder, "Music", "Books")}, all},
		{"unknown scan folder matches nothing", []model.LibraryFilter{libFilter(model.FilterScanFolder, "Gone")}, []string{}},

		// /music/a must not swallow its sibling /music/ab, and is case-sensitive.
		{"path is a directory prefix", []model.LibraryFilter{libFilter(model.FilterPath, "/music/a")}, []string{"a", "c"}},
		{"path tolerates a trailing slash", []model.LibraryFilter{libFilter(model.FilterPath, "/music/a/")}, []string{"a", "c"}},
		{"path treats % and _ literally", []model.LibraryFilter{libFilter(model.FilterPath, "/music/100%_pure")}, []string{"f"}},
		{"path / is everything", []model.LibraryFilter{libFilter(model.FilterPath, "/")}, all},
		{"path values are OR-ed", []model.LibraryFilter{libFilter(model.FilterPath, "/books", "/music/ab")}, []string{"b", "d"}},

		{"format", []model.LibraryFilter{libFilter(model.FilterFormat, "flac")}, []string{"a"}},
		{"format values are lowercased", []model.LibraryFilter{libFilter(model.FilterFormat, "MP3", "m4b")}, []string{"b", "c", "d", "e"}},

		{"release type", []model.LibraryFilter{libFilter(model.FilterReleaseType, "Album")}, []string{"a", "b", "d"}},
		{"release type is case-insensitive", []model.LibraryFilter{libFilter(model.FilterReleaseType, "SINGLE")}, []string{"c"}},
		{"empty release type means untyped", []model.LibraryFilter{libFilter(model.FilterReleaseType, "")}, []string{"e", "f"}},
		{"untyped combines with a type", []model.LibraryFilter{libFilter(model.FilterReleaseType, "", "single")}, []string{"c", "e", "f"}},

		// The flag OR a "Compilation" release type, exactly like isCompilation on /rest.
		{"compilation true", []model.LibraryFilter{libFilter(model.FilterCompilation, "true")}, []string{"d", "e"}},
		{"compilation false", []model.LibraryFilter{libFilter(model.FilterCompilation, "false")}, []string{"a", "b", "c", "f"}},

		{"genre", []model.LibraryFilter{libFilter(model.FilterGenre, "Rock")}, []string{"a", "b"}},
		{"genre values are OR-ed", []model.LibraryFilter{libFilter(model.FilterGenre, "Rock", "Jazz")}, []string{"a", "b", "c", "e"}},
		{"two genre filters are AND-ed", []model.LibraryFilter{libFilter(model.FilterGenre, "Rock"), libFilter(model.FilterGenre, "Live")}, []string{"b"}},

		{"filters are AND-ed", []model.LibraryFilter{libFilter(model.FilterScanFolder, "Music"), libFilter(model.FilterFormat, "mp3")}, []string{"b", "c"}},
		{"track and album level combine", []model.LibraryFilter{libFilter(model.FilterScanFolder, "Books"), libFilter(model.FilterCompilation, "true"), libFilter(model.FilterGenre, "Jazz")}, []string{"e"}},

		// Fail closed: a filter the compiler cannot honor must never widen the library.
		{"unknown field matches nothing", []model.LibraryFilter{libFilter("bitrate", "320")}, []string{}},
		{"empty values match nothing", []model.LibraryFilter{libFilter(model.FilterGenre)}, []string{}},
		{"blank value matches nothing", []model.LibraryFilter{libFilter(model.FilterGenre, "")}, []string{}},
		{"malformed compilation matches nothing", []model.LibraryFilter{libFilter(model.FilterCompilation, "maybe")}, []string{}},
		{"two compilation values match nothing", []model.LibraryFilter{libFilter(model.FilterCompilation, "true", "false")}, []string{}},
		{"one bad filter empties an otherwise valid library", []model.LibraryFilter{libFilter(model.FilterScanFolder, "Music"), libFilter("bitrate", "320")}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tracksIn(t, s, store.ScopeOf(tc.filters))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestScopeZeroAndNone(t *testing.T) {
	s := seedScopeCatalog(t)

	if !(store.TrackScope{}).IsZero() || !store.ScopeOf(nil).IsZero() {
		t.Fatal("the zero scope and ScopeOf(nil) must both report IsZero")
	}
	if store.NoTracks().IsZero() {
		t.Fatal("NoTracks must not report IsZero: it filters everything out")
	}
	if got := tracksIn(t, s, store.NoTracks()); len(got) != 0 {
		t.Fatalf("NoTracks matched %v", got)
	}
}

// An album is in a scope when at least one of its tracks is — the same
// "EXISTS a matching track" semantics a literal tracks.scan_folder filter had.
func TestScopeAppliesToAlbums(t *testing.T) {
	s := seedScopeCatalog(t)

	names := func(sc store.TrackScope) []string {
		albums, err := s.GetAlbumList("alphabeticalByName", 100, 0, &store.AlbumListFilter{Scope: sc})
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(albums))
		for _, al := range albums {
			out = append(out, al.Name)
		}
		return out
	}

	// lp holds a (flac) and b (mp3): one matching track is enough.
	if got, want := names(store.ScopeOf([]model.LibraryFilter{libFilter(model.FilterFormat, "flac")})), []string{"lp"}; !slices.Equal(got, want) {
		t.Fatalf("flac albums = %v, want %v", got, want)
	}
	if got, want := names(store.ScopeOf([]model.LibraryFilter{libFilter(model.FilterScanFolder, "Books")})), []string{"comp", "flagged"}; !slices.Equal(got, want) {
		t.Fatalf("Books albums = %v, want %v", got, want)
	}
	if got := names(store.NoTracks()); len(got) != 0 {
		t.Fatalf("NoTracks albums = %v, want none", got)
	}

	letters, total, err := s.GetAlbumLetterIndex(&store.AlbumListFilter{Scope: store.ScopeOf([]model.LibraryFilter{libFilter(model.FilterScanFolder, "Books")})})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(letters) != 2 || letters[0].Letter != "C" || letters[1].Letter != "F" {
		t.Fatalf("letter index = %+v (total %d), want C and F", letters, total)
	}
}

func TestLibraryScopeSelectsTheLibrarysTracks(t *testing.T) {
	s := seedScopeCatalog(t)
	got := tracksIn(t, s, store.LibraryScope(&model.Library{Name: "Books", Filters: scanFolderFilter("Books")}))
	if want := []string{"d", "e"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// The compiler binds values exactly as given: what it matches was stored
// verbatim by the scanner, so trimming here would make a padded name or tag
// unselectable.
func TestScopeBindsValuesVerbatim(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	padded := &model.Genre{Name: "Rock "}
	if err := db.Create(padded).Error; err != nil {
		t.Fatal(err)
	}
	album := model.Album{Name: "al", NameNorm: "al", AlbumArtistNorm: "x"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	tr := model.Track{Title: "t", TitleNorm: "t", AlbumID: album.ID, ScanFolder: " Jazz ", FilePath: "/j/1.mp3", Filename: "1.mp3", Suffix: "mp3"}
	if err := db.Create(&tr).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&tr).Association("Genres").Replace([]*model.Genre{padded}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		filter model.LibraryFilter
		want   []string
	}{
		{"padded scan folder matches verbatim", libFilter(model.FilterScanFolder, " Jazz "), []string{"t"}},
		{"trimmed scan folder does not", libFilter(model.FilterScanFolder, "Jazz"), []string{}},
		{"padded genre matches verbatim", libFilter(model.FilterGenre, "Rock "), []string{"t"}},
		{"trimmed genre does not", libFilter(model.FilterGenre, "Rock"), []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tracksIn(t, s, store.ScopeOf([]model.LibraryFilter{tc.filter}))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	// LibraryScope must not add its own normalization on top of ScopeOf: a
	// padded filter value still binds verbatim when reached through a library.
	if got := tracksIn(t, s, store.LibraryScope(&model.Library{Filters: scanFolderFilter(" Jazz ")})); !slices.Equal(got, []string{"t"}) {
		t.Fatalf("a library's filter values must bind verbatim through LibraryScope, got %v", got)
	}
}

// scope.go's header claims one clause shape drops into all three embedding
// sites, but until now no subquery-bearing clause (genre, release type,
// compilation) was exercised through a NESTING site — tracksIn only ever runs
// a scope directly against a tracks query. Phase 3 switches these filters on
// without touching scope.go again, so the nesting sites need their own
// coverage now: GetAlbumList's `EXISTS (… FROM tracks …)` under an albums
// query, and SearchGenres's own `genres g` alias inside a query over genres.
func TestScopeSubqueryClausesNest(t *testing.T) {
	s := seedScopeCatalog(t)

	albumNames := func(sc store.TrackScope) []string {
		albums, err := s.GetAlbumList("alphabeticalByName", 100, 0, &store.AlbumListFilter{Scope: sc})
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(albums))
		for _, al := range albums {
			out = append(out, al.Name)
		}
		return out
	}
	albumCases := []struct {
		name    string
		filters []model.LibraryFilter
		want    []string
	}{
		{"genre", []model.LibraryFilter{libFilter(model.FilterGenre, "Rock")}, []string{"lp"}},
		{"release type", []model.LibraryFilter{libFilter(model.FilterReleaseType, "Album")}, []string{"comp", "lp"}},
		{"untyped release type", []model.LibraryFilter{libFilter(model.FilterReleaseType, "")}, []string{"flagged", "untyped"}},
		{"compilation true", []model.LibraryFilter{libFilter(model.FilterCompilation, "true")}, []string{"comp", "flagged"}},
		{"compilation false", []model.LibraryFilter{libFilter(model.FilterCompilation, "false")}, []string{"lp", "single", "untyped"}},
		{"scan folder + compilation + genre", []model.LibraryFilter{
			libFilter(model.FilterScanFolder, "Books"),
			libFilter(model.FilterCompilation, "true"),
			libFilter(model.FilterGenre, "Jazz"),
		}, []string{"flagged"}},
	}
	for _, tc := range albumCases {
		t.Run("albums: "+tc.name, func(t *testing.T) {
			got := albumNames(store.ScopeOf(tc.filters))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}

	genreNames := func(sc store.TrackScope) []string {
		genres, err := s.SearchGenres("", 20, 0, &store.SearchFilter{Scope: sc})
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(genres))
		for _, g := range genres {
			out = append(out, g.Name)
		}
		return out
	}
	genreCases := []struct {
		name    string
		filters []model.LibraryFilter
		want    []string
	}{
		{"genre", []model.LibraryFilter{libFilter(model.FilterGenre, "Rock")}, []string{"Live", "Rock"}},
		{"format", []model.LibraryFilter{libFilter(model.FilterFormat, "flac")}, []string{"Rock"}},
	}
	for _, tc := range genreCases {
		t.Run("genres: "+tc.name, func(t *testing.T) {
			got := genreNames(store.ScopeOf(tc.filters))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// scanFolderScope is the scope a library named name resolves to.
func scanFolderScope(name string) store.TrackScope {
	return store.ScopeOf([]model.LibraryFilter{libFilter(model.FilterScanFolder, name)})
}

// scanFolderOf is the name of the library with this id, so fixtures that are
// handed a library id can stamp the marker the scope matches on.
func scanFolderOf(t *testing.T, s *store.Store, libID uint) string {
	t.Helper()
	lib, err := s.GetLibrary(libID)
	if err != nil {
		t.Fatal(err)
	}
	return lib.Name
}

func TestPathRange(t *testing.T) {
	for _, tc := range []struct{ dir, lo, hi string }{
		{"/music/a", "/music/a/", "/music/a0"},
		{"/music/a/", "/music/a/", "/music/a0"},
		{"/music/../music/a", "/music/a/", "/music/a0"},
		{"/", "/", "0"},
	} {
		lo, hi := store.PathRange(tc.dir)
		if lo != tc.lo || hi != tc.hi {
			t.Errorf("PathRange(%q) = %q, %q; want %q, %q", tc.dir, lo, hi, tc.lo, tc.hi)
		}
	}
}

// A library is its filters, not its name: renaming one must not change what it
// selects. (Until this phase a library selected the scan folder called like it,
// so a rename emptied it until the next scan.)
func TestLibraryScopeComesFromTheFiltersNotTheName(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	if err := db.Create(&album).Error; err != nil {
		t.Fatal(err)
	}
	for _, tr := range []model.Track{
		{AlbumID: album.ID, Filename: "m.flac", FilePath: "/m/m.flac", ScanFolder: "Music", Suffix: "flac", Title: "m.flac", TitleNorm: "m.flac"},
		{AlbumID: album.ID, Filename: "b.mp3", FilePath: "/b/b.mp3", ScanFolder: "Books", Suffix: "mp3", Title: "b.mp3", TitleNorm: "b.mp3"},
	} {
		if err := db.Create(&tr).Error; err != nil {
			t.Fatal(err)
		}
	}
	lib := model.Library{
		Name:    "Whatever I Like",
		Filters: []model.LibraryFilter{{Field: model.FilterScanFolder, Values: []string{"Music"}}},
	}
	if err := s.CreateLibrary(&lib); err != nil {
		t.Fatal(err)
	}
	if got := tracksIn(t, s, store.LibraryScope(&lib)); !slices.Equal(got, []string{"m.flac"}) {
		t.Fatalf("scope = %v, want the Music track", got)
	}

	lib.Name = "Renamed"
	if err := s.UpdateLibrary(&lib); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.GetLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := tracksIn(t, s, store.LibraryScope(&reloaded)); !slices.Equal(got, []string{"m.flac"}) {
		t.Fatalf("after a rename the scope = %v, want it unchanged", got)
	}
}

// No filters is the whole catalog — and the filters survive the JSON column.
func TestLibraryWithNoFiltersIsTheWholeCatalog(t *testing.T) {
	s := testStore(t)
	lib := model.Library{Name: "Everything"}
	if err := s.CreateLibrary(&lib); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.GetLibrary(lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !store.LibraryScope(&reloaded).IsZero() {
		t.Fatal("a library without filters must compile to the zero scope")
	}
}

func TestCountTracks(t *testing.T) {
	s := seedScopeCatalog(t)
	for _, tc := range []struct {
		name string
		sc   store.TrackScope
		want int64
	}{
		{"zero scope counts everything", store.TrackScope{}, 6},
		{"scan folder", scanFolderScope("Books"), 2},
		{"none", store.NoTracks(), 0},
		{"album-level clause", store.ScopeOf([]model.LibraryFilter{libFilter(model.FilterCompilation, "true")}), 2},
	} {
		got, err := s.CountTracks(tc.sc)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%s: CountTracks = %d, want %d", tc.name, got, tc.want)
		}
	}
}
