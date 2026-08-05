package subsonic

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/store"
	"github.com/gorilla/mux"
)

// IdentityResolver resolves the authenticated user for a /rest request.
// ok=false means the request carries no valid identity. This is the single
// seam the future PAT/token layer replaces (docs/agents/authentication.md):
// today production wires a session-cookie resolver, later a Subsonic
// token verifier — handlers only ever see the owner string.
type IdentityResolver func(r *http.Request) (owner string, ok bool)

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

type Handler struct {
	store  *store.Store
	assets *assetstore.Store
	images *imagecache.Cache
}

func Register(r *mux.Router, s *store.Store, assets *assetstore.Store, images *imagecache.Cache, identity IdentityResolver) {
	h := &Handler{store: s, assets: assets, images: images}
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
			owner := defaultOwner
			if identity != nil {
				// CSRF hardening (interim, until PAT layer): reject cross-site
				// requests. Every /rest endpoint is GET-reachable and the session
				// cookie is SameSite=Lax, so a top-level cross-site navigation can
				// trigger destructive writes (deletePlaylist, star, savePlayQueue...).
				// Requests without Sec-Fetch-Site (curl, older clients, future PAT
				// clients) pass — this is defense-in-depth, not a gate.
				if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
					writeError(w, 50, "cross-site request rejected")
					return
				}
				var ok bool
				owner, ok = identity(r)
				if !ok || owner == "" {
					// Subsonic error 40: the protocol's "bad credentials"
					// code — there is no separate "no credentials" code.
					writeError(w, 40, "authentication required")
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

// paramLibraryID parses the optional musicFolderId query parameter.
// Returns nil when absent or unparseable — treated as "cross-library"
// per the Subsonic spec (param is optional).
func paramLibraryID(r *http.Request) *uint {
	s := r.URL.Query().Get("musicFolderId")
	if s == "" {
		return nil
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil
	}
	u := uint(n)
	return &u
}
