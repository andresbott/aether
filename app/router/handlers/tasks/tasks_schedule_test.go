package tasks

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	apptasks "github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func newTestScheduler(t *testing.T) *taskrunner.Scheduler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	sched, err := taskrunner.NewScheduler(taskrunner.SchedulerCfg{DB: db, Enqueuer: runner})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	if err := sched.Start(context.Background()); err != nil {
		t.Fatalf("start scheduler: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = sched.Stop(ctx)
	})
	return sched
}

// createSchedule POSTs a schedule for name and fails the test unless the
// response is 201, returning the decoded created schedule.
func createSchedule(t *testing.T, h *Handler, name, body string) taskrunner.Schedule {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+name+"/schedules", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"name": name})
	rec := httptest.NewRecorder()
	h.CreateSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	var created taskrunner.Schedule
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return created
}

func TestCreateAndGetTaskSchedule(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}

	created := createSchedule(t, h, "scan", `{"cron_expression":"0 0 0 * * *","enabled":true,"params":{"full":true}}`)
	if created.ID == "" {
		t.Fatal("expected a non-empty schedule id")
	}
	if string(created.Params) != `{"full":true}` {
		t.Fatalf("params = %q, want {\"full\":true}", created.Params)
	}

	req := httptest.NewRequest(http.MethodGet, "/tasks/scan", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec := httptest.NewRecorder()
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
	if len(got.Schedules) != 1 || got.Schedules[0].CronExpression != "0 0 0 * * *" {
		t.Fatalf("schedule not returned: %+v", got.Schedules)
	}
	if string(got.Schedules[0].Params) != `{"full":true}` {
		t.Fatalf("params not returned: %+v", got.Schedules[0])
	}
}

// A task may carry more than one schedule at once (e.g. an hourly incremental
// scan and a nightly full scan) — this is the whole point of Task 5.
func TestCreateTaskSchedule_MultiplePerTask(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}

	fast := createSchedule(t, h, "scan", `{"cron_expression":"0 0 * * * *","params":{"full":false}}`)
	full := createSchedule(t, h, "scan", `{"cron_expression":"0 0 3 * * *","params":{"full":true}}`)
	if fast.ID == full.ID {
		t.Fatalf("expected two distinct schedule ids, got the same %q twice", fast.ID)
	}

	req := httptest.NewRequest(http.MethodGet, "/tasks/scan", nil)
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec := httptest.NewRecorder()
	h.GetTask().ServeHTTP(rec, req)
	var got TaskWithSchedule
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if len(got.Schedules) != 2 {
		t.Fatalf("expected 2 schedules for scan, got %d: %+v", len(got.Schedules), got.Schedules)
	}
}

