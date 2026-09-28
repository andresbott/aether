package store

import (
	"sort"
	"strings"
)

// FilterOptions lists the values present in the catalog for the filter fields
// whose vocabulary comes from the data: what a filter builder offers.
type FilterOptions struct {
	Formats      []string
	Genres       []string
	ReleaseTypes []string
}

// FilterOptions reads the distinct file formats, the genres that tag at least
// one track, and the release types albums carry. Release types are matched
// case-insensitively by the filter, so spellings that differ only in case are
// offered once.
func (s *Store) FilterOptions() (FilterOptions, error) {
	var out FilterOptions
	if err := s.db.Raw(`SELECT DISTINCT suffix FROM tracks WHERE suffix <> '' ORDER BY suffix`).
		Scan(&out.Formats).Error; err != nil {
		return FilterOptions{}, err
	}
	if err := s.db.Raw(`SELECT DISTINCT g.name FROM genres g JOIN track_genres tg ON tg.genre_id = g.id ORDER BY g.name`).
		Scan(&out.Genres).Error; err != nil {
		return FilterOptions{}, err
	}
	var types []string
	// je.type = 'text' skips what a nil slice serializes to: the JSON literal
	// null, which json_each yields as one row whose value is NULL.
	if err := s.db.Raw(`SELECT DISTINCT je.value FROM albums a, json_each(a.release_types) je
		WHERE je.type = 'text' ORDER BY je.value`).Scan(&types).Error; err != nil {
		return FilterOptions{}, err
	}
	seen := map[string]bool{}
	for _, t := range types {
		key := strings.ToLower(t)
		if t == "" || seen[key] {
			continue
		}
		seen[key] = true
		out.ReleaseTypes = append(out.ReleaseTypes, t)
	}
	sort.Strings(out.ReleaseTypes)
	return out, nil
}
