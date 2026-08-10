package router

import (
	"crypto/md5" //nolint:gosec // Subsonic salted-token auth is MD5 by spec
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	metadataHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/app/router/handlers/subsonic"
	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/andresbott/aether/app/spa"
	"github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/identify"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/go-bumbu/http/middleware"
	"github.com/go-bumbu/userauth"
	"github.com/go-bumbu/userauth/auth/cookieauth"
	"github.com/go-bumbu/userauth/auth/headerauth"
	"github.com/go-bumbu/userauth/service/pat"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"github.com/gorilla/mux"
)

// imageCacheDir holds the display-sized, re-encoded copies of entity images
// (see internal/imagecache). It is a pure cache: deleting it costs nothing but
// the work to rebuild, which is why it lives outside metadata/ — that tree holds
// the only copy of manually uploaded art and must never be cleared.
const imageCacheDir = "image-cache"

type Cfg struct {
	Logger        *slog.Logger
	TaskRunner    *taskrunner.Runner
	TaskLogGetter taskrunner.TaskLogGetter
	ScheduleStore *taskrunner.ScheduleStore
	Scheduler     *taskrunner.Scheduler
	Store         *store.Store
	DataDir       string
	TagReader     tags.Reader
	ArtistFetcher tasks.Fetcher
	// Identifier is optional: nil disables audio identification in the
	// metadata editor.
	Identifier *identify.Identifier
	// IdentifyUnavailableReason is the user-facing explanation shown by the
	// editor when Identifier is nil (e.g. fpcalc missing). Ignored otherwise.
	IdentifyUnavailableReason string
	// Rescanner re-indexes files the metadata editor writes, so an edit shows
	// up in the music UI without a scan task. Optional: nil disables it.
	Rescanner metadataHandler.TrackRescanner
	// AuthMethod is the configured authentication method
	// ("none"/"native"/"proxy-header"), reported to the SPA via GET /api/v1/me.
	AuthMethod string
	// Users is the user store; nil unless AuthMethod is "native" or
	// "proxy-header". The users CRUD is mounted only in native mode; proxy
	// mode provisions users on first sight and manages them at the proxy's IdP.
	Users *userdb.Store
	// Sessions is the cookie session manager; nil unless AuthMethod is
	// "native". When set, the login/logout endpoints are mounted and every
	// /api/v1 route except the public bootstrap set requires a session.
	Sessions *cookieauth.Manager
	// Tokens is the personal-access-token service; nil unless AuthMethod is
	// "native" or "proxy-header". When set, the session-scoped token endpoints
	// are mounted on /api/v1 and /rest authenticates via OpenSubsonic apiKey.
	Tokens *pat.Service
	// HeaderAuth validates proxy-injected identity headers; nil unless
	// AuthMethod is "proxy-header". When set, every /api/v1 route except the
	// public bootstrap set requires a trusted header identity.
	HeaderAuth *headerauth.HeaderHandler
	// AdminGroup is the proxy-asserted group that grants the admin role;
	// only meaningful with HeaderAuth.
	AdminGroup string
}

type MainAppHandler struct {
	router        *mux.Router
	logger        *slog.Logger
	taskRunner    *taskrunner.Runner
	taskLogGetter taskrunner.TaskLogGetter
	scheduleStore *taskrunner.ScheduleStore
	scheduler     *taskrunner.Scheduler
	store         *store.Store
	dataDir       string
	tagReader     tags.Reader
	artistFetcher tasks.Fetcher
	assets        *assetstore.Store
	images        *imagecache.Cache
	identifier    *identify.Identifier
	identifyOff   string
	rescanner     metadataHandler.TrackRescanner
	authMethod    string
	users         *userdb.Store
	sessions      *cookieauth.Manager
	tokens        *pat.Service
	headerAuth    *headerauth.HeaderHandler
	adminGroup    string
}

