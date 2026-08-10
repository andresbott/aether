package users_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/users"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func newTestHandler(t *testing.T) (*userdb.Store, *mux.Router) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store, err := userdb.New(db, userdb.Opts{BcryptDifficulty: bcrypt.MinCost, DefaultEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	h := &users.Handler{Users: store}
	r := mux.NewRouter()
	h.Routes(r)
	return store, r
}

// mustCreate creates a user directly in the store and returns its stable UUID,
// which is what the update/delete routes are addressed by.
func mustCreate(t *testing.T, store *userdb.Store, login, pw string) string {
	t.Helper()
	if err := store.Create(login, pw); err != nil {
		t.Fatal(err)
	}
	usr, err := store.GetUserByLogin(login)
	if err != nil {
		t.Fatal(err)
	}
	return usr.ID
}

// mustAdmin creates an enabled user already in the admin group, for tests that
// need a specific admin population rather than a specific request.
func mustAdmin(t *testing.T, store *userdb.Store, login string) string {
	t.Helper()
	id := mustCreate(t, store, login, "pw")
	if err := store.SetGroups(id, []string{users.AdminGroup}); err != nil {
		t.Fatal(err)
	}
	return id
}

func mustRole(t *testing.T, store *userdb.Store, id string) string {
	t.Helper()
	role, err := users.RoleOf(store, id)
	if err != nil {
		t.Fatal(err)
	}
	return role
}

func mustGroups(t *testing.T, store *userdb.Store, id string) []string {
	t.Helper()
	groups, err := store.GetGroups(id)
	if err != nil {
		t.Fatal(err)
	}
	return groups
}

