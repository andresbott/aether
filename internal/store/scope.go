package store

import (
	"path/filepath"
	"strings"

	"github.com/andresbott/aether/internal/model"
	"gorm.io/gorm"
)

// TrackScope is a compiled library predicate: a boolean SQL expression over a
// tracks alias. The zero value matches every track, so "no library selected"
// needs no special case at the call sites.
//
// A scope is compiled from a library's filters by ScopeOf. Filters are AND-ed;
// the values inside one filter are OR-ed. Track-level filters test the track
// row itself; album-level ones (release type, compilation) reach the track's
// album through a correlated subquery. That keeps every clause a predicate over
// ONE tracks alias, which is the only shape that drops into all three places the
// store scopes a query: a tracks query, an `EXISTS (… FROM tracks …)` under an
// albums/genres query, and the `t` alias inside excludeHiddenArtists.
//
// An entity other than a track is in a scope when at least one of its tracks
// is — the semantics `tracks.library_id = ?` always had.
type TrackScope struct {
	clauses []scopeClause
	// none makes the scope match nothing. It is what an unknown library, or a
	// filter the compiler cannot honor, resolves to: a broken filter must never
	// widen a library to the whole catalog.
	none bool
}

// scopeClause is one compiled filter. sql names the tracks alias as {t}; args
// are its bind values in order.
type scopeClause struct {
	sql  string
	args []any
}

// tracksAlias is the placeholder a clause uses for the tracks alias.
const tracksAlias = "{t}"

// NoTracks is the scope that matches nothing. An unknown musicFolderId resolves
// to it, so such a request keeps answering empty lists instead of erroring.
func NoTracks() TrackScope { return TrackScope{none: true} }

// IsZero reports whether the scope matches every track, i.e. needs no SQL.
func (sc TrackScope) IsZero() bool { return !sc.none && len(sc.clauses) == 0 }

// LibraryScope is the scope a library selects. Until libraries carry their own
// filters that is every track indexed under the library's name.
func LibraryScope(lib *model.Library) TrackScope {
	return ScopeOf([]model.LibraryFilter{{Field: model.FilterScanFolder, Values: []string{lib.Name}}})
}

// ScopeOf compiles a library's filters. It is pure — no database, no
// filesystem, no configuration — so a scope can be built anywhere. A filter it
// cannot honor (unknown field, no usable value, a malformed boolean) makes the
// whole scope match nothing: validation belongs to whoever stores filters, and
// failing closed is the only safe answer for what slips past it.
func ScopeOf(filters []model.LibraryFilter) TrackScope {
	var sc TrackScope
	for _, f := range filters {
		clause, ok := compileFilter(f)
		if !ok {
			return NoTracks()
		}
		sc.clauses = append(sc.clauses, clause)
	}
	return sc
}

func compileFilter(f model.LibraryFilter) (scopeClause, bool) {
	switch f.Field {
	case model.FilterScanFolder:
		return inClause(tracksAlias+".scan_folder IN ?", nonBlank(f.Values))
	case model.FilterFormat:
		return inClause(tracksAlias+".suffix IN ?", lowered(nonBlank(f.Values)))
	case model.FilterGenre:
		return inClause("EXISTS (SELECT 1 FROM track_genres tg JOIN genres g ON g.id = tg.genre_id "+
			"WHERE tg.track_id = "+tracksAlias+".id AND g.name IN ?)", nonBlank(f.Values))
	case model.FilterPath:
		return pathClause(nonBlank(f.Values))
	case model.FilterReleaseType:
		return releaseTypeClause(f.Values)
	case model.FilterCompilation:
		return compilationClause(f.Values)
	default:
		return scopeClause{}, false
	}
}

// inClause builds a single-IN clause; an empty value list is not a usable filter.
func inClause(sql string, values []string) (scopeClause, bool) {
	if len(values) == 0 {
		return scopeClause{}, false
	}
	return scopeClause{sql: sql, args: []any{values}}, true
}

