package artistimage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/andresbott/aether/internal/upstream"
)

type TheAudioDB struct {
	APIKey  string
	BaseURL string
	// Doer carries the throttle, retry policy and error classification shared
	// by all of aether's outbound clients.
	Doer *upstream.Doer
}

func NewTheAudioDB(apiKey string) *TheAudioDB {
	return &TheAudioDB{
		APIKey:  apiKey,
		BaseURL: "https://www.theaudiodb.com",
		Doer:    upstream.New("TheAudioDB", "", requestsPerSecond),
	}
}

func (p *TheAudioDB) Name() string { return "theaudiodb" }

func (p *TheAudioDB) Fetch(ctx context.Context, mbid string) ([]byte, string, error) {
	if p.APIKey == "" || mbid == "" {
		return nil, "", nil
	}
	u := fmt.Sprintf("%s/api/v1/json/%s/artist-mb.php?i=%s", p.BaseURL, p.APIKey, mbid)
	// As with fanart.tv, a refusal means "no artwork here", not a failure.
	resp, err := p.Doer.Get(ctx, u, nil)
	if err != nil {
		if upstream.IsRejected(err) {
			return nil, "", nil
		}
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Artists []struct {
			Thumb string `json:"strArtistThumb"`
		} `json:"artists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, "", err
	}
	if len(body.Artists) == 0 || body.Artists[0].Thumb == "" {
		return nil, "", nil
	}
	return download(ctx, p.Doer, body.Artists[0].Thumb)
}

func (p *TheAudioDB) List(ctx context.Context, mbid string) ([]ImageCandidate, error) {
	if p.APIKey == "" || mbid == "" {
		return nil, nil
	}
	u := fmt.Sprintf("%s/api/v1/json/%s/artist-mb.php?i=%s", p.BaseURL, p.APIKey, mbid)
	resp, err := p.Doer.Get(ctx, u, nil)
	if err != nil {
		if upstream.IsRejected(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Artists []struct {
			Thumb string `json:"strArtistThumb"`
		} `json:"artists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if len(body.Artists) == 0 || body.Artists[0].Thumb == "" {
		return nil, nil
	}
	full := body.Artists[0].Thumb
	return []ImageCandidate{{
		FullURL:  full,
		ThumbURL: full + "/preview",
		Provider: p.Name(),
	}}, nil
}

func (p *TheAudioDB) Download(ctx context.Context, url string) ([]byte, string, error) {
	return download(ctx, p.Doer, url)
}