func doJSON(t *testing.T, r *mux.Router, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestListUsers(t *testing.T) {
	store, r := newTestHandler(t)

	w := doJSON(t, r, http.MethodGet, "/users", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		Users []map[string]any `json:"users"`
		Total int              `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Users) != 0 || body.Total != 0 {
		t.Fatalf("expected empty list, got %+v", body)
	}

	id := mustCreate(t, store, "alice", "pw")
	w = doJSON(t, r, http.MethodGet, "/users", "")
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body.Users) != 1 || body.Total != 1 {
		t.Fatalf("expected one user, got %+v", body)
	}
	if body.Users[0]["login"] != "alice" || body.Users[0]["enabled"] != true {
		t.Fatalf("unexpected user payload: %+v", body.Users[0])
	}
	if body.Users[0]["id"] != id {
		t.Fatalf("expected id %q, got %v", id, body.Users[0]["id"])
	}
	if body.Users[0]["role"] != "user" {
		t.Fatalf("groupless user must list as role user, got %v", body.Users[0]["role"])
	}

	// membership in the admin group surfaces as role admin
	if err := store.SetGroups(id, []string{users.AdminGroup}); err != nil {
		t.Fatal(err)
	}
	w = doJSON(t, r, http.MethodGet, "/users", "")
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Users[0]["role"] != "admin" {
		t.Fatalf("admin-group member must list as role admin, got %v", body.Users[0]["role"])
	}
}

func TestCreateUser(t *testing.T) {
	t.Run("creates an enabled user by default and returns its id", func(t *testing.T) {
		store, r := newTestHandler(t)
		w := doJSON(t, r, http.MethodPost, "/users", `{"login":"bob","password":"secret"}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
		}
		var dto map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &dto)
		usr, err := store.GetUserByLogin("bob")
		if err != nil {
			t.Fatal(err)
		}
		if dto["id"] != usr.ID {
			t.Errorf("response id %v does not match stored uuid %q", dto["id"], usr.ID)
		}
		if !usr.Enabled {
			t.Error("user should be enabled by default")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(usr.HashPw), []byte("secret")); err != nil {
			t.Errorf("stored hash does not verify the password: %v", err)
		}
		if dto["role"] != "user" {
			t.Errorf("default role must be user, got %v", dto["role"])
		}
		if groups := mustGroups(t, store, usr.ID); len(groups) != 0 {
			t.Errorf("regular user must have no groups, got %v", groups)
		}
	})

	t.Run("respects enabled=false", func(t *testing.T) {
		store, r := newTestHandler(t)
		w := doJSON(t, r, http.MethodPost, "/users", `{"login":"bob","password":"secret","enabled":false}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", w.Code)
		}
		usr, err := store.GetUserByLogin("bob")
		if err != nil {
			t.Fatal(err)
		}
		if usr.Enabled {
			t.Error("user should be disabled")
		}
	})

	t.Run("rejects missing fields", func(t *testing.T) {
		_, r := newTestHandler(t)
		for name, body := range map[string]string{
			"empty login":       `{"login":"  ","password":"pw"}`,
			"empty password":    `{"login":"x","password":""}`,
			"invalid json":      `{`,
			"too long password": `{"login":"x","password":"` + strings.Repeat("a", 73) + `"}`,
		} {
			w := doJSON(t, r, http.MethodPost, "/users", body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", name, w.Code)
			}
		}
	})

	t.Run("duplicate login conflicts", func(t *testing.T) {
		_, r := newTestHandler(t)
		doJSON(t, r, http.MethodPost, "/users", `{"login":"bob","password":"pw"}`)
		w := doJSON(t, r, http.MethodPost, "/users", `{"login":"bob","password":"pw"}`)
		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
		}
	})
}

func TestCreateRejectsTokenShapedLogin(t *testing.T) {
	_, r := newTestHandler(t)
	for _, login := range []string{"abc123defg", "0123456789", "zzzzzzzzzz"} {
		w := doJSON(t, r, http.MethodPost, "/users", `{"login":"`+login+`","password":"longenough1"}`)
		if w.Code != http.StatusBadRequest {
			t.Errorf("login %q: status %d, want 400", login, w.Code)
		}
	}
	// Nearby shapes stay allowed: wrong length, uppercase, symbols.
	for _, login := range []string{"abc123defgh", "abc123def", "Abc123defg", "abc-123defg"} {
		w := doJSON(t, r, http.MethodPost, "/users", `{"login":"`+login+`","password":"longenough1"}`)
		if w.Code == http.StatusBadRequest {
			t.Errorf("login %q: unexpectedly rejected", login)
		}
	}
}

func TestCreateUserRole(t *testing.T) {
	t.Run("creates an admin via role", func(t *testing.T) {
		store, r := newTestHandler(t)
		w := doJSON(t, r, http.MethodPost, "/users", `{"login":"root","password":"secret","role":"admin"}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
		}
		var dto map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &dto)
		if dto["role"] != "admin" {
			t.Errorf("response role = %v, want admin", dto["role"])
		}
		usr, err := store.GetUserByLogin("root")
		if err != nil {
			t.Fatal(err)
		}
		groups := mustGroups(t, store, usr.ID)
		if len(groups) != 1 || groups[0] != users.AdminGroup {
			t.Errorf("admin must be a member of %q only, got %v", users.AdminGroup, groups)
		}
	})

	t.Run("rejects an unknown role", func(t *testing.T) {
		_, r := newTestHandler(t)
		w := doJSON(t, r, http.MethodPost, "/users", `{"login":"x","password":"pw","role":"superuser"}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
		}
	})
}

func TestUpdateUser(t *testing.T) {
	t.Run("changes password", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustCreate(t, store, "alice", "old")
		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"password":"new-pw"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
		}
		usr, err := store.GetUser(id)
		if err != nil {
			t.Fatal(err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(usr.HashPw), []byte("new-pw")); err != nil {
			t.Errorf("new password does not verify: %v", err)
		}
	})

	// Rename is refused, not silently ignored: per-user data (queue, stars,
	// playlists, history) is keyed on the LOGIN string, so a rename would
	// orphan it. See TestRenameIsRefused for the full contract.
	t.Run("a login change is refused", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustCreate(t, store, "alice", "pw")
		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"login":"alice2"}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
		}
		usr, err := store.GetUser(id)
		if err != nil {
			t.Fatal(err)
		}
		if usr.LoginID != "alice" {
			t.Errorf("login = %q, want it unchanged as alice", usr.LoginID)
		}
	})

	t.Run("toggles enabled without touching the password", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustCreate(t, store, "alice", "pw")
		before, _ := store.GetUser(id)
		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"enabled":false}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		usr, err := store.GetUser(id)
		if err != nil {
			t.Fatal(err)
		}
		if usr.Enabled {
			t.Error("user should be disabled")
		}
		if usr.HashPw != before.HashPw {
			t.Error("password hash must be unchanged")
		}
	})

	t.Run("missing user is 404", func(t *testing.T) {
		_, r := newTestHandler(t)
		w := doJSON(t, r, http.MethodPut, "/users/no-such-id", `{"enabled":true}`)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestUpdateUserRole(t *testing.T) {
	t.Run("promotes and demotes via role", func(t *testing.T) {
		store, r := newTestHandler(t)
		// A standing admin keeps the demotion below out of the last-admin guard
		// (TestLastAdminGuard covers that path); this case is about the group
		// bookkeeping of promote/demote.
		mustAdmin(t, store, "root")
		id := mustCreate(t, store, "alice", "pw")

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"role":"admin"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
		}
		var dto map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &dto)
		if dto["role"] != "admin" {
			t.Errorf("response role = %v, want admin", dto["role"])
		}
		groups := mustGroups(t, store, id)
		if len(groups) != 1 || groups[0] != users.AdminGroup {
			t.Fatalf("promotion must add the admin group, got %v", groups)
		}

		w = doJSON(t, r, http.MethodPut, "/users/"+id, `{"role":"user"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
		}
		if groups := mustGroups(t, store, id); len(groups) != 0 {
			t.Fatalf("demotion must remove the admin group, got %v", groups)
		}
	})

	t.Run("role change keeps unrelated group memberships", func(t *testing.T) {
		store, r := newTestHandler(t)
		mustAdmin(t, store, "root")
		id := mustCreate(t, store, "alice", "pw")
		if err := store.SetGroups(id, []string{"beta-testers"}); err != nil {
			t.Fatal(err)
		}

		doJSON(t, r, http.MethodPut, "/users/"+id, `{"role":"admin"}`)
		groups := mustGroups(t, store, id)
		if len(groups) != 2 || groups[0] != users.AdminGroup || groups[1] != "beta-testers" {
			t.Fatalf("promotion must keep other groups, got %v", groups)
		}

		doJSON(t, r, http.MethodPut, "/users/"+id, `{"role":"user"}`)
		groups = mustGroups(t, store, id)
		if len(groups) != 1 || groups[0] != "beta-testers" {
			t.Fatalf("demotion must only remove the admin group, got %v", groups)
		}
	})

	t.Run("rejects an unknown role", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustCreate(t, store, "alice", "pw")
		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"role":"superuser"}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
		}
	})
}

