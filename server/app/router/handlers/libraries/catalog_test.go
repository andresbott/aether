package libraries_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type catalogBody struct {
	SplitViews bool `json:"split_views"`
}

func decodeCatalog(t *testing.T, code, want int, body []byte) catalogBody {
	t.Helper()
	if code != want {
		t.Fatalf("expected %d, got %d, body=%s", want, code, body)
	}
	var got catalogBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// The root library splits its views until told otherwise; an update that
// omits split_views keeps what is stored.
func TestCatalogSettings(t *testing.T) {
	_, _, r := newTestHandler(t)
	w := send(t, r, "GET", "/libraries/catalog", "")
	if got := decodeCatalog(t, w.Code, http.StatusOK, w.Body.Bytes()); !got.SplitViews {
		t.Fatal("expected split_views=true by default")
	}
	w = send(t, r, "PUT", "/libraries/catalog", `{"split_views":false}`)
	if got := decodeCatalog(t, w.Code, http.StatusOK, w.Body.Bytes()); got.SplitViews {
		t.Fatal("expected split_views=false after the update")
	}
	w = send(t, r, "PUT", "/libraries/catalog", `{}`)
	if got := decodeCatalog(t, w.Code, http.StatusOK, w.Body.Bytes()); got.SplitViews {
		t.Fatal("expected split_views=false kept on an omitted key")
	}
	w = send(t, r, "GET", "/libraries/catalog", "")
	if got := decodeCatalog(t, w.Code, http.StatusOK, w.Body.Bytes()); got.SplitViews {
		t.Fatal("expected split_views=false read back")
	}
	if w = send(t, r, "PUT", "/libraries/catalog", `{`); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed JSON, got %d", w.Code)
	}
}
