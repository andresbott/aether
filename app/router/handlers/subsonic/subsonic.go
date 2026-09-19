package subsonic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/pathguard"
	"github.com/andresbott/aether/internal/store"
	"github.com/gorilla/mux"
)

// IdentityResolver resolves the authenticated user for a /rest request.
// Success iff owner != ""; otherwise code is the Subsonic error code the
// middleware must answer with: 40 wrong or missing credentials, 41 token auth
// not supported for this user, 43 conflicting auth mechanisms, 44 invalid API
// key, 0 internal error. The resolver owns auth policy
// (docs/agents/authentication.md); handlers only ever see the owner.
type IdentityResolver func(r *http.Request) (owner string, code int)

// AdminChecker reports whether owner holds the admin role. The router
// injects it alongside the IdentityResolver (same seam: auth policy lives
// outside the handlers); nil means no role system exists (auth "none") and
// every caller passes. Spec-restricted endpoints (the radio CRUD writes)
// consult it via requireAdmin.
type AdminChecker func(owner string) (bool, error)

// ownerCtxKey carries the resolved owner on the request context.
type ownerCtxKey struct{}

// defaultOwner is the fixed owner used when no IdentityResolver is
// configured (auth method "none"): the single-user behavior /rest always had.
const defaultOwner = "admin"

// requestOwner returns the owner the identity middleware resolved, or
// defaultOwner when none ran (auth "none", or a handler under test).
func requestOwner(r *http.Request) string {
	if v, ok := r.Context().Value(ownerCtxKey{}).(string); ok && v != "" {
		return v
	}
	return defaultOwner
}

// requireAdmin answers Subsonic error 50 (not authorized) unless the session
// owner holds the admin role, reporting whether the handler may proceed. With
// no AdminChecker installed (auth "none") every caller passes: that mode has
// a single fixed owner who is the admin.
func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if h.admin == nil {
		return true
	}
	ok, err := h.admin(requestOwner(r))
	if err != nil {
		writeError(w, 0, "internal error")
		return false
	}
	if !ok {
		writeError(w, 50, "admin privileges required")
		return false
	}
	return true
}

// authErrorMessage keeps the per-code wording uniform: no distinction
// between unknown and expired keys (no probing oracle).
func authErrorMessage(code int) string {
	switch code {
	case 40:
		return "authentication required"
	case 41:
		return "token authentication not supported for this user; create a user token in aether's settings"
	case 43:
		return "multiple conflicting authentication mechanisms provided"
	case 44:
		return "invalid API key"
	default:
		return "authentication error"
	}
}

type Handler struct {
	store  *store.Store
	assets *assetstore.Store
	images *imagecache.Cache
	// admin reports whether an owner holds the admin role; nil means no role
	// system (auth "none") and requireAdmin passes everyone.
	admin AdminChecker
	// mediaGuard confines the files the media handlers will read to the
	// configured scan-folder roots. Paths reach those handlers from the DB, not
	// from the request, so this enforces that a track/cover row actually points
	// into a scan folder. nil disables the check (no roots configured).
	mediaGuard *pathguard.Guard
}

// Option customizes the /rest handler at registration time.
type Option func(*Handler)

// WithAdminChecker installs the role lookup behind requireAdmin, gating the
// spec's admin-only endpoints (radio CRUD writes). Not installing it leaves
// them open to every authenticated user — correct only for auth "none".
func WithAdminChecker(admin AdminChecker) Option {
	return func(h *Handler) {
		h.admin = admin
	}
}

// WithMediaRoots confines stream/getCoverArt to files under a fixed set of
// roots — in production the configured scan folders, which cannot change while
// the server runs, so a snapshot is exact. Called with no usable roots it
// installs no guard, so a server with no scan folders yet keeps serving its own
// generated covers.
func WithMediaRoots(roots ...string) Option {
	return func(h *Handler) {
		if g := newGuard(roots); g != nil {
			h.mediaGuard = g
		}
	}
}

// newGuard builds a guard over the usable (non-empty) roots, or nil when there
// are none — "no libraries configured" must not become "deny everything", which
// would black out every cover on a fresh install.
func newGuard(roots []string) *pathguard.Guard {
	usable := make([]string, 0, len(roots))
	for _, r := range roots {
		if r != "" {
			usable = append(usable, r)
		}
	}
	if len(usable) == 0 {
		return nil
	}
	return pathguard.New(usable...)
}

