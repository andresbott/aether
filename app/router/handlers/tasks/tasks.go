package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	apptasks "github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Handler struct {
	Runner        *taskrunner.Runner
	TaskLogGetter taskrunner.TaskLogGetter
	ScheduleStore *taskrunner.ScheduleStore
	Scheduler     *taskrunner.Scheduler
	Logger        *slog.Logger
}

// TaskWithSchedule is a task definition combined with its schedule (if any).
type TaskWithSchedule struct {
	apptasks.TaskDef
	Schedule *taskrunner.Schedule `json:"schedule,omitempty"`
}

func taskDefByName(name string) (apptasks.TaskDef, bool) {
	for _, t := range apptasks.AvailableTasks {
		if t.ID == name {
			return t, true
		}
	}
	return apptasks.TaskDef{}, false
}

// writeErr builds and writes an application/problem+json error — the same
// shape as metadata's writeErr shim, and equivalent to the writeError shim
// each other migrated handler package (libraries, artists, radiobrowser,
// tokens, users) defines locally.
func writeErr(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	httperr.Write(w, r, status, code, httperr.TitleFor(code), msg)
}

func (h *Handler) ListTasks() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out := make([]TaskWithSchedule, 0, len(apptasks.AvailableTasks))
		var scheduleMap map[string]taskrunner.Schedule
		if h.ScheduleStore != nil {
			list, err := h.ScheduleStore.List(r.Context())
			if err != nil {
				h.Logger.Error("list tasks: schedule list failed", "err", err)
				writeErr(w, r, http.StatusInternalServerError, "internal", "Failed to list tasks.")
				return
			}
			scheduleMap = make(map[string]taskrunner.Schedule, len(list))
			for _, s := range list {
				scheduleMap[s.TaskName] = s
			}
		}
		for _, t := range apptasks.AvailableTasks {
			ent := TaskWithSchedule{TaskDef: t}
			if s, ok := scheduleMap[t.ID]; ok {
				ent.Schedule = &s
			}
			out = append(out, ent)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]TaskWithSchedule{"tasks": out})
	})
}

func (h *Handler) GetTask() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := mux.Vars(r)["name"]
		def, ok := taskDefByName(name)
		if !ok {
			writeErr(w, r, http.StatusNotFound, "not_found", "unknown task: "+name)
			return
		}
		out := TaskWithSchedule{TaskDef: def}
		if h.ScheduleStore != nil {
			if sch, err := h.ScheduleStore.GetByTaskName(r.Context(), name); err == nil {
				out.Schedule = &sch
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
}

func (h *Handler) TriggerTask() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := mux.Vars(r)["name"]
		if !apptasks.TaskNameExists(name) {
			writeErr(w, r, http.StatusNotFound, "not_found", "unknown task: "+name)
			return
		}
		id, err := h.Runner.AddRun(name)
		if err != nil {
			if errors.Is(err, taskrunner.ErrQueueFull) {
				httperr.Write(w, r, http.StatusTooManyRequests, "queue_full", httperr.TitleFor("queue_full"), "Task queue is full. Try again later.")
				return
			}
			h.Logger.Error("trigger task: add run failed", "task", name, "err", err)
			writeErr(w, r, http.StatusInternalServerError, "internal", "Failed to trigger the task.")
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]taskrunner.ExecutionInfo{"executions": executions})
	})
}

func (h *Handler) CancelExecution() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(mux.Vars(r)["id"])
		if err != nil {
			writeErr(w, r, http.StatusBadRequest, "validation_error", "invalid execution id")
			return
		}
		if err := h.Runner.Cancel(r.Context(), id); err != nil {
			// Cancel folds "unknown execution id" and any other cancellation
			// failure into the same conflict — a documented quirk this refactor
			// preserves as-is (see cancelTaskExecution's description in
			// docs/openapi/aether-v1.yaml).
			h.Logger.Error("cancel execution failed", "id", id.String(), "err", err)
			writeErr(w, r, http.StatusConflict, "conflict", "Failed to cancel the execution.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func (h *Handler) GetExecutionLog() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.TaskLogGetter == nil {
			writeErr(w, r, http.StatusServiceUnavailable, "unavailable", "task logs not available")
			return
		}
		id, err := uuid.Parse(mux.Vars(r)["id"])
		if err != nil {
			writeErr(w, r, http.StatusBadRequest, "validation_error", "invalid execution id")
			return
		}
		text, err := h.TaskLogGetter.GetTaskLog(r.Context(), id)
		if err != nil {
			h.Logger.Error("get execution log failed", "id", id.String(), "err", err)
			writeErr(w, r, http.StatusInternalServerError, "internal", "Failed to retrieve the task log.")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write([]byte(text)) //nolint:gosec // text/plain with nosniff; cannot execute as script
	})
}

