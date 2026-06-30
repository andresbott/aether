package artistimage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFanartTVFetch(t *testing.T) {
	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/v3/music/mbid-1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"artistthumb":[{"url":"` + base + `/img.png","likes":"3"}]}`))
	})
	mux.HandleFunc("/img.png", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("PNGDATA"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL
	p := NewFanartTV("key")
	p.BaseURL = srv.URL
	p.Client = srv.Client()

	data, ext, err := p.Fetch(context.Background(), "mbid-1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(data) != "PNGDATA" || ext != "png" {
		t.Fatalf("got data=%q ext=%q", data, ext)
	}
}

func TestEmptyKeySkips(t *testing.T) {
	p := NewFanartTV("")
	data, _, err := p.Fetch(context.Background(), "x")
	if err != nil || data != nil {
		t.Fatalf("empty key should skip, got data=%v err=%v", data, err)
	}
}

func TestChainFallsThrough(t *testing.T) {
	empty := stubProvider{}
	hit := stubProvider{data: []byte("X"), ext: "jpg"}
	c := NewChain(empty, hit)
	data, ext, err := c.Fetch(context.Background(), "m")
	if err != nil || string(data) != "X" || ext != "jpg" {
		t.Fatalf("chain: data=%q ext=%q err=%v", data, ext, err)
	}
}

type stubProvider struct {
	data []byte
	ext  string
}

func (s stubProvider) Name() string { return "stub" }
func (s stubProvider) Fetch(_ context.Context, _ string) ([]byte, string, error) {
	return s.data, s.ext, nil
}
