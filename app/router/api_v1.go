package router

import (
	"fmt"
	"net/http"

	"github.com/andresbott/aether/app/metainfo"
	"github.com/andresbott/aether/app/router/handlers"
	artistsHandler "github.com/andresbott/aether/app/router/handlers/artists"
	libraryHandler "github.com/andresbott/aether/app/router/handlers/libraries"
	metadataHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	radiobrowserHandler "github.com/andresbott/aether/app/router/handlers/radiobrowser"
	taskHandler "github.com/andresbott/aether/app/router/handlers/tasks"
	"github.com/andresbott/aether/internal/albumidentify"
	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/coverart"
	"github.com/andresbott/aether/internal/radiobrowser"
	"github.com/gorilla/mux"
)

func (h *MainAppHandler) attachApiV1(r *mux.Router) {
	r.Path("/health").Methods(http.MethodGet).Handler(handlers.HealthHandler())
	r.Path("/version").Methods(http.MethodGet).Handler(handlers.VersionHandler())

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
