package router

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth/service/pat"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// mintUserToken creates a usertoken PAT for the logged-in session and returns
// the virtual username and password.
func mintUserToken(t *testing.T, h *MainAppHandler, attach func(*http.Request), name string) (username, password string) {
	t.Helper()
	body := strings.NewReader(`{"name":"` + name + `","type":"usertoken"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", body)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("mint usertoken: status %d, body %s", w.Code, w.Body.String())
	}
	var res struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res.Username, res.Password
}

// subsonicCode pings /rest with the given query and returns the subsonic
// error code, or 0 for an ok response.
func subsonicCode(t *testing.T, h *MainAppHandler, query string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/rest/ping?f=json&"+query, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var body struct {
		SubsonicResponse struct {
			Status string `json:"status"`
			Error  *struct {
				Code int `json:"code"`
			} `json:"error"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad body %q: %v", w.Body.String(), err)
	}
	if body.SubsonicResponse.Status == "ok" {
		return 0
	}
	if body.SubsonicResponse.Error == nil {
		t.Fatalf("failed status without error element: %s", w.Body.String())
	}
	return body.SubsonicResponse.Error.Code
}

func saltedToken(password, salt string) string {
	sum := md5.Sum([]byte(password + salt))
	return hex.EncodeToString(sum[:])
}

func TestRestUserTokenSaltedAuth(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	user, pw := mintUserToken(t, h, attach, "phone")

	salt := "c19b2d"
	tok := saltedToken(pw, salt)

	if code := subsonicCode(t, h, "u="+user+"&t="+tok+"&s="+salt); code != 0 {
		t.Errorf("valid t+s auth: code %d, want ok", code)
	}
	// Case-mangled virtual username still authenticates.
	if code := subsonicCode(t, h, "u="+strings.ToUpper(user)+"&t="+tok+"&s="+salt); code != 0 {
		t.Errorf("upper-cased u: code %d, want ok", code)
	}
	// Uppercase hex t param (some clients uppercase the digest).
	if code := subsonicCode(t, h, "u="+user+"&t="+strings.ToUpper(tok)+"&s="+salt); code != 0 {
		t.Errorf("upper-cased t: code %d, want ok", code)
	}
	// Wrong token digest → 40.
	if code := subsonicCode(t, h, "u="+user+"&t=ffffffffffffffffffffffffffffffff&s="+salt); code != 40 {
		t.Errorf("wrong t: code %d, want 40", code)
	}
	// Missing salt → 40.
	if code := subsonicCode(t, h, "u="+user+"&t="+tok); code != 40 {
		t.Errorf("t without s: code %d, want 40", code)
	}
}

func TestRestUserTokenPlainPassword(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	user, pw := mintUserToken(t, h, attach, "legacy-app")

	if code := subsonicCode(t, h, "u="+user+"&p="+pw); code != 0 {
		t.Errorf("valid p auth: code %d, want ok", code)
	}
	// enc: hex obfuscation per spec.
	if code := subsonicCode(t, h, "u="+user+"&p=enc:"+hex.EncodeToString([]byte(pw))); code != 0 {
		t.Errorf("enc: p auth: code %d, want ok", code)
	}
	if code := subsonicCode(t, h, "u="+user+"&p=wrong"); code != 40 {
		t.Errorf("wrong p: code %d, want 40", code)
	}
}

func TestRestRealLoginAnswers41(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	// bob is a real login, not a virtual username: token auth unsupported.
	if code := subsonicCode(t, h, "u=bob&t=ffffffffffffffffffffffffffffffff&s=abc123"); code != 41 {
		t.Errorf("real login via t+s: code %d, want 41", code)
	}
	// Same for the password param: the login password NEVER works on /rest.
	if code := subsonicCode(t, h, "u=bob&p=secret"); code != 41 {
		t.Errorf("real login via p: code %d, want 41", code)
	}
}

func TestRestApikeyTokenIDAnswers41OnTS(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	// An apikey-type PAT's tokenID used as a username: recoverable storage
	// is absent for it → 41 (token auth not supported for this user).
	body := strings.NewReader(`{"name":"scripted","type":"apikey"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", body)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var res struct {
		TokenID string `json:"tokenId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if code := subsonicCode(t, h, "u="+res.TokenID+"&t=ffffffffffffffffffffffffffffffff&s=abc123"); code != 41 {
		t.Errorf("apikey id via t+s: code %d, want 41", code)
	}
}

