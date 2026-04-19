package taskrunner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/reugn/go-quartz/quartz"
)

type TaskEnqueuer interface {
	EnqueueTask(ctx context.Context, taskName string) error
}

type FuncEnqueuer func(ctx context.Context, taskName string) error

func (f FuncEnqueuer) EnqueueTask(ctx context.Context, taskName string) error {
	return f(ctx, taskName)
}

type enqueueJob struct {
	taskName string
	enqueuer TaskEnqueuer
}

func (j *enqueueJob) Execute(ctx context.Context) error {
	return j.enqueuer.EnqueueTask(ctx, j.taskName)
}

func (j *enqueueJob) Description() string {
	return fmt.Sprintf("enqueue task %q", j.taskName)
}

type Scheduler struct {
	quartzSched quartz.Scheduler
	store       *ScheduleStore
	enqueuer    TaskEnqueuer
	logger      *slog.Logger
	mu          sync.Mutex
}

type SchedulerCfg struct {
	ScheduleStore *ScheduleStore
	Enqueuer      TaskEnqueuer
	Logger        *slog.Logger
}

func NormalizeCronExpression(cron string) string {
	cron = strings.TrimSpace(cron)
	parts := strings.Fields(cron)
	if len(parts) == 5 {
		return "0 " + cron
	}
	return cron
}

func ValidateCronExpression(cron string) error {
	if cron == "" {
		return fmt.Errorf("cron expression is required")
	}
	cron = NormalizeCronExpression(cron)
	_, err := quartz.NewCronTrigger(cron)
	return err
}

func NewScheduler(cfg SchedulerCfg) (*Scheduler, error) {
	if cfg.ScheduleStore == nil {
		return nil, fmt.Errorf("schedule store is required")
	}
	if cfg.Enqueuer == nil {
		return nil, fmt.Errorf("enqueuer is required")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	qs, err := quartz.NewStdScheduler()
	if err != nil {
		return nil, fmt.Errorf("create quartz scheduler: %w", err)
	}
	return &Scheduler{
		quartzSched: qs,
		store:       cfg.ScheduleStore,
		enqueuer:    cfg.Enqueuer,
		logger:      logger,
	}, nil
}

func (s *Scheduler) Start(ctx context.Context) {
	s.quartzSched.Start(ctx)
	if err := s.loadSchedules(ctx); err != nil {
		s.logger.Error("failed to load task schedules", slog.String("component", "taskrunner"), slog.String("error", err.Error()))
	}
}

func (s *Scheduler) loadSchedules(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.quartzSched.Clear(); err != nil {
		return err
	}
	list, err := s.store.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for _, sch := range list {
		expr := NormalizeCronExpression(sch.CronExpression)
		trigger, err := quartz.NewCronTrigger(expr)
		if err != nil {
			s.logger.Warn("invalid cron expression for task, skipping",
				slog.String("component", "taskrunner"),
				slog.String("task", sch.TaskName),
				slog.String("cron", sch.CronExpression),
				slog.String("error", err.Error()))
			continue
		}
		job := &enqueueJob{taskName: sch.TaskName, enqueuer: s.enqueuer}
		jobKey := quartz.NewJobKey("task:" + sch.TaskName)
		detail := quartz.NewJobDetail(job, jobKey)
		if err := s.quartzSched.ScheduleJob(detail, trigger); err != nil {
			s.logger.Warn("failed to schedule task",
				slog.String("component", "taskrunner"),
				slog.String("task", sch.TaskName),
				slog.String("error", err.Error()))
			continue
		}
		s.logger.Info("scheduled task",
			slog.String("component", "taskrunner"),
			slog.String("task", sch.TaskName),
			slog.String("cron", sch.CronExpression))
	}
	return nil
}

func (s *Scheduler) Refresh(ctx context.Context) error {
	return s.loadSchedules(ctx)
}

func (s *Scheduler) Stop() {
	s.quartzSched.Stop()
}

func (s *Scheduler) Wait(ctx context.Context) {
	s.quartzSched.Wait(ctx)
}