func TestCreateScheduleInvalidCron(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	req := httptest.NewRequest(http.MethodPost, "/tasks/scan/schedules",
		strings.NewReader(`{"cron_expression":"not a cron","enabled":true}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan"})
	rec := httptest.NewRecorder()
	h.CreateSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
}

func TestCreateScheduleUnknownTask(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	req := httptest.NewRequest(http.MethodPost, "/tasks/nope/schedules",
		strings.NewReader(`{"cron_expression":"0 0 0 * * *"}`))
	req = mux.SetURLVars(req, map[string]string{"name": "nope"})
	rec := httptest.NewRecorder()
	h.CreateSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestPatchSchedule(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	created := createSchedule(t, h, "scan", `{"cron_expression":"0 0 0 * * *","enabled":true}`)

	req := httptest.NewRequest(http.MethodPatch, "/tasks/scan/schedules/"+created.ID,
		strings.NewReader(`{"enabled":false}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan", "id": created.ID})
	rec := httptest.NewRecorder()
	h.PatchSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var got taskrunner.Schedule
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if got.Enabled {
		t.Fatalf("expected Enabled=false, got true")
	}
	if got.CronExpression != "0 0 0 * * *" {
		t.Fatalf("cron_expression = %q, want %q", got.CronExpression, "0 0 0 * * *")
	}
}

// An explicit JSON null for params (`{"params":null}`) must be treated the
// same as an omitted params field — "leave params unchanged" — not as a
// literal null overwrite (PatchTaskScheduleRequest.params' documented
// contract; the OpenAPI TaskSchedule.params schema also disallows null).
func TestPatchScheduleExplicitNullParamsKeepsExisting(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	created := createSchedule(t, h, "scan", `{"cron_expression":"0 0 0 * * *","enabled":true,"params":{"full":true}}`)

	req := httptest.NewRequest(http.MethodPatch, "/tasks/scan/schedules/"+created.ID,
		strings.NewReader(`{"enabled":false,"params":null}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan", "id": created.ID})
	rec := httptest.NewRecorder()
	h.PatchSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

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
	if len(got.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %+v", got.Schedules)
	}
	sch := got.Schedules[0]
	if sch.Enabled {
		t.Fatalf("expected Enabled=false, got true")
	}
	if string(sch.Params) != `{"full":true}` {
		t.Fatalf("params = %q, want {\"full\":true} (explicit null must keep existing params)", sch.Params)
	}
}

func TestPatchScheduleUnknownID(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodPatch, "/tasks/scan/schedules/"+id,
		strings.NewReader(`{"enabled":false}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan", "id": id})
	rec := httptest.NewRecorder()
	h.PatchSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("patch status = %d, want 404 (%s)", rec.Code, rec.Body.String())
	}
}

func TestDeleteSchedule(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	created := createSchedule(t, h, "scan", `{"cron_expression":"0 0 0 * * *","enabled":true}`)

	req := httptest.NewRequest(http.MethodDelete, "/tasks/scan/schedules/"+created.ID, nil)
	req = mux.SetURLVars(req, map[string]string{"name": "scan", "id": created.ID})
	rec := httptest.NewRecorder()
	h.DeleteSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (%s)", rec.Code, rec.Body.String())
	}

	// GET scan and assert the schedule list no longer contains it.
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
	if len(got.Schedules) != 0 {
		t.Fatalf("expected no schedules after delete, got %+v", got.Schedules)
	}
}

func TestDeleteScheduleUnknownID(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodDelete, "/tasks/scan/schedules/"+id, nil)
	req = mux.SetURLVars(req, map[string]string{"name": "scan", "id": id})
	rec := httptest.NewRecorder()
	h.DeleteSchedule().ServeHTTP(rec, req)
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

// A singleton task (scan) that is triggered again while already queued must
// coalesce: the handler answers 202 with the in-flight execution id and
// reused=true, rather than enqueuing a duplicate.
func TestTriggerTaskSingletonCoalesces(t *testing.T) {
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	// Registered Singleton but never Started, so the first run stays queued and
	// the second trigger has an in-flight instance to coalesce onto.
	runner.RegisterTask(func(context.Context, *slog.Logger) error { return nil }, apptasks.ScanTaskName, 1, taskrunner.Singleton())
	h := &Handler{Runner: runner}

	type triggerResp struct {
		ExecutionID string `json:"execution_id"`
		Reused      bool   `json:"reused"`
	}
	trigger := func() (int, triggerResp) {
		req := httptest.NewRequest(http.MethodPost, "/tasks/"+apptasks.ScanTaskName+"/trigger", nil)
		req = mux.SetURLVars(req, map[string]string{"name": apptasks.ScanTaskName})
		rec := httptest.NewRecorder()
		h.TriggerTask().ServeHTTP(rec, req)
		var body triggerResp
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode trigger body: %s", rec.Body.String())
		}
		return rec.Code, body
	}

	code1, first := trigger()
	if code1 != http.StatusAccepted {
		t.Fatalf("first trigger status = %d, want 202", code1)
	}
	if first.ExecutionID == "" {
		t.Fatal("first trigger must return an execution_id")
	}
	if first.Reused {
		t.Fatal("first trigger must report reused=false")
	}

	code2, second := trigger()
	if code2 != http.StatusAccepted {
		t.Fatalf("second trigger status = %d, want 202", code2)
	}
	if !second.Reused {
		t.Fatal("second trigger of a singleton task must report reused=true")
	}
	if second.ExecutionID != first.ExecutionID {
		t.Fatalf("coalesced trigger execution_id = %q, want the in-flight id %q", second.ExecutionID, first.ExecutionID)
	}
}

func TestTriggerTaskForwardsBodyAsParams(t *testing.T) {
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{QueueSize: 4})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	h := &Handler{Runner: runner}

	// A params body is forwarded verbatim to the enqueued task.
	req := httptest.NewRequest(http.MethodPost, "/tasks/scan/trigger",
		strings.NewReader(`{"full":true}`))
	req = mux.SetURLVars(req, map[string]string{"name": apptasks.ScanTaskName})
	rec := httptest.NewRecorder()
	h.TriggerTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (%s)", rec.Code, rec.Body.String())
	}
	list := runner.List()
	if len(list) != 1 {
		t.Fatalf("queued %d tasks, want 1", len(list))
	}
	if string(list[0].Params) != `{"full":true}` {
		t.Fatalf("queued params = %q, want {\"full\":true}", list[0].Params)
	}

	// An empty body enqueues nil params (an incremental scan).
	runner2, err := taskrunner.NewRunner(taskrunner.Cfg{QueueSize: 4})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	h2 := &Handler{Runner: runner2}
	req = httptest.NewRequest(http.MethodPost, "/tasks/scan/trigger", nil)
	req = mux.SetURLVars(req, map[string]string{"name": apptasks.ScanTaskName})
	rec = httptest.NewRecorder()
	h2.TriggerTask().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("empty-body status = %d, want 202 (%s)", rec.Code, rec.Body.String())
	}
	list = runner2.List()
	if len(list) != 1 || len(list[0].Params) != 0 {
		t.Fatalf("empty body should queue nil params, got %q", list[0].Params)
	}
}
