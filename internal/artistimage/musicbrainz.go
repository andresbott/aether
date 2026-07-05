package artistimage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

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

// MusicBrainzSearch searches the MusicBrainz artist search API by name. It is
// separate from the Provider interface (image fetch by MBID) — this client
// looks artists up, it does not fetch images.
type MusicBrainzSearch struct {
	BaseURL   string
	UserAgent string
	Client    *http.Client
	limiter   *rate.Limiter
}

func NewMusicBrainzSearch(userAgent string) *MusicBrainzSearch {
	return &MusicBrainzSearch{
		BaseURL:   "https://musicbrainz.org",
		UserAgent: userAgent,
		Client:    &http.Client{Timeout: 20 * time.Second},
		limiter:   rate.NewLimiter(requestsPerSecond, 1),
	}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", m.UserAgent)
	if err := m.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz search: status %d", resp.StatusCode)
	}
	var body mbArtistSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
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
