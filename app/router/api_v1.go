package router

import (
	"net/http"

	"github.com/andresbott/aether/app/router/handlers"
	libraryHandler "github.com/andresbott/aether/app/router/handlers/libraries"
	metadataHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	taskHandler "github.com/andresbott/aether/app/router/handlers/tasks"
	"github.com/gorilla/mux"
)

func (h *MainAppHandler) attachApiV1(r *mux.Router) {
	r.Path("/health").Methods(http.MethodGet).Handler(handlers.HealthHandler())

	if h.taskRunner != nil {
		th := taskHandler.Handler{
			Runner:        h.taskRunner,
			TaskLogGetter: h.taskLogGetter,
		}
		r.Path("/tasks").Methods(http.MethodGet).Handler(th.ListTasks())
		r.Path("/tasks/{name}").Methods(http.MethodPost).Handler(th.TriggerTask())
		r.Path("/tasks/{name}/executions").Methods(http.MethodGet).Handler(th.ListExecutions())
		r.Path("/tasks/{name}/executions/{id}").Methods(http.MethodDelete).Handler(th.CancelExecution())
		r.Path("/tasks/{name}/executions/{id}/log").Methods(http.MethodGet).Handler(th.GetExecutionLog())
	}

	if h.store != nil {
		lh := &libraryHandler.Handler{Store: h.store}
		lh.Routes(r)

		if h.tagReader != nil {
			mh := &metadataHandler.Handler{Store: h.store, Reader: h.tagReader}
			mh.Routes(r)
		}
	}

	r.PathPrefix("").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "wrong api call", http.StatusBadRequest)
	})
}
