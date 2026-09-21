package libraryfilter_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/libraryfilter"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanfolder"
)

func folders(t *testing.T, names ...string) *scanfolder.Set {
	t.Helper()
	in := make([]scanfolder.Folder, 0, len(names))
	for _, n := range names {
		in = append(in, scanfolder.Folder{Name: n, Path: "/srv/" + n})
	}
	set, err := scanfolder.NewSet(in)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func f(field model.LibraryFilterField, values ...string) model.LibraryFilter {
	return model.LibraryFilter{Field: field, Values: values}
}

func TestValidateNormalizes(t *testing.T) {
	got, issues := libraryfilter.Validate([]model.LibraryFilter{
		f(model.FilterScanFolder, " Music ", "Music"),
		f(model.FilterPath, "/srv/Music/Jazz/", "/srv/Music/./Rock"),
		f(model.FilterFormat, "FLAC", " mp3 "),
		f(model.FilterReleaseType, "Single", ""),
		f(model.FilterCompilation, " true "),
		f(model.FilterGenre, "Hard Bop"),
	}, folders(t, "Music"))
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	want := []model.LibraryFilter{
		f(model.FilterScanFolder, "Music"),                        // trimmed, de-duplicated
		f(model.FilterPath, "/srv/Music/Jazz", "/srv/Music/Rock"), // cleaned
		f(model.FilterFormat, "flac", "mp3"),                      // lowercased
		f(model.FilterReleaseType, "Single", ""),                  // "" = untyped, legal here only
		f(model.FilterCompilation, "true"),
		f(model.FilterGenre, "Hard Bop"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %d filters, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Field != want[i].Field || !slices.Equal(got[i].Values, want[i].Values) {
			t.Errorf("filter %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// genre, release_type and path are matched against data the scanner recorded,
// so Validate must not trim them beyond deciding blankness: store.ScopeOf
// binds them verbatim (see the nonBlank note in store/scope.go), so a genre
// really tagged "Rock " has to stay selectable, exactly as stored. scan_folder,
// format and compilation are vocabularies this package owns, so those keep
// being trimmed and normalized.
func TestValidateKeepsScannedValuesVerbatim(t *testing.T) {
	got, issues := libraryfilter.Validate([]model.LibraryFilter{
		f(model.FilterGenre, "Rock ", "Rock"),
		f(model.FilterReleaseType, " Live ", "  "),
		f(model.FilterPath, "/srv/Music/Album ", "/srv/Music/Jazz/"),
		f(model.FilterScanFolder, " Music "),
		f(model.FilterFormat, " FLAC "),
		f(model.FilterCompilation, " true "),
	}, folders(t, "Music"))
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	want := []model.LibraryFilter{
		f(model.FilterGenre, "Rock ", "Rock"),                       // untrimmed; both kept distinct
		f(model.FilterReleaseType, " Live ", ""),                    // untrimmed; all-blank collapses to "" (untyped)
		f(model.FilterPath, "/srv/Music/Album ", "/srv/Music/Jazz"), // trailing space kept; Clean still applies
		f(model.FilterScanFolder, "Music"),                          // own vocabulary: trimmed
		f(model.FilterFormat, "flac"),                               // own vocabulary: trimmed + lowercased
		f(model.FilterCompilation, "true"),                          // own vocabulary: trimmed
	}
	if len(got) != len(want) {
		t.Fatalf("got %d filters, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Field != want[i].Field || !slices.Equal(got[i].Values, want[i].Values) {
			t.Errorf("filter %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Zero filters is a valid library: the whole catalog.
func TestValidateAcceptsNoFilters(t *testing.T) {
	got, issues := libraryfilter.Validate(nil, folders(t))
	if len(issues) != 0 || len(got) != 0 {
		t.Fatalf("got %+v / %+v, want none", got, issues)
	}
}

func TestValidateReportsEveryProblemWithItsPointer(t *testing.T) {
	many := make([]string, libraryfilter.MaxValues+1)
	for i := range many {
		many[i] = "g" + strings.Repeat("x", i%7)
	}
	_, issues := libraryfilter.Validate([]model.LibraryFilter{
		f("mood", "happy"),                          // 0: unknown field
		f(model.FilterGenre),                        // 1: no values
		f(model.FilterScanFolder, "Music", "Gone"),  // 2: second value not configured
		f(model.FilterPath, "relative/dir"),         // 3: not absolute
		f(model.FilterFormat, "fl ac"),              // 4: not an extension
		f(model.FilterCompilation, "true", "false"), // 5: more than one value
		f(model.FilterCompilation, "maybe"),         // 6: not a boolean
		f(model.FilterGenre, "Jazz", "  "),          // 7: blank value
		f(model.FilterGenre, many...),               // 8: too many values
		f(model.FilterPath, " /srv/x"),              // 9: leading space is not absolute
	}, folders(t, "Music"))

	want := map[string]string{
		"/filters/0/field":    "unknown",
		"/filters/1/values":   "at least one",
		"/filters/2/values/1": "not configured",
		"/filters/3/values/0": "absolute",
		"/filters/4/values/0": "extension",
		"/filters/5/values":   "exactly one",
		"/filters/6/values/0": "true or false",
		"/filters/7/values/1": "empty",
		"/filters/8/values":   "at most",
		"/filters/9/values/0": "absolute",
	}
	got := map[string]string{}
	for _, is := range issues {
		got[is.Pointer] = is.Detail
	}
	for pointer, fragment := range want {
		detail, ok := got[pointer]
		if !ok {
			t.Errorf("no issue at %s; got %+v", pointer, issues)
			continue
		}
		if !strings.Contains(detail, fragment) {
			t.Errorf("issue at %s = %q, want it to mention %q", pointer, detail, fragment)
		}
	}
	if len(issues) != len(want) {
		t.Errorf("got %d issues, want %d: %+v", len(issues), len(want), issues)
	}
}

func TestValidateCapsTheNumberOfFilters(t *testing.T) {
	in := make([]model.LibraryFilter, libraryfilter.MaxFilters+1)
	for i := range in {
		in[i] = f(model.FilterGenre, "Jazz")
	}
	_, issues := libraryfilter.Validate(in, folders(t))
	if len(issues) != 1 || issues[0].Pointer != "/filters" || !strings.Contains(issues[0].Detail, "at most") {
		t.Fatalf("issues = %+v, want one at /filters", issues)
	}
}

// The per-filter cap does not bound a library on its own: twenty maximal
// filters are a scope SQLite spends seconds on, and that scope is evaluated on
// every GET /libraries and every /rest call scoped to the library. The total is
// counted on the REQUEST, so de-duplication cannot hide an oversized body.
func TestValidateCapsTheTotalNumberOfValues(t *testing.T) {
	full := func(prefix string) model.LibraryFilter {
		values := make([]string, libraryfilter.MaxValues)
		for i := range values {
			values[i] = prefix + strconv.Itoa(i)
		}
		return f(model.FilterGenre, values...)
	}
	// 3 × 100 = 300 values. No single filter is over its own cap, so the total
	// is the only thing wrong with this request.
	_, issues := libraryfilter.Validate([]model.LibraryFilter{full("a"), full("b"), full("c")}, folders(t))
	if len(issues) != 1 || issues[0].Pointer != "/filters" || !strings.Contains(issues[0].Detail, "in total") {
		t.Fatalf(`issues = %+v, want exactly one at /filters mentioning "in total"`, issues)
	}
	// 2 × 100 = 200 is exactly the cap, not over it.
	if _, issues := libraryfilter.Validate([]model.LibraryFilter{full("a"), full("b")}, folders(t)); len(issues) != 0 {
		t.Fatalf("unexpected issues at exactly the cap: %+v", issues)
	}
}

// A nil *scanfolder.Set is a valid empty set (see scanfolder.Set.ByName), so
// Validate must not panic on it — a scan_folder value just fails to resolve,
// exactly as if no folder by that name were configured.
func TestValidateNilFolders(t *testing.T) {
	_, issues := libraryfilter.Validate([]model.LibraryFilter{
		f(model.FilterScanFolder, "Music"),
	}, nil)
	if len(issues) != 1 || issues[0].Pointer != "/filters/0/values/0" || !strings.Contains(issues[0].Detail, "not configured") {
		t.Fatalf("issues = %+v, want one at /filters/0/values/0 mentioning \"not configured\"", issues)
	}
}

// A stored filter can outlive the folder it names: the folder is renamed or
// removed in the config file. That is reported, never an error — the library
// still loads, and that value simply matches nothing.
func TestDangling(t *testing.T) {
	filters := []model.LibraryFilter{
		f(model.FilterGenre, "Gone"), // a genre called like a missing folder is not a folder
		f(model.FilterScanFolder, "Music", "Gone"),
	}
	got := libraryfilter.Dangling(filters, folders(t, "Music"))
	if len(got) != 1 || got[0].Pointer != "/filters/1/values/1" || !strings.Contains(got[0].Detail, `"Gone"`) {
		t.Fatalf("Dangling = %+v", got)
	}
	if got := libraryfilter.Dangling(filters, nil); len(got) != 2 {
		t.Fatalf("with no folders configured both values dangle, got %+v", got)
	}
}
