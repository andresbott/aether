package tasks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// A schedule is addressed as a sub-resource of its task
// (/tasks/{name}/schedules/{id}); patching it under a different task's name must
// not edit it — it does not belong to that task, so the answer is 404.
func TestPatchScheduleUnderWrongTaskIs404(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	sc := createSchedule(t, h, "scan", `{"cron_expression":"0 0 0 * * *","enabled":true}`)

	req := httptest.NewRequest(http.MethodPatch, "/tasks/scan-full/schedules/"+sc.ID, strings.NewReader(`{"enabled":false}`))
	req = mux.SetURLVars(req, map[string]string{"name": "scan-full", "id": sc.ID})
	rec := httptest.NewRecorder()
	h.PatchSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("patch under wrong task: status = %d, want 404 (%s)", rec.Code, rec.Body.String())
	}
}

// Likewise a delete under the wrong task must 404 and leave the schedule intact.
func TestDeleteScheduleUnderWrongTaskIs404(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	sc := createSchedule(t, h, "scan", `{"cron_expression":"0 0 0 * * *","enabled":true}`)

	req := httptest.NewRequest(http.MethodDelete, "/tasks/scan-full/schedules/"+sc.ID, nil)
	req = mux.SetURLVars(req, map[string]string{"name": "scan-full", "id": sc.ID})
	rec := httptest.NewRecorder()
	h.DeleteSchedule().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete under wrong task: status = %d, want 404 (%s)", rec.Code, rec.Body.String())
	}
	if _, err := h.Schedules.Get(context.Background(), sc.ID); err != nil {
		t.Fatalf("schedule should survive a wrong-task delete, got: %v", err)
	}
}

// An explicit `{"params":null}` on create must be treated as an omitted params
// field (nil), exactly as PatchSchedule does, so a created schedule never
// round-trips a bare `null` for its params object.
func TestCreateScheduleNormalizesNullParams(t *testing.T) {
	h := &Handler{Schedules: newTestScheduler(t)}
	created := createSchedule(t, h, "scan", `{"cron_expression":"0 0 0 * * *","params":null}`)
	if len(created.Params) != 0 {
		t.Fatalf("explicit params:null should store no params, got %q", created.Params)
	}
}