func TestDeleteUser(t *testing.T) {
	store, r := newTestHandler(t)
	// A second admin exists so deleting alice is not a last-admin removal.
	mustAdmin(t, store, "root")
	id := mustCreate(t, store, "alice", "pw")

	w := doJSON(t, r, http.MethodDelete, "/users/"+id, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if _, err := store.GetUser(id); err == nil {
		t.Error("user should be gone")
	}

	w = doJSON(t, r, http.MethodDelete, "/users/"+id, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on second delete, got %d", w.Code)
	}
}

// TestRenameIsRefused pins the reason rename is closed off: owner-keyed data
// (play queue, stars, playlists, history) is keyed on the login STRING, not the
// user UUID, so renaming would leave that data behind under the old key and
// silently hide it from its owner. Until the owner columns key on the UUID, a
// login change is rejected rather than half-applied.
func TestRenameIsRefused(t *testing.T) {
	t.Run("a changed login is 400 and mutates nothing", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustCreate(t, store, "alice", "pw")

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"login":"alice2","enabled":false}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
		}
		usr, err := store.GetUser(id)
		if err != nil {
			t.Fatal(err)
		}
		if usr.LoginID != "alice" {
			t.Errorf("login = %q, want alice", usr.LoginID)
		}
		// The rename is rejected before any other field is applied, so the
		// request is all-or-nothing rather than partially committed.
		if !usr.Enabled {
			t.Error("enabled must not be applied when the request is rejected")
		}
	})

	// Sending the login unchanged is how the edit dialog submits a form that
	// simply displays it, so it must not trip the guard.
	t.Run("resubmitting the same login is accepted", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustCreate(t, store, "alice", "pw")

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"login":"alice","enabled":false}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
		}
		usr, err := store.GetUser(id)
		if err != nil {
			t.Fatal(err)
		}
		if usr.Enabled {
			t.Error("enabled=false should have been applied")
		}
	})

	t.Run("whitespace-only differences do not count as a rename", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustCreate(t, store, "alice", "pw")

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"login":"  alice  "}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
		}
	})
}

