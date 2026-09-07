package taskrunner

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/go-bumbu/tempo/dbschedule"
	"github.com/go-bumbu/tempo/schedule"
	"gorm.io/gorm"
)

// Schedule is aether's API-facing view of one task schedule. The scheduler keeps
// at most one Schedule per task (task-name-keyed), even though the underlying
// tempo scheduler is uuid-keyed and would allow several per task.
type Schedule struct {
	ID             string    `json:"id"`
	TaskName       string    `json:"task_name"`
	CronExpression string    `json:"cron_expression"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ErrScheduleNotFound is returned when no schedule exists for a task.
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

// Scheduler fires registered tasks on a cron timetable. It is a thin façade over
// tempo's schedule.Scheduler presenting a one-schedule-per-task, task-name-keyed
// API. tempo owns the write path atomically (persist + reschedule in one call),
// so there is no separate "refresh" step.
type Scheduler struct {
	sched *schedule.Scheduler
	// mu serializes the read-then-write in the write methods below, so two
	// concurrent UpsertByTaskName calls for the same task cannot both miss in
	// findByTaskName and both Create — which would leave two schedule rows for
	// one task (the store has no unique index on task_name, matching tempo's
	// multi-schedule design). aether runs a single scheduler process, tempo's
	// own "one process per store" assumption, so an in-process mutex is the
	// right scope.
	mu sync.Mutex
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
		return nil, err
	}
	s, err := schedule.New(schedule.Cfg{
		Store:    store,
		Enqueuer: cfg.Enqueuer,
		Logger:   cfg.Logger,
	})
	if err != nil {
		return nil, err
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

// findByTaskName returns the single schedule for a task, or ErrScheduleNotFound.
// This is where the one-schedule-per-task invariant lives now that the store no
// longer carries a unique index on task_name.
func (s *Scheduler) findByTaskName(ctx context.Context, name string) (schedule.ScheduleInfo, error) {
	list, err := s.sched.List(ctx)
	if err != nil {
		return schedule.ScheduleInfo{}, err
	}
	for _, si := range list {
		if si.TaskName == name {
			return si, nil
		}
	}
	return schedule.ScheduleInfo{}, ErrScheduleNotFound
}

// GetByTaskName returns the schedule for a task, or ErrScheduleNotFound.
func (s *Scheduler) GetByTaskName(ctx context.Context, name string) (Schedule, error) {
	si, err := s.findByTaskName(ctx, name)
	if err != nil {
		return Schedule{}, err
	}
	return toSchedule(si), nil
}

// UpsertByTaskName enforces one schedule per task: it updates the task's existing
// schedule when there is one, otherwise creates it. cron should already be
// validated (ValidateCronExpression); tempo validates and normalizes it again.
func (s *Scheduler) UpsertByTaskName(ctx context.Context, name, cron string, enabled bool) (Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.findByTaskName(ctx, name)
	switch {
	case err == nil:
		out, uErr := s.sched.Update(ctx, schedule.Schedule{
			ID:       existing.ID,
			TaskName: name,
			Cron:     cron,
			Enabled:  enabled,
		})
		if uErr != nil {
			return Schedule{}, uErr
		}
		return toSchedule(out), nil
	case errors.Is(err, ErrScheduleNotFound):
		out, cErr := s.sched.Create(ctx, schedule.Schedule{
			TaskName: name,
			Cron:     cron,
			Enabled:  enabled,
		})
		if cErr != nil {
			return Schedule{}, cErr
		}
		return toSchedule(out), nil
	default:
		return Schedule{}, err
	}
}

// PatchByTaskName updates cron and/or enabled on a task's existing schedule,
// leaving a nil field unchanged. Returns ErrScheduleNotFound when the task has no
// schedule. A non-nil cron should already be validated.
func (s *Scheduler) PatchByTaskName(ctx context.Context, name string, cron *string, enabled *bool) (Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.findByTaskName(ctx, name)
	if err != nil {
		return Schedule{}, err
	}
	upd := schedule.Schedule{
		ID:       existing.ID,
		TaskName: name,
		Cron:     existing.Cron,
		Enabled:  existing.Enabled,
	}
	if cron != nil {
		upd.Cron = *cron
	}
	if enabled != nil {
		upd.Enabled = *enabled
	}
	out, err := s.sched.Update(ctx, upd)
	if err != nil {
		return Schedule{}, err
	}
	return toSchedule(out), nil
}

// DeleteByTaskName removes a task's schedule, or returns ErrScheduleNotFound.
func (s *Scheduler) DeleteByTaskName(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.findByTaskName(ctx, name)
	if err != nil {
		return err
	}
	return s.sched.Delete(ctx, existing.ID)
}
