package router

import (
	"log/slog"
	"net/http"
	"path/filepath"

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
	// AuthMethod is the configured authentication method ("none"/"native"),
	// reported to the SPA via GET /api/v1/auth.
	AuthMethod string
	// Users is the native user store; nil unless AuthMethod is "native".
	// When set, the users CRUD is mounted on /api/v1.
	Users *userdb.Store
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
}

func (h *MainAppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func New(cfg Cfg) (*MainAppHandler, error) {
	r := mux.NewRouter()
	app := MainAppHandler{
		router:        r,
		logger:        cfg.Logger,
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
	}
	if app.authMethod == "" {
		app.authMethod = "none"
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
	r.Use(prodMid.Middleware)
	r.Use(jsonErrorEnvelope)

	app.attachApiV1(app.router.PathPrefix("/api/v1").Subrouter())

	if app.store != nil {
		subsonic.Register(app.router, app.store, app.assets, app.images)
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
