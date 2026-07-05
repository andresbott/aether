package artistimage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type TheAudioDB struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
	limiter *rate.Limiter
}

func NewTheAudioDB(apiKey string) *TheAudioDB {
	return &TheAudioDB{
		APIKey:  apiKey,
		BaseURL: "https://www.theaudiodb.com",
		Client:  &http.Client{Timeout: 20 * time.Second},
		limiter: rate.NewLimiter(requestsPerSecond, 1),
	}
}

func (p *TheAudioDB) Name() string { return "theaudiodb" }

func (p *TheAudioDB) Fetch(ctx context.Context, mbid string) ([]byte, string, error) {
	if p.APIKey == "" || mbid == "" {
		return nil, "", nil
	}
	u := fmt.Sprintf("%s/api/v1/json/%s/artist-mb.php?i=%s", p.BaseURL, p.APIKey, mbid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, "", err
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", nil
	}
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
	return download(ctx, p.limiter, p.Client, body.Artists[0].Thumb)
}
