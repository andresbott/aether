package tasks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
