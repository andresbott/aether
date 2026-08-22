package tasks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func newTestScheduleStore(t *testing.T) *taskrunner.ScheduleStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	store, err := taskrunner.NewScheduleStore(db)
	if err != nil {
		t.Fatalf("schedule store: %v", err)
	}
	return store
}

func TestUpsertAndGetTaskSchedule(t *testing.T) {
	h := &Handler{ScheduleStore: newTestScheduleStore(t)}

	req := httptest.NewRequest(http.MethodPut, "/tasks/scan",
		strings.NewReader(`{"cron_expression":"0 0 0 * * *","enabled":true}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec := httptest.NewRecorder()
	h.UpsertTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/tasks/scan", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec = httptest.NewRecorder()
	h.GetTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", rec.Code)
	}
	var got TaskWithSchedule
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if got.ID != "scan" {
		t.Fatalf("task id = %q, want scan", got.ID)
	}
	if got.Schedule == nil || got.Schedule.CronExpression != "0 0 0 * * *" {
		t.Fatalf("schedule not returned: %+v", got.Schedule)
	}
}

func TestUpsertTaskInvalidCron(t *testing.T) {
	h := &Handler{ScheduleStore: newTestScheduleStore(t)}
	req := httptest.NewRequest(http.MethodPut, "/tasks/scan",
		strings.NewReader(`{"cron_expression":"not a cron","enabled":true}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec := httptest.NewRecorder()
	h.UpsertTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
}

func TestUpsertUnknownTask(t *testing.T) {
	h := &Handler{ScheduleStore: newTestScheduleStore(t)}
	req := httptest.NewRequest(http.MethodPut, "/tasks/nope",
		strings.NewReader(`{"cron_expression":"0 0 0 * * *"}`))
	req = mux.SetURLVars(req, map[string]string{"name": "nope"})
	rec := httptest.NewRecorder()
	h.UpsertTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestPatchTaskSchedule(t *testing.T) {
	h := &Handler{ScheduleStore: newTestScheduleStore(t)}

	// First upsert a schedule for "scan"
	req := httptest.NewRequest(http.MethodPut, "/tasks/scan",
		strings.NewReader(`{"cron_expression":"0 0 0 * * *","enabled":true}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec := httptest.NewRecorder()
	h.UpsertTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	// PATCH to disable the schedule
	req = httptest.NewRequest(http.MethodPatch, "/tasks/scan",
		strings.NewReader(`{"enabled":false}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec = httptest.NewRecorder()
	h.PatchTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var got TaskWithSchedule
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if got.Schedule == nil {
		t.Fatal("expected schedule in response, got nil")
	}
	if got.Schedule.Enabled {
		t.Fatalf("expected Enabled=false, got true")
	}
	if got.Schedule.CronExpression != "0 0 0 * * *" {
		t.Fatalf("cron_expression = %q, want %q", got.Schedule.CronExpression, "0 0 0 * * *")
	}
}

func TestPatchTaskNoSchedule(t *testing.T) {
	h := &Handler{ScheduleStore: newTestScheduleStore(t)}

	// PATCH "scan" which has no schedule
	req := httptest.NewRequest(http.MethodPatch, "/tasks/scan",
		strings.NewReader(`{"enabled":false}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec := httptest.NewRecorder()
	h.PatchTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("patch status = %d, want 404 (%s)", rec.Code, rec.Body.String())
	}
}

func TestDeleteTaskSchedule(t *testing.T) {
	h := &Handler{ScheduleStore: newTestScheduleStore(t)}

	// Upsert a schedule for "scan"
	req := httptest.NewRequest(http.MethodPut, "/tasks/scan",
		strings.NewReader(`{"cron_expression":"0 0 0 * * *","enabled":true}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec := httptest.NewRecorder()
	h.UpsertTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	// DELETE the schedule
	req = httptest.NewRequest(http.MethodDelete, "/tasks/scan", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec = httptest.NewRecorder()
	h.DeleteTaskSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (%s)", rec.Code, rec.Body.String())
	}

	// GET scan and assert schedule is nil
	req = httptest.NewRequest(http.MethodGet, "/tasks/scan", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec = httptest.NewRecorder()
	h.GetTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var got TaskWithSchedule
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if got.Schedule != nil {
		t.Fatalf("expected Schedule=nil after delete, got %+v", got.Schedule)
	}
}

func TestDeleteTaskScheduleNoSchedule(t *testing.T) {
	h := &Handler{ScheduleStore: newTestScheduleStore(t)}

	// DELETE "scan" with no schedule
	req := httptest.NewRequest(http.MethodDelete, "/tasks/scan", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec := httptest.NewRecorder()
	h.DeleteTaskSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete status = %d, want 404 (%s)", rec.Code, rec.Body.String())
	}
}

// TriggerTask's queue-full case used to answer an ad hoc {"message":...}
// body, which the middleware's isJSONObject check forwards untouched (it's
// already a JSON object) — so it never got the router-level problem+json
// fallback the way a bare http.Error would. It must call httperr directly so
// the SPA's shared error parser (which reads detail/title) can surface it.
func TestTriggerTaskQueueFull(t *testing.T) {
	// QueueSize: 1 and the runner is never Start()ed, so nothing ever drains
	// the one waiting slot: the second AddRun is guaranteed to see it full.
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{QueueSize: 1})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	h := &Handler{Runner: runner}

	trigger := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/tasks/scan/trigger", nil)
		req = mux.SetURLVars(req, map[string]string{"name": "scan"})
		rec := httptest.NewRecorder()
		h.TriggerTask().ServeHTTP(rec, req)
		return rec
	}

	if rec := trigger(); rec.Code != http.StatusAccepted {
		t.Fatalf("first trigger status = %d, want 202 (%s)", rec.Code, rec.Body.String())
	}

	rec := trigger()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second trigger status = %d, want 429 (%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var body httperr.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not problem+json: %s", rec.Body.String())
	}
	if got := httperr.Slug(body.Type); got != "queue_full" {
		t.Errorf("slug = %q, want queue_full", got)
	}
	if body.Detail != "Task queue is full. Try again later." {
		t.Errorf("detail = %q, want the queue-full message", body.Detail)
	}
}
