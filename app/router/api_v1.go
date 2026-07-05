package router

import (
	"fmt"
	"net/http"

	"github.com/andresbott/aether/app/metainfo"
	"github.com/andresbott/aether/app/router/handlers"
	artistsHandler "github.com/andresbott/aether/app/router/handlers/artists"
	libraryHandler "github.com/andresbott/aether/app/router/handlers/libraries"
	metadataHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	taskHandler "github.com/andresbott/aether/app/router/handlers/tasks"
	"github.com/andresbott/aether/internal/artistimage"
	"github.com/gorilla/mux"
)

func (h *MainAppHandler) attachApiV1(r *mux.Router) {
	r.Path("/health").Methods(http.MethodGet).Handler(handlers.HealthHandler())

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
			mh := &metadataHandler.Handler{Store: h.store, Reader: h.tagReader}
			mh.Routes(r)
		}

		userAgent := fmt.Sprintf("Aether/%s (https://github.com/andresbott/aether)", metainfo.Version)
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
