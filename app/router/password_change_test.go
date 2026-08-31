package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-bumbu/userauth/service/throttle"
	throttlemem "github.com/go-bumbu/userauth/service/throttle/store/memory"
	"gorm.io/gorm"
)

// doChangePassword posts a change-own-password request through attach (the
// caller's session cookies) and returns the recorder.
func doChangePassword(t *testing.T, h *MainAppHandler, attach func(*http.Request), current, next string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"currentPassword":"` + current + `","newPassword":"` + next + `"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/password", body)
	if attach != nil {
		attach(req)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// meRole fetches /api/v1/me through attach and returns the reported role, or ""
// when anonymous.
func meRole(t *testing.T, h *MainAppHandler, attach func(*http.Request)) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	attach(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var body struct {
		User *struct {
			Login string `json:"login"`
			Role  string `json:"role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/me body is not JSON: %s", w.Body.String())
	}
	if body.User == nil {
		return ""
	}
	return body.User.Role
}

// A regular (non-admin) user changes their own password: the session-tier
// route accepts them, the old password stops working and the new one logs in.
// A successful change signs the caller out — the response clears the session
// cookie — so they must sign back in with the new password.
func TestChangePasswordUpdatesCredential(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	w := doChangePassword(t, h, attach, "secret", "brand-new-pw")
	if w.Code != http.StatusNoContent {
		t.Fatalf("change password = %d, want 204: %s", w.Code, w.Body.String())
	}
	// A browser applies the response's Set-Cookie, replacing the session cookie
	// with the cleared one: /me carrying the response cookies reports anonymous,
	// so this device is signed out.
	if len(w.Result().Cookies()) == 0 {
		t.Fatal("change password did not clear the session cookie")
	}
	signedOut := func(r *http.Request) {
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}
	}
	if role := meRole(t, h, signedOut); role != "" {
		t.Fatalf("caller still signed in after a password change: /me role = %q, want anonymous", role)
	}

	// The old password no longer authenticates; the new one does.
	if wl, _ := doLogin(t, h, "bob", "secret"); wl.Code != http.StatusUnauthorized {
		t.Errorf("login with old password = %d, want 401", wl.Code)
	}
	if wl, _ := doLogin(t, h, "bob", "brand-new-pw"); wl.Code != http.StatusOK {
		t.Errorf("login with new password = %d, want 200: %s", wl.Code, wl.Body.String())
	}
}

// A wrong current password is refused with 403 (not 401) and leaves the
// credential untouched. 403 is deliberate: the session is still valid, so a
// 401 would make the SPA treat it as an expired session and sign the caller
// out instead of showing a form error.
func TestChangePasswordRejectsWrongCurrent(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	w := doChangePassword(t, h, attach, "not-my-password", "brand-new-pw")
	if w.Code != http.StatusForbidden {
		t.Fatalf("change password with wrong current = %d, want 403: %s", w.Code, w.Body.String())
	}
	if wl, _ := doLogin(t, h, "bob", "secret"); wl.Code != http.StatusOK {
		t.Errorf("original password stopped working after a failed change: %d", wl.Code)
	}
	// The re-issued session must still work: a failed change does not sign the
	// caller out. The cookie from login is still valid.
	if role := meRole(t, h, attach); role != "user" {
		t.Errorf("session dropped after a wrong-password change: /me role = %q, want user", role)
	}
}

// Without a session the route is unreachable, exactly like every other guarded
// /api/v1 route.
func TestChangePasswordRequiresSession(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	w := doChangePassword(t, h, nil, "secret", "brand-new-pw")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("change password without a session = %d, want 401: %s", w.Code, w.Body.String())
	}
}

// The new password runs through the same validation as the admin CRUD: empty
// is a 400 missing field, over-length is a 422 validation problem.
func TestChangePasswordValidatesNewPassword(t *testing.T) {
	h, _ := newNativeAuthRouter(t)
	_, attach := doLogin(t, h, "bob", "secret")

	if w := doChangePassword(t, h, attach, "secret", ""); w.Code != http.StatusBadRequest {
		t.Errorf("empty new password = %d, want 400: %s", w.Code, w.Body.String())
	}
	if w := doChangePassword(t, h, attach, "secret", strings.Repeat("x", 73)); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("over-length new password = %d, want 422: %s", w.Code, w.Body.String())
	}
	// A rejected new password never touches the stored credential.
	if wl, _ := doLogin(t, h, "bob", "secret"); wl.Code != http.StatusOK {
		t.Errorf("password changed despite validation failure: %d", wl.Code)
	}
}

// Repeated wrong current-password guesses escalate to a 429: the handler must
// throttle the re-auth so a stolen session cannot brute-force the password.
func TestChangePasswordThrottlesWrongCurrent(t *testing.T) {
	aggressive := func(t *testing.T, cfg *Cfg, _ *gorm.DB) {
		// One free failure, then a long backoff: the second wrong guess is
		// denied without even reaching the verifier.
		cfg.Reauth = &throttle.Backoff{Store: throttlemem.New(), FreeFailures: 1, BaseDelay: time.Hour}
	}
	h, _ := newNativeAuthRouter(t, aggressive)
	_, attach := doLogin(t, h, "bob", "secret")

	if w := doChangePassword(t, h, attach, "wrong-1", "brand-new-pw"); w.Code != http.StatusForbidden {
		t.Fatalf("first wrong current = %d, want 403: %s", w.Code, w.Body.String())
	}
	if w := doChangePassword(t, h, attach, "wrong-2", "brand-new-pw"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second wrong current = %d, want 429: %s", w.Code, w.Body.String())
	}
}
