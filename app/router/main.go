package router

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	metadataHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/app/router/handlers/subsonic"
	"github.com/andresbott/aether/app/spa"
	"github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/identify"
	"github.com/andresbott/aether/internal/imagecache"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/go-bumbu/http/middleware"
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

// patIdentityResolver authenticates /rest via the OpenSubsonic apiKey
// parameter against the PAT service — the only authentication on /rest
// (docs/agents/authentication.md). nil when auth is "none".
func (h *MainAppHandler) patIdentityResolver() subsonic.IdentityResolver {
	if h.tokens == nil {
		return nil
	}
	return func(r *http.Request) (string, int) {
		q := r.URL.Query()
		key := q.Get("apiKey")
		if key == "" {
			// Includes u/t/s-only clients: salted-token auth needs
			// recoverable storage (TODO.md) and answers 40 until then.
			return "", 40
		}
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
	// Mask apiKey values in request logs (go-bumbu middleware logs RequestURI).
	// Mutate only RequestURI — handlers parse r.URL, which must stay intact.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("apiKey") {
				r.RequestURI = strings.ReplaceAll(r.RequestURI, "apiKey="+r.URL.Query().Get("apiKey"), "apiKey=***")
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Use(prodMid.Middleware)
	r.Use(jsonErrorEnvelope)

	app.attachApiV1(app.router.PathPrefix("/api/v1").Subrouter())

	if app.store != nil {
		identity := app.patIdentityResolver()
		subsonic.Register(app.router, app.store, app.assets, app.images, identity)
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
