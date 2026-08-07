package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth/service/pat"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func getMe(t *testing.T, h *MainAppHandler) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	return w.Code, body
}

// The SPA bootstraps on /me: it must be public and answer even on a server
// with nothing configured, defaulting the auth method to "none".
func TestMeWithoutAuth(t *testing.T) {
	h := newTestRouter(t)
	code, body := getMe(t, h)

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if body["authMethod"] != "none" {
		t.Errorf("authMethod = %v, want none", body["authMethod"])
	}
	if body["user"] != nil {
		t.Errorf("user = %v, want null (no sessions yet)", body["user"])
	}
	features, ok := body["features"].(map[string]any)
	if !ok {
		t.Fatalf("features missing: %v", body)
	}
	if features["userManagement"] != false {
		t.Errorf("features.userManagement = %v, want false", features["userManagement"])
	}
}

func TestMeWithNativeAuth(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	users, err := userdb.New(db, userdb.Opts{BcryptDifficulty: bcrypt.MinCost, DefaultEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pat.NewService(users.PATStore(), users, pat.Opts{Prefix: "aether"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(Cfg{AuthMethod: "native", Users: users, Tokens: tokens})
	if err != nil {
		t.Fatal(err)
	}

	code, body := getMe(t, h)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if body["authMethod"] != "native" {
		t.Errorf("authMethod = %v, want native", body["authMethod"])
	}
	if features, _ := body["features"].(map[string]any); features["userManagement"] != true {
		t.Errorf("features.userManagement = %v, want true", features["userManagement"])
	}

	// With the store present the users CRUD is mounted.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET /api/v1/users = %d, want 200 with native auth: %s", w.Code, w.Body.String())
	}
}

// With auth method "none" there is no user store and the users routes must not
// exist at all — the request falls through to the /api/v1 catch-all.
func TestUsersAPIIsNotMountedWithoutNativeAuth(t *testing.T) {
	h := newTestRouter(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/v1/users = %d, want the 400 catch-all: %s", w.Code, w.Body.String())
	}
}
