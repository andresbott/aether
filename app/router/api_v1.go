package router

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/andresbott/aether/app/metainfo"
	"github.com/andresbott/aether/app/router/handlers"
	artistsHandler "github.com/andresbott/aether/app/router/handlers/artists"
	authHandler "github.com/andresbott/aether/app/router/handlers/auth"
	"github.com/andresbott/aether/app/router/handlers/httperr"
	libraryHandler "github.com/andresbott/aether/app/router/handlers/libraries"
	metadataHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	radiobrowserHandler "github.com/andresbott/aether/app/router/handlers/radiobrowser"
	taskHandler "github.com/andresbott/aether/app/router/handlers/tasks"
	tokensHandler "github.com/andresbott/aether/app/router/handlers/tokens"
	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/andresbott/aether/internal/albumidentify"
	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/coverart"
	"github.com/andresbott/aether/internal/dlcache"
	"github.com/andresbott/aether/internal/radiobrowser"
	"github.com/go-bumbu/userauth/auth/cookieauth"
	"github.com/gorilla/mux"
)

// apiV1PublicPaths are the /api/v1 routes reachable without a session in
// native mode: the SPA bootstraps on /me before any login, /health and
// /version carry nothing sensitive, and login/logout are the way in and out
// of a session — neither can require one.
var apiV1PublicPaths = map[string]bool{
	"/api/v1/health":      true,
	"/api/v1/version":     true,
	"/api/v1/me":          true,
	"/api/v1/auth/login":  true,
	"/api/v1/auth/logout": true,
}

// apiV1SessionPath reports whether the route needs a valid session but NOT
// the admin role — the session-scoped tier between the public bootstrap set
// and the admin default. Everything here operates strictly on the caller's
// own data (tokens). A func, not a map like apiV1PublicPaths, because the
// token CRUD has a {tokenId} path segment.
func apiV1SessionPath(path string) bool {
	return path == "/api/v1/auth/token" ||
		path == "/api/v1/auth/tokens" ||
		strings.HasPrefix(path, "/api/v1/auth/tokens/")
}