type UpsertTaskRequest struct {
	CronExpression string `json:"cron_expression"`
	Enabled        *bool  `json:"enabled"`
}

func (h *Handler) UpsertTask() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.ScheduleStore == nil {
			writeErr(w, r, http.StatusServiceUnavailable, "unavailable", "schedules not available")
			return
		}
		name := mux.Vars(r)["name"]
		def, ok := taskDefByName(name)
		if !ok {
			writeErr(w, r, http.StatusNotFound, "not_found", "unknown task: "+name)
			return
		}
		var body UpsertTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
			return
		}
		if body.CronExpression == "" {
			writeErr(w, r, http.StatusBadRequest, "validation_error", "cron_expression required")
			return
		}
		cronExpr := taskrunner.NormalizeCronExpression(body.CronExpression)
		if err := taskrunner.ValidateCronExpression(cronExpr); err != nil {
			writeErr(w, r, http.StatusBadRequest, "validation_error", "invalid cron_expression: "+err.Error())
			return
		}
		enabled := true
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		sch, err := h.ScheduleStore.UpsertByTaskName(r.Context(), name, cronExpr, enabled)
		if err != nil {
			h.Logger.Error("upsert task schedule failed", "task", name, "err", err)
			writeErr(w, r, http.StatusInternalServerError, "internal", "Failed to save the task schedule.")
			return
		}
		if h.Scheduler != nil {
			_ = h.Scheduler.Refresh(context.Background())
		}
		out := TaskWithSchedule{TaskDef: def, Schedule: &sch}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
}

type PatchTaskRequest struct {
	CronExpression *string `json:"cron_expression"`
	Enabled        *bool   `json:"enabled"`
}

func (h *Handler) PatchTask() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.ScheduleStore == nil {
			writeErr(w, r, http.StatusServiceUnavailable, "unavailable", "schedules not available")
			return
		}
		name := mux.Vars(r)["name"]
		def, ok := taskDefByName(name)
		if !ok {
			writeErr(w, r, http.StatusNotFound, "not_found", "unknown task: "+name)
			return
		}
		sch, err := h.ScheduleStore.GetByTaskName(r.Context(), name)
		if err != nil {
			if errors.Is(err, taskrunner.ErrScheduleNotFound) {
				writeErr(w, r, http.StatusNotFound, "not_found", "schedule not found for task: "+name)
				return
			}
			h.Logger.Error("patch task: load schedule failed", "task", name, "err", err)
			writeErr(w, r, http.StatusInternalServerError, "internal", "Failed to load the task schedule.")
			return
		}
		var body PatchTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, r, http.StatusBadRequest, "validation_error", "invalid JSON: "+err.Error())
			return
		}
		if body.CronExpression != nil {
			cronExpr := taskrunner.NormalizeCronExpression(*body.CronExpression)
			if err := taskrunner.ValidateCronExpression(cronExpr); err != nil {
				writeErr(w, r, http.StatusBadRequest, "validation_error", "invalid cron_expression: "+err.Error())
				return
			}
			sch.CronExpression = cronExpr
		}
		if body.Enabled != nil {
			sch.Enabled = *body.Enabled
		}
		if err := h.ScheduleStore.Update(r.Context(), sch); err != nil {
			h.Logger.Error("patch task: update schedule failed", "task", name, "err", err)
			writeErr(w, r, http.StatusInternalServerError, "internal", "Failed to update the task schedule.")
			return
		}
		sch, err = h.ScheduleStore.GetByTaskName(r.Context(), name)
		if err != nil {
			h.Logger.Error("patch task: reload schedule failed", "task", name, "err", err)
			writeErr(w, r, http.StatusInternalServerError, "internal", "Failed to reload the task schedule.")
			return
		}
		if h.Scheduler != nil {
			_ = h.Scheduler.Refresh(context.Background())
		}
		out := TaskWithSchedule{TaskDef: def, Schedule: &sch}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
}

func (h *Handler) DeleteTaskSchedule() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.ScheduleStore == nil {
			writeErr(w, r, http.StatusServiceUnavailable, "unavailable", "schedules not available")
			return
		}
		name := mux.Vars(r)["name"]
		if !apptasks.TaskNameExists(name) {
			writeErr(w, r, http.StatusNotFound, "not_found", "unknown task: "+name)
			return
		}
		if err := h.ScheduleStore.DeleteByTaskName(r.Context(), name); err != nil {
			if errors.Is(err, taskrunner.ErrScheduleNotFound) {
				writeErr(w, r, http.StatusNotFound, "not_found", "schedule not found for task: "+name)
				return
			}
			h.Logger.Error("delete task schedule failed", "task", name, "err", err)
			writeErr(w, r, http.StatusInternalServerError, "internal", "Failed to delete the task schedule.")
			return
		}
		if h.Scheduler != nil {
			_ = h.Scheduler.Refresh(context.Background())
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
