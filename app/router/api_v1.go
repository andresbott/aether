package router

import (
	"fmt"
	"net/http"

	"github.com/andresbott/aether/app/metainfo"
	"github.com/andresbott/aether/app/router/handlers"
	artistsHandler "github.com/andresbott/aether/app/router/handlers/artists"
	authHandler "github.com/andresbott/aether/app/router/handlers/auth"
	libraryHandler "github.com/andresbott/aether/app/router/handlers/libraries"
	metadataHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	radiobrowserHandler "github.com/andresbott/aether/app/router/handlers/radiobrowser"
	taskHandler "github.com/andresbott/aether/app/router/handlers/tasks"
	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/andresbott/aether/internal/albumidentify"
	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/coverart"
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

// sessionGuard requires a valid session cookie on every /api/v1 route except
// the public bootstrap set, and an admin role on top of it: everything /api/v1
// mounts beyond that set is server administration (users, libraries, tasks,
// metadata, the musicbrainz/radiobrowser proxies), so a non-admin session gets
// 403. When a session-only route appears (the planned token-mint endpoint),
// give it an allowlist like apiV1PublicPaths rather than weakening this
// default. Only installed in native mode; with auth method "none" /api/v1
// stays open.
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
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		data, err := cookieauth.CtxGetUserData(r)
		if err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		role, err := usersHandler.RoleOf(h.users, data.UserId)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if role != usersHandler.RoleAdmin {
			http.Error(w, "admin privileges required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
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
	role, err := usersHandler.RoleOf(h.users, usr.ID)
	if err != nil {
		return nil
	}
	_ = h.sessions.TouchSession(r, w)
	return &handlers.MeUser{Login: usr.LoginID, Role: role}
}

func (h *MainAppHandler) attachApiV1(r *mux.Router) {
	var identity handlers.MeIdentity
	if h.sessions != nil {
		identity = h.meIdentity
		r.Use(h.sessionGuard)
	}

	r.Path("/health").Methods(http.MethodGet).Handler(handlers.HealthHandler())
	r.Path("/version").Methods(http.MethodGet).Handler(handlers.VersionHandler())
	r.Path("/me").Methods(http.MethodGet).Handler(handlers.MeHandler(h.authMethod, h.users != nil, identity))

	// Native auth only: login/logout and the users CRUD. With method "none"
	// h.users and h.sessions are nil, the routes are never mounted and
	// /api/v1/me reports the feature as absent.
	if h.users != nil && h.sessions != nil {
		ah := &authHandler.Handler{Users: h.users, Sessions: h.sessions, Logger: h.logger}
		ah.Routes(r)
	}
	if h.users != nil {
		uh := &usersHandler.Handler{Users: h.users}
		uh.Routes(r)
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
				Assets:                    h.assets,
				CoverArt:                  coverart.New(userAgent),
				Images:                    h.images,
				IdentifyUnavailableReason: h.identifyOff,
				Rescan:                    h.rescanner,
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
