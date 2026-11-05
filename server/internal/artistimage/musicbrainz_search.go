package artistimage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// ReleaseGroupCandidate is a simplified MusicBrainz release-group search
// result: the album across all its editions.
type ReleaseGroupCandidate struct {
	MBID             string   `json:"mbid"`
	Title            string   `json:"title"`
	Artist           string   `json:"artist"`
	PrimaryType      string   `json:"primaryType"`
	SecondaryTypes   []string `json:"secondaryTypes"`
	FirstReleaseDate string   `json:"firstReleaseDate"`
	Disambiguation   string   `json:"disambiguation"`
	Score            int      `json:"score"`
}

// RecordingCandidate is a simplified MusicBrainz recording search result: a
// song, whatever release it appears on. Release is the title of one release
// carrying it — an album's in preference to a single's or a compilation's.
type RecordingCandidate struct {
	MBID             string `json:"mbid"`
	Title            string `json:"title"`
	Artist           string `json:"artist"`
	Release          string `json:"release"`
	FirstReleaseDate string `json:"firstReleaseDate"`
	DurationSeconds  int    `json:"durationSeconds"`
	Disambiguation   string `json:"disambiguation"`
	Score            int    `json:"score"`
}

type mbArtistCredit []struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
}

func (c mbArtistCredit) String() string {
	var b strings.Builder
	for _, ac := range c {
		b.WriteString(ac.Name)
		b.WriteString(ac.JoinPhrase)
	}
	return b.String()
}

type mbReleaseGroupSearchResponse struct {
	ReleaseGroups []struct {
		ID               string         `json:"id"`
		Title            string         `json:"title"`
		PrimaryType      string         `json:"primary-type"`
		SecondaryTypes   []string       `json:"secondary-types"`
		FirstReleaseDate string         `json:"first-release-date"`
		Disambiguation   string         `json:"disambiguation"`
		Score            int            `json:"score"`
		ArtistCredit     mbArtistCredit `json:"artist-credit"`
	} `json:"release-groups"`
}

// SearchReleaseGroups looks up release groups (albums) with a MusicBrainz
// search query — build one with SearchQuery — returning up to limit candidates
// in MusicBrainz's relevance order. An empty query returns (nil, nil) without
// a request.
func (m *MusicBrainzSearch) SearchReleaseGroups(ctx context.Context, query string, limit int) ([]ReleaseGroupCandidate, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	u := fmt.Sprintf("%s/ws/2/release-group/?query=%s&fmt=json&limit=%d", m.BaseURL, url.QueryEscape(query), limit)
	var body mbReleaseGroupSearchResponse
	if err := m.getJSON(ctx, u, &body); err != nil {
		return nil, err
	}
	out := make([]ReleaseGroupCandidate, 0, len(body.ReleaseGroups))
	for _, rg := range body.ReleaseGroups {
		out = append(out, ReleaseGroupCandidate{
			MBID:             rg.ID,
			Title:            rg.Title,
			Artist:           rg.ArtistCredit.String(),
			PrimaryType:      rg.PrimaryType,
			SecondaryTypes:   rg.SecondaryTypes,
			FirstReleaseDate: rg.FirstReleaseDate,
			Disambiguation:   rg.Disambiguation,
			Score:            rg.Score,
		})
	}
	return out, nil
}

type mbRecordingSearchResponse struct {
	Recordings []struct {
		ID               string         `json:"id"`
		Title            string         `json:"title"`
		Length           int            `json:"length"`
		FirstReleaseDate string         `json:"first-release-date"`
		Disambiguation   string         `json:"disambiguation"`
		Score            int            `json:"score"`
		ArtistCredit     mbArtistCredit `json:"artist-credit"`
		Releases         []struct {
			Title        string `json:"title"`
			ReleaseGroup struct {
				PrimaryType    string   `json:"primary-type"`
				SecondaryTypes []string `json:"secondary-types"`
			} `json:"release-group"`
		} `json:"releases"`
	} `json:"recordings"`
}

// SearchRecordings looks up recordings (songs) with a MusicBrainz search query
// — build one with SearchQuery — returning up to limit candidates in
// MusicBrainz's relevance order. An empty query returns (nil, nil) without a
// request.
func (m *MusicBrainzSearch) SearchRecordings(ctx context.Context, query string, limit int) ([]RecordingCandidate, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	u := fmt.Sprintf("%s/ws/2/recording/?query=%s&fmt=json&limit=%d", m.BaseURL, url.QueryEscape(query), limit)
	var body mbRecordingSearchResponse
	if err := m.getJSON(ctx, u, &body); err != nil {
		return nil, err
	}
	out := make([]RecordingCandidate, 0, len(body.Recordings))
	for _, rec := range body.Recordings {
		release := ""
		for i, rel := range rec.Releases {
			// A studio album (primary Album, no secondary type such as
			// Compilation or Live) is where a listener knows the song from.
			if rel.ReleaseGroup.PrimaryType == "Album" && len(rel.ReleaseGroup.SecondaryTypes) == 0 {
				release = rel.Title
				break
			}
			if i == 0 {
				release = rel.Title
			}
		}
		out = append(out, RecordingCandidate{
			MBID:             rec.ID,
			Title:            rec.Title,
			Artist:           rec.ArtistCredit.String(),
			Release:          release,
			FirstReleaseDate: rec.FirstReleaseDate,
			DurationSeconds:  rec.Length / 1000,
			Disambiguation:   rec.Disambiguation,
			Score:            rec.Score,
		})
	}
	return out, nil
}

// luceneSpecial are the characters MusicBrainz's Lucene query syntax treats
// as operators.
const luceneSpecial = `+-&|!(){}[]^"~*?:\/`

// escapeLucene backslash-escapes every Lucene operator character in s, so
// user text is searched for literally.
func escapeLucene(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(luceneSpecial, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SearchQuery builds a MusicBrainz search query from free user text, searched
// in the entity's default fields — which for release groups and recordings
// already weigh the artist credit, so "in utero nirvana" finds Nirvana's album.
// The text is escaped, so it cannot inject query syntax. Empty text gives an
// empty query.
func SearchQuery(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return "(" + escapeLucene(text) + ")"
}