// sessionGuard enforces three tiers on /api/v1 in native mode: (1) public
// bootstrap (health/version/me/login/logout), (2) session-scoped endpoints
// where a valid session suffices (personal token mint + CRUD), (3) everything
// else defaults to admin-only (users CRUD, libraries, tasks, metadata). The
// public tier is checked first, the session-scoped tier second; if neither
// matches the path defaults to admin-only. With auth method "none" the guard
// is not installed and /api/v1 stays open.
func (h *MainAppHandler) sessionGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiV1PublicPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		// HandleAuth validates the cookie, puts the identity on the request
		// context and renews the rolling expiry ("remember me" sessions).
		ok, _ := h.sessions.HandleAuth(w, r)
		if !ok {
			httperr.Write(w, r, http.StatusUnauthorized, "unauthorized", httperr.TitleFor("unauthorized"), "authentication required")
			return
		}
		data, err := cookieauth.CtxGetUserData(r)
		if err != nil {
			httperr.Write(w, r, http.StatusUnauthorized, "unauthorized", httperr.TitleFor("unauthorized"), "authentication required")
			return
		}
		// The DB Enabled flag is aether's kill-switch and it must close sessions
		// that are ALREADY open, not just future logins: otherwise a disabled
		// admin keeps its session and can re-enable itself through this very API.
		// Checked before the session-scoped tier so a disabled user cannot mint
		// a fresh /rest token either. Mirrors headerGuard in proxy_auth.go.
		usr, err := h.users.GetUser(data.UserId)
		if err != nil {
			// A session pointing at a deleted user authenticates nothing.
			httperr.Write(w, r, http.StatusUnauthorized, "unauthorized", httperr.TitleFor("unauthorized"), "authentication required")
			return
		}
		if !usr.Enabled {
			httperr.Write(w, r, http.StatusForbidden, "forbidden", httperr.TitleFor("forbidden"), "user is disabled")
			return
		}
		// Session-scoped tier: authenticated, any role. Non-admin ≠ public —
		// only the role check is skipped, never the session check above.
		if apiV1SessionPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		role, err := usersHandler.RoleOf(h.users, data.UserId)
		if err != nil {
			httperr.Write(w, r, http.StatusInternalServerError, "internal", httperr.TitleFor("internal"), "internal error")
			return
		}
		if role != usersHandler.RoleAdmin {
			httperr.Write(w, r, http.StatusForbidden, "forbidden", httperr.TitleFor("forbidden"), "admin privileges required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sessionCaller resolves the tokens handler's caller from the session cookie
// the sessionGuard validated (native mode's side of the resolver seam).
func sessionCaller(r *http.Request) (string, bool) {
	data, err := cookieauth.CtxGetUserData(r)
	if err != nil || !data.IsAuthenticated {
		return "", false
	}
	return data.UserId, true
}

// meIdentity resolves the /me caller from the session cookie, nil when there
// is none. It renews the rolling expiry: the SPA polls /me on boot, which is
// exactly the "activity" a remember-me session should stay alive on.
func (h *MainAppHandler) meIdentity(w http.ResponseWriter, r *http.Request) *handlers.MeUser {
	data, err := h.sessions.GetSessData(r)
	if err != nil || !data.IsAuthenticated {
		return nil
	}
	usr, err := h.users.GetUser(data.UserId)
	if err != nil {
		// Session refers to a deleted user: treat as unauthenticated.
		return nil
	}
	// A disabled user reads as anonymous rather than 403: /me is public-tier, and
	// reporting no identity is what makes the SPA fall back to the login view.
	if !usr.Enabled {
		return nil
	}
	role, err := usersHandler.RoleOf(h.users, usr.ID)
	if err != nil {
		return nil
	}
	_ = h.sessions.TouchSession(r, w)
	return &handlers.MeUser{Login: usr.LoginID, Role: role}
}

func (h *MainAppHandler) attachApiV1(r *mux.Router) {
	// Identity for /me and the tokens handler comes from whichever guard the
	// mode installs: the session cookie (native) or the proxy headers.
	var identity handlers.MeIdentity
	var caller func(*http.Request) (string, bool)
	switch {
	case h.sessions != nil:
		identity = h.meIdentity
		caller = sessionCaller
		r.Use(h.sessionGuard)
	case h.headerAuth != nil:
		identity = h.proxyMeIdentity
		caller = proxyCaller
		r.Use(h.headerGuard)
	}

	// The users CRUD is a native-mode feature: in proxy mode users are
	// managed at the proxy's identity provider and provisioned on first
	// sight, so the CRUD stays unmounted and /me reports it absent.
	userManagement := h.users != nil && h.authMethod == "native"

	r.Path("/health").Methods(http.MethodGet).Handler(handlers.HealthHandler())
	r.Path("/version").Methods(http.MethodGet).Handler(handlers.VersionHandler())
	r.Path("/me").Methods(http.MethodGet).Handler(handlers.MeHandler(h.authMethod, userManagement, identity))

	// Native auth only: login/logout and the users CRUD.
	if h.users != nil && h.sessions != nil {
		ah := &authHandler.Handler{Users: h.users, Sessions: h.sessions, Tokens: h.tokens, Logger: h.logger}
		ah.Routes(r)
	}
	if userManagement {
		uh := &usersHandler.Handler{Users: h.users}
		uh.Routes(r)
	}
	if h.tokens != nil {
		th := &tokensHandler.Handler{Tokens: h.tokens, Caller: caller}
		th.Routes(r)
	}

	userAgent := fmt.Sprintf("Aether/%s (https://github.com/andresbott/aether)", metainfo.Version)

	// Radio-browser proxy endpoints (station search + favicon fetch) are an
	// admin import tool with no store dependency, so register them up front.
	rbh := &radiobrowserHandler.Handler{Client: radiobrowser.New(userAgent)}
	rbh.Routes(r)

	if h.taskRunner != nil {
		th := taskHandler.Handler{
			Runner:        h.taskRunner,
			TaskLogGetter: h.taskLogGetter,
			ScheduleStore: h.scheduleStore,
			Scheduler:     h.scheduler,
			Logger:        h.logger,
		}
		// Executions are global. Register these before /tasks/{name} so the
		// {name} var does not capture the literal "executions".
		r.Path("/tasks/executions").Methods(http.MethodGet).Handler(th.ListExecutions())
		r.Path("/tasks/executions/{id}/cancel").Methods(http.MethodPost).Handler(th.CancelExecution())
		r.Path("/tasks/executions/{id}/logs").Methods(http.MethodGet).Handler(th.GetExecutionLog())

		r.Path("/tasks").Methods(http.MethodGet).Handler(th.ListTasks())
		r.Path("/tasks/{name}/trigger").Methods(http.MethodPost).Handler(th.TriggerTask())
		r.Path("/tasks/{name}").Methods(http.MethodGet).Handler(th.GetTask())
		r.Path("/tasks/{name}").Methods(http.MethodPut).Handler(th.UpsertTask())
		r.Path("/tasks/{name}").Methods(http.MethodPatch).Handler(th.PatchTask())
		r.Path("/tasks/{name}").Methods(http.MethodDelete).Handler(th.DeleteTaskSchedule())
	}

	if h.store != nil {
		lh := &libraryHandler.Handler{Store: h.store}
		lh.Routes(r)

		if h.tagReader != nil {
			mh := &metadataHandler.Handler{
				Store:                     h.store,
				Reader:                    h.tagReader,
				CoverArt:                  coverart.New(userAgent),
				Images:                    h.images,
				IdentifyUnavailableReason: h.identifyOff,
				Rescan:                    h.rescanner,
				// Memoize provider image bytes so a repeated pre-save probe and the
				// save reuse one download of the rate-limited image. Bounded and
				// short-lived: an edit session touches a handful of images.
				Downloads: dlcache.New(10*time.Minute, 64<<20),
				// The editor's artist-folder image reuses the artist image provider
				// chain to download online picks. h.artistFetcher is an interface
				// (nil when unconfigured), so assigning it here keeps the field nil
				// rather than wrapping a nil pointer — upload still works, online
				// picks answer 503.
				ArtistImages: h.artistFetcher,
			}
			if h.identifier != nil {
				// Guard both assignments: a nil *identify.Identifier assigned
				// to an interface-typed field produces a non-nil interface
				// wrapping a nil pointer, breaking the nil checks in identify.go
				// and identify_album.go.
				mh.Identifier = h.identifier
				// Album identification needs the same fingerprinting the
				// per-file identify uses, plus MusicBrainz for tracklists.
				// The tracklist lookup is cached IN FRONT of the throttle:
				// MusicBrainz allows one request per second and a run enriches
				// up to MaxEnrichedOptions releases, so without this a repeat
				// identify of the same album still waits several seconds even
				// though the fingerprint pass is already cached.
				mh.AlbumIdentifier = albumidentify.New(
					h.identifier,
					albumidentify.NewCachingReleaseLookup(
						artistimage.NewMusicBrainzSearch(userAgent),
						albumidentify.DefaultReleaseCacheSize,
					),
				)
			}
			mh.Routes(r)
		}

		ah := &artistsHandler.Handler{
			Store:   h.store,
			Assets:  h.assets,
			Fetcher: h.artistFetcher,
			Search:  artistimage.NewMusicBrainzSearch(userAgent),
		}
		ah.Routes(r)
	}

	r.PathPrefix("").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "wrong api call", http.StatusBadRequest)
	})
}
