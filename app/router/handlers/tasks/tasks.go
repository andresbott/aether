package tasks

import (
	"context"
	"encoding/json"
	"errors"
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

func (h *Handler) ListTasks() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out := make([]TaskWithSchedule, 0, len(apptasks.AvailableTasks))
		var scheduleMap map[string]taskrunner.Schedule
		if h.ScheduleStore != nil {
			list, err := h.ScheduleStore.List(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, "unknown task: "+name, http.StatusNotFound)
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
			http.Error(w, "unknown task: "+name, http.StatusNotFound)
			return
		}
		id, err := h.Runner.AddRun(name)
		if err != nil {
			if errors.Is(err, taskrunner.ErrQueueFull) {
				httperr.Write(w, r, http.StatusTooManyRequests, "queue_full", httperr.TitleFor("queue_full"), "Task queue is full. Try again later.")
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]taskrunner.ExecutionInfo{"executions": executions})
	})
}

func (h *Handler) CancelExecution() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(mux.Vars(r)["id"])
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
		id, err := uuid.Parse(mux.Vars(r)["id"])
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
			http.Error(w, "schedules not available", http.StatusServiceUnavailable)
			return
		}
		name := mux.Vars(r)["name"]
		def, ok := taskDefByName(name)
		if !ok {
			http.Error(w, "unknown task: "+name, http.StatusNotFound)
			return
		}
		var body UpsertTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.CronExpression == "" {
			http.Error(w, "cron_expression required", http.StatusBadRequest)
			return
		}
		cronExpr := taskrunner.NormalizeCronExpression(body.CronExpression)
		if err := taskrunner.ValidateCronExpression(cronExpr); err != nil {
			http.Error(w, "invalid cron_expression: "+err.Error(), http.StatusBadRequest)
			return
		}
		enabled := true
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		sch, err := h.ScheduleStore.UpsertByTaskName(r.Context(), name, cronExpr, enabled)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, "schedules not available", http.StatusServiceUnavailable)
			return
		}
		name := mux.Vars(r)["name"]
		def, ok := taskDefByName(name)
		if !ok {
			http.Error(w, "unknown task: "+name, http.StatusNotFound)
			return
		}
		sch, err := h.ScheduleStore.GetByTaskName(r.Context(), name)
		if err != nil {
			if errors.Is(err, taskrunner.ErrScheduleNotFound) {
				http.Error(w, "schedule not found for task: "+name, http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var body PatchTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.CronExpression != nil {
			cronExpr := taskrunner.NormalizeCronExpression(*body.CronExpression)
			if err := taskrunner.ValidateCronExpression(cronExpr); err != nil {
				http.Error(w, "invalid cron_expression: "+err.Error(), http.StatusBadRequest)
				return
			}
			sch.CronExpression = cronExpr
		}
		if body.Enabled != nil {
			sch.Enabled = *body.Enabled
		}
		if err := h.ScheduleStore.Update(r.Context(), sch); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sch, err = h.ScheduleStore.GetByTaskName(r.Context(), name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, "schedules not available", http.StatusServiceUnavailable)
			return
		}
		name := mux.Vars(r)["name"]
		if !apptasks.TaskNameExists(name) {
			http.Error(w, "unknown task: "+name, http.StatusNotFound)
			return
		}
		if err := h.ScheduleStore.DeleteByTaskName(r.Context(), name); err != nil {
			if errors.Is(err, taskrunner.ErrScheduleNotFound) {
				http.Error(w, "schedule not found for task: "+name, http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if h.Scheduler != nil {
			_ = h.Scheduler.Refresh(context.Background())
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