func TestRestAuthParamMatrix(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	user, pw := mintUserToken(t, h, attach, "matrix")
	salt := "abc123"
	tok := saltedToken(pw, salt)

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"no credentials at all", "", 40},
		{"u alone", "u=" + user, 40},
		{"unknown virtual user", "u=zzzzzzzzzz&t=" + tok + "&s=" + salt, 40},
		{"apiKey mixed with u", "apiKey=aether_x_y&u=" + user, 43},
		{"apiKey mixed with t+s", "apiKey=aether_x_y&t=" + tok + "&s=" + salt, 43},
		{"apiKey mixed with p", "apiKey=aether_x_y&p=" + pw, 43},
		{"t+s AND p together still resolves via t+s", "u=" + user + "&t=" + tok + "&s=" + salt + "&p=" + pw, 0},
		{"invalid apiKey alone", "apiKey=aether_zzzzzzzzzz_wrong", 44},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if code := subsonicCode(t, h, tc.query); code != tc.want {
				t.Errorf("code = %d, want %d", code, tc.want)
			}
		})
	}
}

func TestRestUserTokenAlsoWorksAsApiKey(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	user, pw := mintUserToken(t, h, attach, "dual")
	if code := subsonicCode(t, h, "apiKey=aether_"+user+"_"+pw); code != 0 {
		t.Errorf("usertoken via apiKey: code %d, want ok", code)
	}
}

func TestRestOwnerIsRealLoginNotVirtualUser(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	user, pw := mintUserToken(t, h, attach, "owner-check")
	salt := "abc123"

	// getPlayQueue is owner-scoped; an empty queue is a normal "ok" answer.
	// The point: authenticating via the virtual username must resolve to
	// bob's data, which savePlayQueue+getPlayQueue round-trips.
	q := "u=" + user + "&t=" + saltedToken(pw, salt) + "&s=" + salt
	req := httptest.NewRequest(http.MethodGet, "/rest/getNowPlaying?f=json&"+q, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("getNowPlaying via usertoken failed: %s", w.Body.String())
	}
}

func TestRestCredentialMaskingWithPercentEncodingAndCollisions(t *testing.T) {
	// This test ensures F2 (log-masking fix) works correctly: the middleware
	// operates on the raw query string, not URL-decoded values, so
	// percent-encoded params are masked, and anchored key matching prevents
	// corruption of other params that happen to contain the same substring.
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")
	user, pw := mintUserToken(t, h, attach, "mask-test")

	// Capture RequestURI from within the handler chain.
	capture := func(query string) string {
		var captured string
		h.router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured = r.RequestURI
				next.ServeHTTP(w, r)
			})
		})
		req := httptest.NewRequest(http.MethodGet, "/rest/ping?f=json&"+query, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return captured
	}

	t.Run("apiKey masking", func(t *testing.T) {
		uri := capture("apiKey=aether_" + user + "_" + pw)
		if strings.Contains(uri, pw) {
			t.Errorf("RequestURI still contains the plaintext token: %s", uri)
		}
		if !strings.Contains(uri, "apiKey=***") {
			t.Errorf("RequestURI did not mask apiKey: %s", uri)
		}
	})

	t.Run("percent-encoded p value", func(t *testing.T) {
		// Percent-encoded p value: the middleware must mask it without
		// decoding. The raw query string contains "enc%3A...", and that
		// entire segment should become p=***.
		encPw := "enc%3A" + hex.EncodeToString([]byte(pw))
		uri := capture("u=" + user + "&p=" + encPw)
		if strings.Contains(uri, encPw) {
			t.Errorf("RequestURI still contains the percent-encoded password: %s", uri)
		}
		if !strings.Contains(uri, "p=***") {
			t.Errorf("RequestURI did not mask p: %s", uri)
		}
	})

	t.Run("no param collision corruption", func(t *testing.T) {
		salt := "abc123"
		tok := saltedToken(pw, salt)
		// s happens to match a substring in another param value: the
		// masking must be anchored to the param key, not a global replace.
		uri := capture("u=" + user + "&t=" + tok + "&s=" + salt + "&params=abc123")
		if !strings.Contains(uri, "s=***") {
			t.Errorf("s not masked: %s", uri)
		}
		// params must NOT be corrupted.
		if !strings.Contains(uri, "params=abc123") {
			t.Errorf("params was corrupted by masking: %s", uri)
		}
	})
}

func TestRestNilUsersDoesNotPanicOn41Disambiguation(t *testing.T) {
	// A router with native auth but no Users store (violates the validation,
	// but exercises the nil-guard for h.users.GetUserByLogin): must answer
	// 40, not panic.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	users, err := userdb.New(db, userdb.Opts{BcryptDifficulty: bcrypt.MinCost, DefaultEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := pat.NewAESGCMCipher(bytes.Repeat([]byte{0x42}, 32), "k1")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pat.NewService(users.PATStore(), users, pat.Opts{Prefix: "aether", Cipher: cipher})
	if err != nil {
		t.Fatal(err)
	}
	// Omit Users from the config: h.users will be nil.
	h, err := New(Cfg{
		AuthMethod: "native",
		Tokens:     tokens,
		Store:      store.New(db),
		DataDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Unknown virtual username with no user store to check against: must
	// answer 40, not panic trying to dereference h.users.
	if code := subsonicCode(t, h, "u=zzzzzzzzzz&p=nope"); code != 40 {
		t.Errorf("unknown user with nil Users = %d, want 40", code)
	}
}
