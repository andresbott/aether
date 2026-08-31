// Package users exposes CRUD for native users on /api/v1. It is a server
// management concern (like libraries), never a music-client feature: /rest
// stays untouched. The handler is only registered when the auth method is
// "native" — with "none" there is no user store at all.
package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	"github.com/andresbott/aether/internal/store"
	"github.com/go-bumbu/userauth"
	"github.com/go-bumbu/userauth/service/user"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

// BcryptDifficulty is the bcrypt cost for every password aether hashes
// (admin bootstrap, CLI, this handler). 12 is the current OWASP-recommended
// minimum for bcrypt.
const BcryptDifficulty = 12

// maxPasswordLen guards against bcrypt's 72-byte input truncation: two
// passwords sharing the first 72 bytes verify as equal, so longer inputs are
// rejected instead of silently truncated.
const maxPasswordLen = 72

// AdminGroup is the group membership that makes a user an admin. Group names
// are opaque to the userauth library — this constant is aether's only policy
// on top of them: membership means admin, absence means regular user.
const AdminGroup = "admin"

// The two user verticals exposed by the API. There is no third value: a user
// is an admin (member of AdminGroup) or a regular user (no membership).
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// tokenShapedLogin matches the virtual-username namespace of usertoken PATs
// (10 chars of lowercase base36, see the pat library's token IDs). Such a
// login would collide with token authentication on /rest, where u is
// resolved as a tokenID first.
var tokenShapedLogin = regexp.MustCompile(`^[0-9a-z]{10}$`)

type Handler struct {
	Users *user.Service
}

// userDTO exposes both halves of the upstream identity split: id is the
// stable UUID every mutation keys on, login the mutable login name. Role is
// the derived vertical ("admin"/"user"), not the raw group list.
type userDTO struct {
	ID      string `json:"id"`
	Login   string `json:"login"`
	Enabled bool   `json:"enabled"`
	Role    string `json:"role"`
}

type createInput struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Enabled  *bool  `json:"enabled"`
	Role     string `json:"role"` // "admin" or "user"; empty defaults to "user"
}

// updateInput carries partial updates: nil/empty fields are left untouched.
// Login is accepted only when it equals the current login (the edit dialog
// submits the field it displays) — an actual rename is refused, see
// errRenameUnsupported.
type updateInput struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Enabled  *bool  `json:"enabled"`
	Role     string `json:"role"` // "admin" or "user"; empty leaves the role untouched
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/users").Methods(http.MethodGet).HandlerFunc(h.list)
	r.Path("/users").Methods(http.MethodPost).HandlerFunc(h.create)
	r.Path("/users/{id}").Methods(http.MethodPut).HandlerFunc(h.update)
	r.Path("/users/{id}").Methods(http.MethodDelete).HandlerFunc(h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	// The store caps pages at 200; a self-hosted music server does not have
	// more users than that, so the UI gets everything in one response.
	res, err := h.Users.List(user.ListOpts{Limit: 200})
	if err != nil {
		httperr.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]userDTO, 0, len(res.Users))
	for _, u := range res.Users {
		role, err := h.roleOf(u.ID)
		if err != nil {
			httperr.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out = append(out, userDTO{ID: u.ID, Login: u.LoginID, Enabled: u.Enabled, Role: role})
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out, "total": res.Total})
}

// RoleOf derives the vertical from the stored group memberships: membership
// in AdminGroup means admin, anything else (including no groups) means user.
// Exported because the /api/v1 admin guard and /me apply the same policy.
func RoleOf(store *user.Service, userID string) (string, error) {
	groups, err := store.GetGroups(userID)
	if err != nil {
		return "", err
	}
	for _, g := range groups {
		if g == AdminGroup {
			return RoleAdmin, nil
		}
	}
	return RoleUser, nil
}

func (h *Handler) roleOf(userID string) (string, error) {
	return RoleOf(h.Users, userID)
}

// errRenameUnsupported explains why a login change is refused rather than
// applied. Per-user data (play queue, stars, playlists, history) is keyed on the
// login STRING, not on the stable UUID, so a rename leaves every owner-keyed row
// behind under the old key — the data survives in the DB but becomes invisible
// to the only user entitled to it. Refusing is the honest behaviour until the
// owner columns key on User.ID.
var errRenameUnsupported = errors.New(
	"renaming a user is not supported: per-user data is keyed on the login name")

