package router

import (
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/andresbott/aether/app/router/handlers/subsonic"
	"github.com/andresbott/aether/app/spa"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/go-bumbu/http/middleware"
	"github.com/gorilla/mux"
)

type Cfg struct {
	Logger        *slog.Logger
	TaskRunner    *taskrunner.Runner
	TaskLogGetter taskrunner.TaskLogGetter
	Store         *store.Store
	DataDir       string
	TagReader     tags.Reader
}

type MainAppHandler struct {
	router        *mux.Router
	logger        *slog.Logger
	taskRunner    *taskrunner.Runner
	taskLogGetter taskrunner.TaskLogGetter
	store         *store.Store
	dataDir       string
	tagReader     tags.Reader
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
		store:         cfg.Store,
		dataDir:       cfg.DataDir,
		tagReader:     cfg.TagReader,
	}

	hist, _ := middleware.NewPromHistogram("", nil, nil)
	prodMid := middleware.New(middleware.Cfg{
		JsonErrors:  true,
		GenericErrs: false,
		Logger:      cfg.Logger,
		PromHisto:   hist,
	})
	r.Use(prodMid.Middleware)

	app.attachApiV1(app.router.PathPrefix("/api/v1").Subrouter())

	if app.store != nil {
		assets := assetstore.New(filepath.Join(app.dataDir, "metadata"))
		subsonic.Register(app.router, app.store, assets, filepath.Join(app.dataDir, "generated-covers"), filepath.Join(app.dataDir, "radio-covers"))
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
