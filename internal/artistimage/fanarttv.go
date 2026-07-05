package artistimage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type FanartTV struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
	limiter *rate.Limiter
}

func NewFanartTV(apiKey string) *FanartTV {
	return &FanartTV{
		APIKey:  apiKey,
		BaseURL: "https://webservice.fanart.tv",
		Client:  &http.Client{Timeout: 20 * time.Second},
		limiter: rate.NewLimiter(requestsPerSecond, 1),
	}
}

func (p *FanartTV) Name() string { return "fanart.tv" }

func (p *FanartTV) Fetch(ctx context.Context, mbid string) ([]byte, string, error) {
	if p.APIKey == "" || mbid == "" {
		return nil, "", nil
	}
	u := fmt.Sprintf("%s/v3/music/%s?api_key=%s", p.BaseURL, mbid, p.APIKey)
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
		return nil, "", nil // 404 = no artwork; not an error
	}
	var body struct {
		ArtistThumb []struct {
			URL string `json:"url"`
		} `json:"artistthumb"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, "", err
	}
	if len(body.ArtistThumb) == 0 || body.ArtistThumb[0].URL == "" {
		return nil, "", nil
	}
	return download(ctx, p.limiter, p.Client, body.ArtistThumb[0].URL)
}

// download fetches imageURL and returns its bytes and a normalized extension.
// It waits on limiter before the request so downloads count toward the
// provider's fair-use rate.
func download(ctx context.Context, limiter *rate.Limiter, client *http.Client, imageURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	if err := limiter.Wait(ctx); err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download %s: status %d", imageURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, "", err
	}
	return data, extFromURL(imageURL), nil
}
