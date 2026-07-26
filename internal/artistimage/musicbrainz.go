package artistimage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/andresbott/aether/internal/upstream"
)

// mbServiceName is what the user sees when MusicBrainz misbehaves.
const mbServiceName = "MusicBrainz"

// Candidate is a simplified MusicBrainz artist search result.
type Candidate struct {
	MBID           string `json:"mbid"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Disambiguation string `json:"disambiguation"`
	LifeSpanBegin  string `json:"lifeSpanBegin"`
	LifeSpanEnd    string `json:"lifeSpanEnd"`
	Score          int    `json:"score"`
}

// ReleaseCandidate is a simplified MusicBrainz release search result. It
// carries both the release MBID and its parent release-group MBID so the
// caller can set MUSICBRAINZ_ALBUMID and MUSICBRAINZ_RELEASEGROUPID from a
// single pick.
type ReleaseCandidate struct {
	ReleaseMBID      string                `json:"releaseMbid"`
	ReleaseGroupMBID string                `json:"releaseGroupMbid"`
	Title            string                `json:"title"`
	Artist           string                `json:"artist"`
	Artists          []ReleaseArtistCredit `json:"artists"`
	Date             string                `json:"date"`
	Country          string                `json:"country"`
	TrackCount       int                   `json:"trackCount"`
	Disambiguation   string                `json:"disambiguation"`
	Score            int                   `json:"score"`
}

// ReleaseArtistCredit is one credited artist on a release: the credited-as
// name (what gets written to the tag) and the artist's own MBID.
type ReleaseArtistCredit struct {
	Name string `json:"name"`
	MBID string `json:"mbid"`
}

// MusicBrainzSearch searches the MusicBrainz artist search API by name. It is
// separate from the Provider interface (image fetch by MBID) — this client
// looks artists up, it does not fetch images.
type MusicBrainzSearch struct {
	BaseURL string
	// Doer carries the throttle, retry policy and error classification shared
	// by all of aether's outbound clients.
	Doer *upstream.Doer
}

func NewMusicBrainzSearch(userAgent string) *MusicBrainzSearch {
	return &MusicBrainzSearch{
		BaseURL: "https://musicbrainz.org",
		Doer:    upstream.New(mbServiceName, userAgent, requestsPerSecond),
	}
}

// getJSON performs a throttled, retrying GET and decodes the JSON body into out.
func (m *MusicBrainzSearch) getJSON(ctx context.Context, url string, out any) error {
	resp, err := m.Doer.Get(ctx, url, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return m.Doer.BadResponse(err)
	}
	return nil
}

type mbArtistSearchResponse struct {
	Artists []struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Type           string `json:"type"`
		Disambiguation string `json:"disambiguation"`
		Score          int    `json:"score"`
		LifeSpan       struct {
			Begin string `json:"begin"`
			End   string `json:"end"`
		} `json:"life-span"`
	} `json:"artists"`
}

// Search looks up artists by name against the MusicBrainz search API,
// returning up to limit candidates ordered by MusicBrainz's own relevance
// score. An empty query returns (nil, nil) without making a request.
func (m *MusicBrainzSearch) Search(ctx context.Context, query string, limit int) ([]Candidate, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	u := fmt.Sprintf("%s/ws/2/artist/?query=%s&fmt=json&limit=%d", m.BaseURL, url.QueryEscape(query), limit)
	var body mbArtistSearchResponse
	if err := m.getJSON(ctx, u, &body); err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(body.Artists))
	for _, a := range body.Artists {
		out = append(out, Candidate{
			MBID:           a.ID,
			Name:           a.Name,
			Type:           a.Type,
			Disambiguation: a.Disambiguation,
			LifeSpanBegin:  a.LifeSpan.Begin,
			LifeSpanEnd:    a.LifeSpan.End,
			Score:          a.Score,
		})
	}
	return out, nil
}

type mbReleaseGroupGenresResponse struct {
	Genres []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"genres"`
}

// maxReleaseGroupGenres caps how many genres a lookup returns; MusicBrainz
// tag lists can carry dozens of low-vote entries.
const maxReleaseGroupGenres = 10

// ReleaseGroupGenres looks up the genres of a release group, ordered by vote
// count descending and capped at maxReleaseGroupGenres. An empty mbid returns
// (nil, nil) without making a request.
func (m *MusicBrainzSearch) ReleaseGroupGenres(ctx context.Context, mbid string) ([]string, error) {
	if mbid == "" {
		return nil, nil
	}
	u := fmt.Sprintf("%s/ws/2/release-group/%s?fmt=json&inc=genres", m.BaseURL, url.PathEscape(mbid))
	var body mbReleaseGroupGenresResponse
	if err := m.getJSON(ctx, u, &body); err != nil {
		return nil, err
	}
	sort.SliceStable(body.Genres, func(i, j int) bool {
		return body.Genres[i].Count > body.Genres[j].Count
	})
	n := len(body.Genres)
	if n > maxReleaseGroupGenres {
		n = maxReleaseGroupGenres
	}
	out := make([]string, 0, n)
	for _, g := range body.Genres[:n] {
		out = append(out, g.Name)
	}
	return out, nil
}

type mbReleaseSearchResponse struct {
	Releases []struct {
		ID             string `json:"id"`
		Title          string `json:"title"`
		Disambiguation string `json:"disambiguation"`
		Date           string `json:"date"`
		Country        string `json:"country"`
		TrackCount     int    `json:"track-count"`
		Score          int    `json:"score"`
		ArtistCredit   []struct {
			Name       string `json:"name"`
			JoinPhrase string `json:"joinphrase"`
			Artist     struct {
				ID string `json:"id"`
			} `json:"artist"`
		} `json:"artist-credit"`
		ReleaseGroup struct {
			ID string `json:"id"`
		} `json:"release-group"`
	} `json:"releases"`
}

// SearchRelease looks up releases by name against the MusicBrainz release
// search API, returning up to limit candidates ordered by MusicBrainz's own
// relevance score. Each candidate carries both the release and its parent
// release-group MBID. An empty query returns (nil, nil) without a request.
func (m *MusicBrainzSearch) SearchRelease(ctx context.Context, query string, limit int) ([]ReleaseCandidate, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	u := fmt.Sprintf("%s/ws/2/release/?query=%s&fmt=json&limit=%d", m.BaseURL, url.QueryEscape(query), limit)
	var body mbReleaseSearchResponse
	if err := m.getJSON(ctx, u, &body); err != nil {
		return nil, err
	}
	out := make([]ReleaseCandidate, 0, len(body.Releases))
	for _, rel := range body.Releases {
		var artist strings.Builder
		credits := make([]ReleaseArtistCredit, 0, len(rel.ArtistCredit))
		for _, ac := range rel.ArtistCredit {
			artist.WriteString(ac.Name)
			artist.WriteString(ac.JoinPhrase)
			credits = append(credits, ReleaseArtistCredit{Name: ac.Name, MBID: ac.Artist.ID})
		}
		out = append(out, ReleaseCandidate{
			ReleaseMBID:      rel.ID,
			ReleaseGroupMBID: rel.ReleaseGroup.ID,
			Title:            rel.Title,
			Artist:           artist.String(),
			Artists:          credits,
			Date:             rel.Date,
			Country:          rel.Country,
			TrackCount:       rel.TrackCount,
			Disambiguation:   rel.Disambiguation,
			Score:            rel.Score,
		})
	}
	return out, nil
}