// errLastAdmin is the lockout guard. The users CRUD is the ONLY path that grants
// the admin role, and bootstrapAdmin re-seeds only while the store is empty, so
// removing the final usable admin cannot be undone without editing the database
// by hand. Demote, disable and delete all check it.
var errLastAdmin = errors.New(
	"refusing to remove the last admin: promote another user to admin first")

// isLastEnabledAdmin reports whether userID is the only admin that can still
// administer. Disabled admins do not count: they cannot log in, so leaving one
// behind would be the same lockout with extra steps.
func (h *Handler) isLastEnabledAdmin(userID string) (bool, error) {
	role, err := h.roleOf(userID)
	if err != nil {
		return false, err
	}
	if role != RoleAdmin {
		return false, nil
	}
	// The list is capped at 200 like list(); a self-hosted server does not have
	// more users, and the alternative (a group-joined COUNT) is not exposed by
	// the user store.
	res, err := h.Users.List(user.ListOpts{Limit: 200})
	if err != nil {
		return false, err
	}
	for _, u := range res.Users {
		if u.ID == userID || !u.Enabled {
			continue
		}
		peerRole, err := h.roleOf(u.ID)
		if err != nil {
			return false, err
		}
		if peerRole == RoleAdmin {
			return false, nil
		}
	}
	return true, nil
}

// guardLastAdmin writes the 409 and reports whether the caller must stop. An
// infrastructure failure is reported as 500 and also stops the caller: it must
// never be read as "the change is safe".
func (h *Handler) guardLastAdmin(w http.ResponseWriter, r *http.Request, userID string) (blocked bool) {
	last, err := h.isLastEnabledAdmin(userID)
	if err != nil {
		httperr.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return true
	}
	if last {
		httperr.Write(w, r, http.StatusConflict, "last_admin", errLastAdmin.Error())
		return true
	}
	return false
}

// roleGroups maps a role to the group memberships that encode it.
func roleGroups(role string) []string {
	if role == RoleAdmin {
		return []string{AdminGroup}
	}
	return nil
}

func validRole(role string) error {
	if role != RoleAdmin && role != RoleUser {
		return errors.New(`role must be "admin" or "user"`)
	}
	return nil
}

// errPasswordTooLong flags a password that is present but would silently
// truncate at bcrypt's 72-byte input limit — a well-formed-but-invalid value,
// unlike an outright missing password.
var errPasswordTooLong = errors.New("password must be at most 72 characters")

func validPassword(pw string) error {
	if pw == "" {
		return errors.New("password is required")
	}
	if len(pw) > maxPasswordLen {
		return errPasswordTooLong
	}
	return nil
}

