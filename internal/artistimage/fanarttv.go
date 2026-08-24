package artistimage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/andresbott/aether/internal/upstream"
)

type FanartTV struct {
	APIKey  string
	BaseURL string
	// Doer carries the throttle, retry policy and error classification shared
	// by all of aether's outbound clients.
	Doer *upstream.Doer
}

func NewFanartTV(apiKey string) *FanartTV {
	return &FanartTV{
		APIKey:  apiKey,
		BaseURL: "https://webservice.fanart.tv",
		Doer:    upstream.New("fanart.tv", "", requestsPerSecond),
	}
}

func (p *FanartTV) Name() string { return "fanart.tv" }

func (p *FanartTV) List(ctx context.Context, mbid string) ([]ImageCandidate, error) {
	if p.APIKey == "" || mbid == "" {
		return nil, nil
	}
	u := fmt.Sprintf("%s/v3/music/%s?api_key=%s", p.BaseURL, mbid, p.APIKey)
	resp, err := p.Doer.Get(ctx, u, nil)
	if err != nil {
		if upstream.IsRejected(err) {
			return nil, nil // 4xx = "no artwork for this MBID", not an error
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		ArtistThumb []struct {
			URL string `json:"url"`
		} `json:"artistthumb"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	var out []ImageCandidate
	for _, t := range body.ArtistThumb {
		if t.URL == "" {
			continue
		}
		out = append(out, ImageCandidate{
			FullURL:  t.URL,
			ThumbURL: fanartPreviewURL(t.URL),
			Provider: p.Name(),
		})
	}
	return out, nil
}

func (p *FanartTV) Download(ctx context.Context, url string) ([]byte, string, error) {
	return download(ctx, p.Doer, url)
}

// fanartPreviewURL turns a full asset URL into its lighter preview variant, which
// loads faster and does not increment fanart.tv's download counter.
func fanartPreviewURL(full string) string {
	if strings.Contains(full, "/fanart/") {
		return strings.Replace(full, "/fanart/", "/preview/", 1)
	}
	return full
}

// download fetches imageURL through doer, so the image download shares the
// provider's fair-use throttle and retry policy.
func download(ctx context.Context, doer *upstream.Doer, imageURL string) ([]byte, string, error) {
	resp, err := doer.Get(ctx, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, "", doer.BadResponse(err)
	}
	return data, extFromURL(imageURL), nil
}