// TestLastAdminGuard covers the lockout hole: the users CRUD is the only path
// that can grant the admin role, and bootstrapAdmin only seeds while the store
// is EMPTY, so removing the final admin is unrecoverable without hand-editing
// the database. Demote, disable and delete are each refused for the last one.
func TestLastAdminGuard(t *testing.T) {
	t.Run("the last admin cannot demote itself", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustAdmin(t, store, "root")

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"role":"user"}`)
		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
		}
		if role := mustRole(t, store, id); role != users.RoleAdmin {
			t.Errorf("role = %q, want it still admin", role)
		}
	})

	t.Run("the last admin cannot be disabled", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustAdmin(t, store, "root")

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"enabled":false}`)
		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
		}
		usr, err := store.GetUser(id)
		if err != nil {
			t.Fatal(err)
		}
		if !usr.Enabled {
			t.Error("the last admin must stay enabled")
		}
	})

	t.Run("the last admin cannot be deleted", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustAdmin(t, store, "root")

		w := doJSON(t, r, http.MethodDelete, "/users/"+id, "")
		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d, body=%s", w.Code, w.Body.String())
		}
		if _, err := store.GetUser(id); err != nil {
			t.Errorf("the last admin must survive: %v", err)
		}
	})

	// A disabled admin cannot log in, so it cannot administer either: it must
	// not count towards the quorum that keeps the guard satisfied.
	t.Run("a disabled admin does not count as a remaining admin", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustAdmin(t, store, "root")
		other := mustAdmin(t, store, "ghost")
		if err := store.SetEnabled(other, false); err != nil {
			t.Fatal(err)
		}

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"role":"user"}`)
		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409 with only a disabled peer admin, got %d, body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("demote, disable and delete succeed with a second enabled admin", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustAdmin(t, store, "root")
		mustAdmin(t, store, "root2")

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"role":"user"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("demote: expected 200, got %d, body=%s", w.Code, w.Body.String())
		}

		id2 := mustAdmin(t, store, "root3")
		w = doJSON(t, r, http.MethodPut, "/users/"+id2, `{"enabled":false}`)
		if w.Code != http.StatusOK {
			t.Fatalf("disable: expected 200, got %d, body=%s", w.Code, w.Body.String())
		}

		id3 := mustAdmin(t, store, "root4")
		w = doJSON(t, r, http.MethodDelete, "/users/"+id3, "")
		if w.Code != http.StatusNoContent {
			t.Fatalf("delete: expected 204, got %d, body=%s", w.Code, w.Body.String())
		}
	})

	// Only the admin population is protected; a regular user is freely removed
	// even when it is the only account besides the admin.
	t.Run("a regular user is unaffected by the guard", func(t *testing.T) {
		store, r := newTestHandler(t)
		mustAdmin(t, store, "root")
		id := mustCreate(t, store, "alice", "pw")

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"enabled":false}`)
		if w.Code != http.StatusOK {
			t.Fatalf("disable regular user: expected 200, got %d, body=%s", w.Code, w.Body.String())
		}
		w = doJSON(t, r, http.MethodDelete, "/users/"+id, "")
		if w.Code != http.StatusNoContent {
			t.Fatalf("delete regular user: expected 204, got %d, body=%s", w.Code, w.Body.String())
		}
	})

	// Re-disabling an already-disabled admin is a no-op, not a lockout: the
	// guard must key on whether the change removes the last ENABLED admin.
	t.Run("a no-op enabled write on a disabled admin is allowed", func(t *testing.T) {
		store, r := newTestHandler(t)
		id := mustAdmin(t, store, "root")
		if err := store.SetEnabled(id, false); err != nil {
			t.Fatal(err)
		}

		w := doJSON(t, r, http.MethodPut, "/users/"+id, `{"enabled":false}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for a no-op write, got %d, body=%s", w.Code, w.Body.String())
		}
	})
}