// pathClause matches tracks under any of the directories. It is a byte range,
// not a LIKE: SQLite's LIKE is ASCII case-insensitive and would need % and _
// escaped, while file_path's BINARY collation makes the range exact. "/" is
// 0x2F and "0" is 0x30, so [dir+"/", dir+"0") is precisely "everything under
// dir" — and never its sibling that merely shares the prefix.
func pathClause(dirs []string) (scopeClause, bool) {
	if len(dirs) == 0 {
		return scopeClause{}, false
	}
	parts := make([]string, 0, len(dirs))
	args := make([]any, 0, 2*len(dirs))
	for _, dir := range dirs {
		// Clean drops a trailing slash; trimming the root's own "/" leaves "",
		// whose range ["/", "0") covers every absolute path.
		prefix := strings.TrimSuffix(filepath.Clean(dir), "/")
		parts = append(parts, "("+tracksAlias+".file_path >= ? AND "+tracksAlias+".file_path < ?)")
		args = append(args, prefix+"/", prefix+"0")
	}
	return scopeClause{sql: strings.Join(parts, " OR "), args: args}, true
}

// albumOf opens the correlated subquery that reaches a track's album.
const albumOf = "EXISTS (SELECT 1 FROM albums sa WHERE sa.id = " + tracksAlias + ".album_id AND "

// releaseTypeClause matches tracks whose album carries one of the types. Tags
// are stored raw, so the match is case-insensitive. The empty string stands for
// "no release type at all", without which an untagged album could never be
// selected by a release-type library.
func releaseTypeClause(values []string) (scopeClause, bool) {
	var typed []string
	untyped := false
	for _, v := range values {
		if v = strings.TrimSpace(v); v == "" {
			untyped = true
		} else {
			typed = append(typed, strings.ToLower(v))
		}
	}
	var parts []string
	var args []any
	if len(typed) > 0 {
		parts = append(parts, albumOf+"EXISTS (SELECT 1 FROM json_each(sa.release_types) je WHERE LOWER(je.value) IN ?))")
		args = append(args, typed)
	}
	if untyped {
		// NULL column, JSON null and [] all count as untyped.
		parts = append(parts, albumOf+"COALESCE(json_array_length(sa.release_types), 0) = 0)")
	}
	if len(parts) == 0 {
		return scopeClause{}, false
	}
	return scopeClause{sql: strings.Join(parts, " OR "), args: args}, true
}

// compilationClause mirrors what /rest reports as isCompilation (albumToMap):
// the compilation flag OR a "compilation" release type.
func compilationClause(values []string) (scopeClause, bool) {
	if len(values) != 1 {
		return scopeClause{}, false
	}
	sql := albumOf + "(sa.compilation = ? OR EXISTS (SELECT 1 FROM json_each(sa.release_types) je WHERE LOWER(je.value) = 'compilation')))"
	switch values[0] {
	case "true":
		return scopeClause{sql: sql, args: []any{true}}, true
	case "false":
		return scopeClause{sql: "NOT " + sql, args: []any{true}}, true
	default:
		return scopeClause{}, false
	}
}

func nonBlank(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func lowered(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, strings.ToLower(v))
	}
	return out
}

// where renders the scope as a boolean SQL expression over the given tracks
// alias, with its bind values. It is total: the zero scope renders as a
// tautology, so a scope can be negated or AND-ed without first checking IsZero.
func (sc TrackScope) where(alias string) (string, []any) {
	if sc.none {
		return "1 = 0", nil
	}
	if len(sc.clauses) == 0 {
		return "1 = 1", nil
	}
	parts := make([]string, 0, len(sc.clauses))
	var args []any
	for _, c := range sc.clauses {
		parts = append(parts, "("+strings.ReplaceAll(c.sql, tracksAlias, alias)+")")
		args = append(args, c.args...)
	}
	return strings.Join(parts, " AND "), args
}

// scopeTracks narrows a query that selects from, or joins, tracks.
func scopeTracks(q *gorm.DB, sc TrackScope) *gorm.DB {
	if sc.IsZero() {
		return q
	}
	sql, args := sc.where("tracks")
	return q.Where(sql, args...)
}

// scopeByAlbum narrows a query whose rows name an album — albumCol is
// "albums.id", or "starred_items.item_id" for album stars — to the albums with
// at least one track in the scope.
func scopeByAlbum(q *gorm.DB, sc TrackScope, albumCol string) *gorm.DB {
	if sc.IsZero() {
		return q
	}
	sql, args := sc.where("tracks")
	return q.Where("EXISTS (SELECT 1 FROM tracks WHERE tracks.album_id = "+albumCol+" AND "+sql+")", args...)
}
