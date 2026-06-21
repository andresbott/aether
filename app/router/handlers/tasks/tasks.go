package tasks

import (
	"encoding/json"
	"errors"
	"net/http"

	apptasks "github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Handler struct {
	Runner        *taskrunner.Runner
	TaskLogGetter taskrunner.TaskLogGetter
}

func (h *Handler) ListTasks() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]apptasks.TaskDef{"tasks": apptasks.AvailableTasks})
	})
}

func (h *Handler) TriggerTask() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := mux.Vars(r)["name"]
		if !apptasks.TaskNameExists(name) {
			http.Error(w, "unknown task: "+name, http.StatusNotFound)
			return
		}
		id, err := h.Runner.AddRun(name)
		if err != nil {
			if errors.Is(err, taskrunner.ErrQueueFull) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "Task queue is full. Try again later."})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"execution_id": id.String()})
	})
}

func (h *Handler) ListExecutions() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executions := h.Runner.Executions()
		name := mux.Vars(r)["name"]
		if name != "" {
			var filtered []taskrunner.ExecutionInfo
			for _, e := range executions {
				if e.TaskName == name {
					filtered = append(filtered, e)
				}
			}
			executions = filtered
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]taskrunner.ExecutionInfo{"executions": executions})
	})
}

func (h *Handler) CancelExecution() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := mux.Vars(r)["id"]
		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "invalid execution id", http.StatusBadRequest)
			return
		}
		if err := h.Runner.Cancel(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func (h *Handler) GetExecutionLog() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.TaskLogGetter == nil {
			http.Error(w, "task logs not available", http.StatusServiceUnavailable)
			return
		}
		idStr := mux.Vars(r)["id"]
		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "invalid execution id", http.StatusBadRequest)
			return
		}
		text, err := h.TaskLogGetter.GetTaskLog(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(text)) //nolint:gosec // G705: response is text/plain, not HTML; log content cannot execute as script
	})
}
