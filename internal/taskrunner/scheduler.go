package taskrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-bumbu/tempo/dbschedule"
	"github.com/go-bumbu/tempo/schedule"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Schedule is aether's API-facing view of one task schedule. Schedules are
// addressed by their own id: the underlying tempo scheduler is uuid-keyed and
// allows several schedules per task (e.g. an hourly incremental scan and a
// nightly full scan), and this façade no longer restricts that to one.
type Schedule struct {
	ID             string          `json:"id"`
	TaskName       string          `json:"task_name"`
	CronExpression string          `json:"cron_expression"`
	Params         json.RawMessage `json:"params,omitempty"`
	Enabled        bool            `json:"enabled"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ErrScheduleNotFound is returned when no schedule exists for the given id.
var ErrScheduleNotFound = schedule.ErrScheduleNotFound

// NormalizeCronExpression and ValidateCronExpression re-export tempo's cron
// helpers so callers (the HTTP handlers) need not import tempo directly.
//
// tempo's NormalizeCron is a superset of the old hand-rolled version: besides
// prepending a "0" seconds field to a 5-field Unix expression, it translates the
// day-of-week field from Unix numbering (0-6, Sun=0) to Quartz (1-7, Sun=1). The
// old code skipped that translation, so a 5-field weekday expression was
// scheduled one day off; it is now correct.
func NormalizeCronExpression(cron string) string { return schedule.NormalizeCron(cron) }

func ValidateCronExpression(cron string) error { return schedule.ValidateCron(cron) }

// Scheduler fires registered tasks on a cron timetable. It is a thin façade
// over tempo's schedule.Scheduler presenting an id-addressable API and
// aether's own Schedule view type. tempo owns the write path atomically
// (persist + reschedule in one call), so there is no separate "refresh" step.
type Scheduler struct {
	sched *schedule.Scheduler
}

// SchedulerCfg configures NewScheduler.
type SchedulerCfg struct {
	// DB backs the schedule store (the tempo_schedules table). Required.
	DB *gorm.DB
	// Enqueuer receives a task when a schedule fires. *Runner satisfies it.
	Enqueuer schedule.Enqueuer
	// Logger receives fire/skip messages; nil uses slog.Default().
	Logger *slog.Logger
}

// Verify *Runner can be used as the scheduler's enqueuer.
var _ schedule.Enqueuer = (*Runner)(nil)

func NewScheduler(cfg SchedulerCfg) (*Scheduler, error) {
	if cfg.DB == nil {
		return nil, errors.New("db is required")
	}
	if cfg.Enqueuer == nil {
		return nil, errors.New("enqueuer is required")
	}
	store, err := dbschedule.New(cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("schedule store: %w", err)
	}
	s, err := schedule.New(schedule.Cfg{
		Store:    store,
		Enqueuer: cfg.Enqueuer,
		Logger:   cfg.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("scheduler: %w", err)
	}
	return &Scheduler{sched: s}, nil
}

// Start loads stored schedules and begins firing. It must be called before the
// write methods below (tempo rejects writes on an unstarted scheduler).
func (s *Scheduler) Start(ctx context.Context) error { return s.sched.Start(ctx) }

// Stop stops firing and waits for in-flight fires, bounded by ctx.
func (s *Scheduler) Stop(ctx context.Context) error { return s.sched.ShutDown(ctx) }

func toSchedule(si schedule.ScheduleInfo) Schedule {
	return Schedule{
		ID:             si.ID.String(),
		TaskName:       si.TaskName,
		CronExpression: si.Cron,
		Params:         si.Params,
		Enabled:        si.Enabled,
		CreatedAt:      si.CreatedAt,
		UpdatedAt:      si.UpdatedAt,
	}
}

// List returns every schedule, enabled or not.
func (s *Scheduler) List(ctx context.Context) ([]Schedule, error) {
	list, err := s.sched.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Schedule, len(list))
	for i, si := range list {
		out[i] = toSchedule(si)
	}
	return out, nil
}

// ListByTaskName returns every schedule registered for a task, newest-id order
// as tempo lists them. Empty (not an error) when the task has no schedules.
func (s *Scheduler) ListByTaskName(ctx context.Context, name string) ([]Schedule, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Schedule, 0)
	for _, sc := range all {
		if sc.TaskName == name {
			out = append(out, sc)
		}
	}
	return out, nil
}

// Get returns one schedule by id, or ErrScheduleNotFound.
func (s *Scheduler) Get(ctx context.Context, id string) (Schedule, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return Schedule{}, ErrScheduleNotFound
	}
	si, err := s.sched.Get(ctx, uid)
	if err != nil {
		if errors.Is(err, ErrScheduleNotFound) {
			return Schedule{}, ErrScheduleNotFound
		}
		return Schedule{}, err
	}
	return toSchedule(si), nil
}

// Create adds a new schedule for a task. cron should already be validated;
// tempo validates and normalizes it again. params may be nil.
func (s *Scheduler) Create(ctx context.Context, taskName, cron string, enabled bool, params json.RawMessage) (Schedule, error) {
	out, err := s.sched.Create(ctx, schedule.Schedule{
		TaskName: taskName,
		Cron:     cron,
		Enabled:  enabled,
		Params:   params,
	})
	if err != nil {
		return Schedule{}, err
	}
	return toSchedule(out), nil
}

// Update partially updates a schedule by id: a nil cron/enabled keeps the
// current value; a non-nil params replaces it, nil params keeps it. Returns
// ErrScheduleNotFound for an unknown id.
func (s *Scheduler) Update(ctx context.Context, id string, cron *string, enabled *bool, params json.RawMessage) (Schedule, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return Schedule{}, ErrScheduleNotFound
	}
	existing, err := s.Get(ctx, id)
	if err != nil {
		return Schedule{}, err
	}
	upd := schedule.Schedule{
		ID:       uid,
		TaskName: existing.TaskName,
		Cron:     existing.CronExpression,
		Enabled:  existing.Enabled,
		Params:   existing.Params,
	}
	if cron != nil {
		upd.Cron = *cron
	}
	if enabled != nil {
		upd.Enabled = *enabled
	}
	if params != nil {
		upd.Params = params
	}
	out, err := s.sched.Update(ctx, upd)
	if err != nil {
		return Schedule{}, err
	}
	return toSchedule(out), nil
}

// Delete removes a schedule by id, or returns ErrScheduleNotFound.
func (s *Scheduler) Delete(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return ErrScheduleNotFound
	}
	if err := s.sched.Delete(ctx, uid); err != nil {
		if errors.Is(err, ErrScheduleNotFound) {
			return ErrScheduleNotFound
		}
		return err
	}
	return nil
}