func Register(r *mux.Router, s *store.Store, assets *assetstore.Store, images *imagecache.Cache, identity IdentityResolver, opts ...Option) {
	h := &Handler{store: s, assets: assets, images: images}
	for _, opt := range opts {
		opt(h)
	}
	sub := r.PathPrefix("/rest").Subrouter()

	sub.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("f") == "xml" {
				writeError(w, 0, "XML format is not supported, use JSON")
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	sub.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// getOpenSubsonicExtensions must be publicly accessible per spec:
			// clients discover apiKey support through this endpoint, so it
			// cannot be gated behind the apiKey. ping stays gated (returning
			// 40 with no credentials is the standard auth-probe mechanism).
			if strings.HasSuffix(r.URL.Path, "/getOpenSubsonicExtensions") ||
				strings.HasSuffix(r.URL.Path, "/getOpenSubsonicExtensions.view") {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ownerCtxKey{}, defaultOwner)))
				return
			}
			owner := defaultOwner
			if identity != nil {
				var code int
				owner, code = identity(r)
				if owner == "" {
					writeError(w, code, authErrorMessage(code))
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ownerCtxKey{}, owner)))
		})
	})

	register := func(name string, fn http.HandlerFunc) {
		sub.HandleFunc("/"+name, fn)
		sub.HandleFunc("/"+name+".view", fn)
	}

	// System
	register("ping", h.ping)
	register("getLicense", h.getLicense)
	register("getOpenSubsonicExtensions", h.getOpenSubsonicExtensions)

	// User (getUser for the caller's own record only; getUsers is intentionally
	// not mounted — user administration lives on /api/v0, see TODO.md)
	register("getUser", h.getUser)

	// Browsing
	register("getMusicFolders", h.getMusicFolders)
	register("getIndexes", h.getIndexes)
	register("getArtists", h.getArtists)
	register("getArtist", h.getArtist)
	register("getAlbum", h.getAlbum)
	register("getSong", h.getSong)
	register("getGenres", h.getGenres)

	// Artists (cover-art extension; no standard updateArtist endpoint exists)
	register("updateArtist", h.updateArtist)

	// Genres (genreCoverArt extension; no standard updateGenre endpoint exists)
	register("updateGenre", h.updateGenre)

	// Albums (albumCoverArt extension; no standard updateAlbum endpoint exists)
	register("updateAlbum", h.updateAlbum)

	// Lists
	register("getAlbumList2", h.getAlbumList2)
	register("getAlbumList2Index", h.getAlbumList2Index)
	register("getRandomSongs", h.getRandomSongs)
	register("getSongsByGenre", h.getSongsByGenre)
	register("getStarred2", h.getStarred2)
	register("getNowPlaying", h.getNowPlaying)

	// Discovery ("discovery" extension; no standard ranked-feed endpoint exists)
	register("getDiscovery", h.getDiscovery)

	// Search
	register("search3", h.search3)

	// Media
	register("stream", h.stream)
	register("getCoverArt", h.getCoverArt)
	register("getGeneratedCoverPreview", h.getGeneratedCoverPreview)
	register("getGeneratedCoverCandidates", h.getGeneratedCoverCandidates)

	// Playlists
	register("getPlaylists", h.getPlaylists)
	register("getPlaylist", h.getPlaylist)
	register("createPlaylist", h.createPlaylist)
	register("updatePlaylist", h.updatePlaylist)
	register("deletePlaylist", h.deletePlaylist)

	// Play queue (savePlayQueue/getPlayQueue are spec; the ByIndex pair is the
	// "indexBasedQueue" extension — both read and write the SAME stored queue)
	register("savePlayQueue", h.savePlayQueue)
	register("getPlayQueue", h.getPlayQueue)
	register("savePlayQueueByIndex", h.savePlayQueueByIndex)
	register("getPlayQueueByIndex", h.getPlayQueueByIndex)

	// Annotation
	register("star", h.star)
	register("unstar", h.unstar)
	register("scrobble", h.scrobble)
	register("setRating", h.setRating)

	// Internet Radio
	register("getInternetRadioStations", h.getInternetRadioStations)
	register("createInternetRadioStation", h.createInternetRadioStation)
	register("updateInternetRadioStation", h.updateInternetRadioStation)
	register("deleteInternetRadioStation", h.deleteInternetRadioStation)
}

func encodeArtistID(id uint) string   { return fmt.Sprintf("ar-%d", id) }
func encodeAlbumID(id uint) string    { return fmt.Sprintf("al-%d", id) }
func encodeTrackID(id uint) string    { return fmt.Sprintf("tr-%d", id) }
func encodePlaylistID(id uint) string { return fmt.Sprintf("pl-%d", id) }
func encodeRadioID(id uint) string    { return fmt.Sprintf("rs-%d", id) }
func encodeGenreID(id uint) string    { return fmt.Sprintf("ge-%d", id) }

func decodeID(s string) (string, uint, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid id: %s", s)
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid id: %s", s)
	}
	var itemType string
	switch parts[0] {
	case "ar":
		itemType = "artist"
	case "al":
		itemType = "album"
	case "tr":
		itemType = "track"
	case "pl":
		itemType = "playlist"
	case "rs":
		itemType = "radio"
	case "ge":
		itemType = "genre"
	default:
		return "", 0, fmt.Errorf("unknown id prefix: %s", parts[0])
	}
	return itemType, uint(id), nil
}

func paramStr(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

func paramInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func paramStrSlice(r *http.Request, key string) []string {
	return r.URL.Query()[key]
}

// paramBoolPtr parses an optional boolean query parameter. It returns nil when
// the key is absent, true for "true"/"1", and false otherwise — letting callers
// distinguish "not provided" from an explicit value.
func paramBoolPtr(r *http.Request, key string) *bool {
	if !r.URL.Query().Has(key) {
		return nil
	}
	v := r.URL.Query().Get(key)
	b := v == "true" || v == "1"
	return &b
}

// libraryScope resolves the optional musicFolderId parameter to the scope of
// the library it names, plus that library. Absent or unparseable answers the
// zero scope and a nil library — cross-library, since the spec makes the
// parameter optional. An id that names no library answers a scope matching
// nothing, so the request keeps returning empty lists. A store failure is a
// different thing: it is answered here as an internal error with ok=false (like
// requireAdmin), because "no tracks" would hand the client a successful empty
// list to cache.
func (h *Handler) libraryScope(w http.ResponseWriter, r *http.Request) (scope store.TrackScope, lib *model.Library, ok bool) {
	s := r.URL.Query().Get("musicFolderId")
	if s == "" {
		return store.TrackScope{}, nil, true
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return store.TrackScope{}, nil, true
	}
	found, err := h.store.GetLibrary(uint(n))
	if errors.Is(err, store.ErrNotFound) {
		return store.NoTracks(), nil, true
	}
	if err != nil {
		writeError(w, 0, "internal error")
		return store.TrackScope{}, nil, false
	}
	return store.LibraryScope(&found), &found, true
}