// writePasswordErr answers a validPassword failure with the right status: an
// empty password is a missing required field (400); an over-length one is
// well-formed but invalid (422).
func writePasswordErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errPasswordTooLong) {
		httperr.WriteValidation(w, r, err.Error(), httperr.FieldError{Pointer: "/password", Detail: err.Error()})
		return
	}
	httperr.Write(w, r, http.StatusBadRequest, "validation_error", err.Error())
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in createInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httperr.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	in.Login = strings.TrimSpace(in.Login)
	if in.Login == "" {
		httperr.Write(w, r, http.StatusBadRequest, "validation_error", "login is required")
		return
	}
	if tokenShapedLogin.MatchString(in.Login) {
		msg := "login must not look like a token id (10 lowercase letters/digits)"
		httperr.WriteValidation(w, r, msg, httperr.FieldError{Pointer: "/login", Detail: msg})
		return
	}
	if err := validPassword(in.Password); err != nil {
		writePasswordErr(w, r, err)
		return
	}
	if in.Role == "" {
		in.Role = RoleUser
	}
	if err := validRole(in.Role); err != nil {
		httperr.WriteValidation(w, r, err.Error(), httperr.FieldError{Pointer: "/role", Detail: err.Error()})
		return
	}
	enabled := in.Enabled == nil || *in.Enabled
	// The identity service stores only a hash; aether hashes at BcryptDifficulty
	// here, the same cost as the CLI and the password service.
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), BcryptDifficulty)
	if err != nil {
		httperr.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	created, err := h.Users.CreateUser(user.Draft{
		LoginID:      in.Login,
		PasswordHash: string(hash),
		Enabled:      &enabled,
		Groups:       roleGroups(in.Role),
	})
	if err != nil {
		status, code := mapStoreError(err)
		httperr.Write(w, r, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, userDTO{ID: created.ID, Login: created.LoginID, Enabled: created.Enabled, Role: in.Role})
}

// validateUpdate rejects an update before any of it is applied, writing the
// error response itself. It reports whether the caller must stop.
func (h *Handler) validateUpdate(w http.ResponseWriter, r *http.Request, id string, existing userauth.User, in updateInput) (blocked bool) {
	if newLogin := strings.TrimSpace(in.Login); newLogin != "" && newLogin != existing.LoginID {
		httperr.Write(w, r, http.StatusBadRequest, "validation_error", errRenameUnsupported.Error())
		return true
	}
	if in.Password != "" {
		if err := validPassword(in.Password); err != nil {
			writePasswordErr(w, r, err)
			return true
		}
	}
	if in.Role != "" {
		if err := validRole(in.Role); err != nil {
			httperr.WriteValidation(w, r, err.Error(), httperr.FieldError{Pointer: "/role", Detail: err.Error()})
			return true
		}
	}
	// Losing the admin role and losing the ability to log in are the same
	// lockout, so both are guarded. A no-op write (already disabled, or the role
	// unchanged) removes nothing and is allowed through.
	demoting := in.Role == RoleUser
	disabling := in.Enabled != nil && !*in.Enabled && existing.Enabled
	if demoting || disabling {
		return h.guardLastAdmin(w, r, id)
	}
	return false
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	existing, err := h.Users.GetUser(id)
	if err != nil {
		status, code := mapStoreError(err)
		httperr.Write(w, r, status, code, err.Error())
		return
	}

	var in updateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httperr.Write(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	// Everything is validated before the store is touched: the mutations below
	// are separate store calls, not one transaction, so a late rejection would
	// otherwise leave the update half-applied.
	if blocked := h.validateUpdate(w, r, id, existing, in); blocked {
		return
	}

	login := existing.LoginID
	if in.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), BcryptDifficulty)
		if err != nil {
			httperr.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if err := h.Users.SetPasswordHash(id, string(hash)); err != nil {
			status, code := mapStoreError(err)
			httperr.Write(w, r, status, code, err.Error())
			return
		}
	}
	enabled := existing.Enabled
	if in.Enabled != nil && *in.Enabled != existing.Enabled {
		if err := h.Users.SetEnabled(id, *in.Enabled); err != nil {
			status, code := mapStoreError(err)
			httperr.Write(w, r, status, code, err.Error())
			return
		}
		enabled = *in.Enabled
	}
	role, err := h.roleOf(id)
	if err != nil {
		httperr.Write(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if in.Role != "" && in.Role != role {
		if err := h.setRole(id, in.Role); err != nil {
			status, code := mapStoreError(err)
			httperr.Write(w, r, status, code, err.Error())
			return
		}
		role = in.Role
	}
	writeJSON(w, http.StatusOK, userDTO{ID: id, Login: login, Enabled: enabled, Role: role})
}

// SetRole rewrites the user's group memberships to encode the role. Only
// AdminGroup is touched: any other (future) memberships survive a promotion
// or demotion. Exported because proxy mode mirrors the header-derived role
// into the DB so surfaces the proxy bypasses (/rest) can consult it.
func SetRole(store *user.Service, userID, role string) error {
	groups, err := store.GetGroups(userID)
	if err != nil {
		return err
	}
	next := make([]string, 0, len(groups)+1)
	for _, g := range groups {
		if g != AdminGroup {
			next = append(next, g)
		}
	}
	if role == RoleAdmin {
		next = append(next, AdminGroup)
	}
	return store.SetGroups(userID, next)
}

func (h *Handler) setRole(userID, role string) error {
	return SetRole(h.Users, userID, role)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	// Existence is checked first so a missing user still answers 404 rather than
	// the lockout 409 (roleOf reports a groupless "user" for an unknown id).
	if _, err := h.Users.GetUser(id); err != nil {
		status, code := mapStoreError(err)
		httperr.Write(w, r, status, code, err.Error())
		return
	}
	if h.guardLastAdmin(w, r, id) {
		return
	}
	if err := h.Users.Delete(id); err != nil {
		status, code := mapStoreError(err)
		httperr.Write(w, r, status, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapStoreError(err error) (status int, code string) {
	if errors.Is(err, userauth.ErrUserNotFound) {
		return http.StatusNotFound, "not_found"
	}
	if errors.Is(err, user.ErrLoginIDTaken) || store.IsUniqueViolation(err) {
		return http.StatusConflict, "conflict"
	}
	return http.StatusInternalServerError, "internal"
}
