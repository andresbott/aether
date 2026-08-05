// Package users exposes CRUD for native users on /api/v1. It is a server
// management concern (like libraries), never a music-client feature: /rest
// stays untouched. The handler is only registered when the auth method is
// "native" — with "none" there is no user store at all.
package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-bumbu/userauth"
	"github.com/go-bumbu/userauth/userstore/userdb"
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

type Handler struct {
	Users *userdb.Store
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
// Login renames the user (the UUID identity is unaffected).
type updateInput struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Enabled  *bool  `json:"enabled"`
	Role     string `json:"role"` // "admin" or "user"; empty leaves the role untouched
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Error: msg, Code: code})
}

func (h *Handler) Routes(r *mux.Router) {
	r.Path("/users").Methods(http.MethodGet).HandlerFunc(h.list)
	r.Path("/users").Methods(http.MethodPost).HandlerFunc(h.create)
	r.Path("/users/{id}").Methods(http.MethodPut).HandlerFunc(h.update)
	r.Path("/users/{id}").Methods(http.MethodDelete).HandlerFunc(h.delete)
}

func (h *Handler) list(w http.ResponseWriter, _ *http.Request) {
	// The store caps pages at 200; a self-hosted music server does not have
	// more users than that, so the UI gets everything in one response.
	res, err := h.Users.List(userdb.ListOpts{Limit: 200})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]userDTO, 0, len(res.Users))
	for _, u := range res.Users {
		role, err := h.roleOf(u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out = append(out, userDTO{ID: u.ID, Login: u.LoginID, Enabled: u.Enabled, Role: role})
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out, "total": res.Total})
}

// RoleOf derives the vertical from the stored group memberships: membership
// in AdminGroup means admin, anything else (including no groups) means user.
// Exported because the /api/v1 admin guard and /me apply the same policy.
func RoleOf(store *userdb.Store, userID string) (string, error) {
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

func validPassword(pw string) error {
	if pw == "" {
		return errors.New("password is required")
	}
	if len(pw) > maxPasswordLen {
		return errors.New("password must be at most 72 characters")
	}
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in createInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	in.Login = strings.TrimSpace(in.Login)
	if in.Login == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "login is required")
		return
	}
	if err := validPassword(in.Password); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if in.Role == "" {
		in.Role = RoleUser
	}
	if err := validRole(in.Role); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	enabled := in.Enabled == nil || *in.Enabled
	usr := userdb.User{
		LoginID: in.Login,
		Pw:      in.Password,
		Enabled: enabled,
		Groups:  roleGroups(in.Role),
	}
	if err := h.Users.CreateUser(usr); err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	// CreateUser does not return the generated UUID; read the row back so the
	// client gets the id it must use for updates and deletes.
	created, err := h.Users.GetUserByLogin(in.Login)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, userDTO{ID: created.ID, Login: created.LoginID, Enabled: created.Enabled, Role: in.Role})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	existing, err := h.Users.GetUser(id)
	if err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}

	var in updateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
		return
	}
	login := existing.LoginID
	if newLogin := strings.TrimSpace(in.Login); newLogin != "" && newLogin != existing.LoginID {
		if err := h.Users.SetLoginID(id, newLogin); err != nil {
			status, code := mapStoreError(err)
			writeError(w, status, code, err.Error())
			return
		}
		login = newLogin
	}
	if in.Password != "" {
		if err := validPassword(in.Password); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), BcryptDifficulty)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if err := h.Users.SetPasswordHash(id, string(hash)); err != nil {
			status, code := mapStoreError(err)
			writeError(w, status, code, err.Error())
			return
		}
	}
	enabled := existing.Enabled
	if in.Enabled != nil && *in.Enabled != existing.Enabled {
		if err := h.Users.SetEnabled(id, *in.Enabled); err != nil {
			status, code := mapStoreError(err)
			writeError(w, status, code, err.Error())
			return
		}
		enabled = *in.Enabled
	}
	role, err := h.roleOf(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if in.Role != "" && in.Role != role {
		if err := validRole(in.Role); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		if err := h.setRole(id, in.Role); err != nil {
			status, code := mapStoreError(err)
			writeError(w, status, code, err.Error())
			return
		}
		role = in.Role
	}
	writeJSON(w, http.StatusOK, userDTO{ID: id, Login: login, Enabled: enabled, Role: role})
}

// setRole rewrites the user's group memberships to encode the role. Only
// AdminGroup is touched: any other (future) memberships survive a promotion
// or demotion.
func (h *Handler) setRole(userID, role string) error {
	groups, err := h.Users.GetGroups(userID)
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
	return h.Users.SetGroups(userID, next)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.Users.Delete(id); err != nil {
		status, code := mapStoreError(err)
		writeError(w, status, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapStoreError(err error) (status int, code string) {
	if errors.Is(err, userauth.ErrUserNotFound) {
		return http.StatusNotFound, "not_found"
	}
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "duplicate") {
			return http.StatusConflict, "conflict"
		}
	}
	return http.StatusInternalServerError, "internal"
}