func (h *MainAppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

// restAdminChecker reports whether a /rest owner (a login, per requestOwner)
// holds the admin role, for the spec's admin-only endpoints (radio CRUD
// writes). Role comes from the DB groups in both modes: native writes them
// directly, proxy mode mirrors the header-derived role into them on every
// /api/v1 request (resolveProxyIdentity) because /rest is proxy-bypassed and
// carries no identity headers. nil when auth is "none" — single fixed owner,
// no roles to check.
func (h *MainAppHandler) restAdminChecker() subsonic.AdminChecker {
	if h.users == nil {
		return nil
	}
	return func(owner string) (bool, error) {
		usr, err := h.users.GetUserByLogin(owner)
		if err != nil {
			if errors.Is(err, userauth.ErrUserNotFound) {
				return false, nil // fail closed, not an infrastructure error
			}
			return false, err
		}
		role, err := usersHandler.RoleOf(h.users, usr.ID)
		if err != nil {
			return false, err
		}
		return role == usersHandler.RoleAdmin, nil
	}
}

// patIdentityResolver authenticates /rest against the PAT service — the only
// authentication on /rest (docs/agents/authentication.md). Two mechanisms:
//   - apiKey=<full token>: hash-only verify, any PAT type.
//   - u=<virtual username> with t+s (salted MD5) or p (plaintext/enc:hex):
//     the OpenSubsonic password flows, where u is a usertoken PAT's tokenID —
//     never a real login. Needs recoverable storage (VerifyMatch).
//
// nil when auth is "none".
func (h *MainAppHandler) patIdentityResolver() subsonic.IdentityResolver {
	if h.tokens == nil {
		return nil
	}
	return func(r *http.Request) (string, int) {
		q := r.URL.Query()
		key := q.Get("apiKey")
		if key != "" {
			// Fail-closed per spec: apiKey mixed with password params is 43.
			if q.Has("u") || q.Has("p") || q.Has("t") || q.Has("s") {
				return "", 43
			}
			info, ok, err := h.tokens.Verify(key)
			if err != nil {
				h.logger.Error("rest auth: token verify failed", "err", err)
				return "", 0 // infrastructure failure, not bad credentials
			}
			if !ok {
				return "", 44
			}
			return info.LoginID, 0
		}
		user := q.Get("u")
		if user == "" {
			return "", 40
		}
		match, ok := subsonicCredentialMatcher(q)
		if !ok {
			return "", 40
		}
		info, ok, err := h.tokens.VerifyMatch(user, match)
		if err != nil {
			if errors.Is(err, pat.ErrNotRecoverable) {
				// The id exists but is not a usertoken (apikey PAT, or a
				// real login shape): token auth unsupported for this user.
				return "", 41
			}
			h.logger.Error("rest auth: token match failed", "err", err)
			return "", 0
		}
		if !ok {
			// Unknown virtual username or wrong credentials. If u is a real
			// login, answer 41: the login password never works on /rest and
			// clients should show "configure a token", not "wrong password".
			if _, uerr := h.users.GetUserByLogin(user); uerr == nil {
				return "", 41
			}
			return "", 40
		}
		return info.LoginID, 0
	}
}

// subsonicCredentialMatcher builds the secret-testing callback for the
// request's password mechanism: t+s (salted MD5 token, preferred) or p
// (plaintext or enc:<hex>). ok=false when neither mechanism is present.
func subsonicCredentialMatcher(q url.Values) (func(secret string) bool, bool) {
	if tok, salt := q.Get("t"), q.Get("s"); tok != "" && salt != "" {
		return func(secret string) bool {
			sum := md5.Sum([]byte(secret + salt)) //nolint:gosec // Subsonic salted-token auth is MD5 by spec
			digest := hex.EncodeToString(sum[:])
			return subtle.ConstantTimeCompare([]byte(digest), []byte(strings.ToLower(tok))) == 1
		}, true
	}
	if p := q.Get("p"); p != "" {
		if hexPw, found := strings.CutPrefix(p, "enc:"); found {
			raw, err := hex.DecodeString(hexPw)
			if err != nil {
				return func(string) bool { return false }, true
			}
			p = string(raw)
		}
		pw := p
		return func(secret string) bool {
			return subtle.ConstantTimeCompare([]byte(secret), []byte(pw)) == 1
		}, true
	}
	return nil, false
}

func New(cfg Cfg) (*MainAppHandler, error) {
	r := mux.NewRouter()
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	app := MainAppHandler{
		router:        r,
		logger:        logger,
		taskRunner:    cfg.TaskRunner,
		taskLogGetter: cfg.TaskLogGetter,
		scheduleStore: cfg.ScheduleStore,
		scheduler:     cfg.Scheduler,
		store:         cfg.Store,
		dataDir:       cfg.DataDir,
		tagReader:     cfg.TagReader,
		artistFetcher: cfg.ArtistFetcher,
		assets:        assetstore.New(filepath.Join(cfg.DataDir, "metadata")),
		images:        imagecache.New(filepath.Join(cfg.DataDir, imageCacheDir)),
		identifier:    cfg.Identifier,
		identifyOff:   cfg.IdentifyUnavailableReason,
		rescanner:     cfg.Rescanner,
		authMethod:    cfg.AuthMethod,
		users:         cfg.Users,
		sessions:      cfg.Sessions,
		tokens:        cfg.Tokens,
		headerAuth:    cfg.HeaderAuth,
		adminGroup:    cfg.AdminGroup,
	}
	if app.authMethod == "" {
		app.authMethod = "none"
	}
	if app.authMethod == "native" && app.tokens == nil {
		return nil, fmt.Errorf("auth method native requires Tokens")
	}
	if app.authMethod == "proxy-header" &&
		(app.headerAuth == nil || app.tokens == nil || app.users == nil || app.adminGroup == "") {
		return nil, fmt.Errorf("auth method proxy-header requires HeaderAuth, Tokens, Users and AdminGroup")
	}

	hist, _ := middleware.NewPromHistogram("", nil, nil)
	// JsonErrors stays off: it wraps *every* error body, which escapes the JSON
	// our handlers already write into a string field and shows the user a raw
	// document. jsonErrorEnvelope does the same job JSON-aware — see errors.go.
	// The middleware keeps logging + metrics.
	prodMid := middleware.New(middleware.Cfg{
		JsonErrors:  false,
		GenericErrs: false,
		Logger:      cfg.Logger,
		PromHisto:   hist,
	})
	// Mask credential values in request logs (go-bumbu middleware logs
	// RequestURI). Mutate only RequestURI — handlers parse r.URL, which must
	// stay intact. u stays visible: it is an identifier, not a secret.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			for _, param := range []string{"apiKey", "t", "s", "p"} {
				if v := q.Get(param); v != "" {
					r.RequestURI = strings.ReplaceAll(r.RequestURI, param+"="+v, param+"=***")
				}
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Use(prodMid.Middleware)
	r.Use(jsonErrorEnvelope)

	app.attachApiV1(app.router.PathPrefix("/api/v1").Subrouter())

	if app.store != nil {
		identity := app.patIdentityResolver()
		// The media handlers serve files named by DB rows (track file_path, album
		// cover_path), so they are confined to the configured library roots — read
		// on demand, since libraries are added while the server runs.
		subsonic.Register(app.router, app.store, app.assets, app.images, identity,
			subsonic.WithLibraryRoots(app.store.LibraryRoots),
			subsonic.WithAdminChecker(app.restAdminChecker()))
	}

	if err := app.attachSpa(app.router.PathPrefix("/").Subrouter(), "/"); err != nil {
		return nil, err
	}

	return &app, nil
}

func (h *MainAppHandler) attachSpa(r *mux.Router, path string) error {
	spaHandler, err := spa.App(path)
	if err != nil {
		return err
	}
	r.Methods(http.MethodGet).PathPrefix(path).Handler(spaHandler)
	return nil
}
