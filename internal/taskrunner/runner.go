package taskrunner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/go-bumbu/tempo"
	"github.com/go-bumbu/tempo/filelog"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrQueueFull = tempo.ErrQueueFull

type Runner struct {
	queue     *tempo.QueueRunner
	logger    *slog.Logger
	logReader tempo.TaskLogReader
}

type Cfg struct {
	Parallelism int
	QueueSize   int
	HistorySize int
	Logger      *slog.Logger
	DB          *gorm.DB
	LogSink     tempo.TaskLogSink
	LogLevel    slog.Level
	LogDir      string
}

// TaskLogGetter reads back a task execution's log as plain text; the tasks HTTP
// handler serves it verbatim as text/plain. *Runner implements it.
type TaskLogGetter interface {
	GetTaskLog(ctx context.Context, executionID uuid.UUID) (string, error)
}

func NewRunner(cfg Cfg) (*Runner, error) {
	if cfg.Parallelism <= 0 {
		cfg.Parallelism = 1
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 20
	}
	if cfg.HistorySize <= 0 {
		cfg.HistorySize = 20
	}

	logSink := cfg.LogSink
	if cfg.LogDir != "" {
		logStore, err := filelog.New(filelog.Config{Dir: cfg.LogDir, DirPerm: 0o750})
		if err != nil {
			return nil, fmt.Errorf("task log sink: %w", err)
		}
		logSink = logStore
	}

	l := cfg.Logger
	if l == nil {
		l = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	var persistence tempo.TaskStatePersistence
	if cfg.DB != nil {
		store, err := NewTaskExecutionStore(cfg.DB, l)
		if err != nil {
			return nil, fmt.Errorf("task execution store: %w", err)
		}
		persistence = store
	} else {
		persistence = tempo.NewMemPersistence()
	}

	qr, err := tempo.NewQueueRunner(tempo.RunnerCfg{
		Parallelism: cfg.Parallelism,
		QueueSize:   cfg.QueueSize,
		HistorySize: cfg.HistorySize,
		Persistence: persistence,
		LogSink:     logSink,
		LogLevel:    cfg.LogLevel,
	})
	if err != nil {
		return nil, fmt.Errorf("queue runner: %w", err)
	}

	var logReader tempo.TaskLogReader
	if lr, ok := logSink.(tempo.TaskLogReader); ok {
		logReader = lr
	}

	return &Runner{
		queue:     qr,
		logger:    l,
		logReader: logReader,
	}, nil
}

// GetTaskLog returns the execution's log as plain text — one
// "<RFC3339Nano-UTC> <LEVEL> <message>" line per entry, empty for an unknown id
// or when no log store is configured. Reproduces the prior on-disk format so the
// /api/v0 task-log endpoint's text/plain output is unchanged.
func (r *Runner) GetTaskLog(ctx context.Context, executionID uuid.UUID) (string, error) {
	if r.logReader == nil {
		return "", nil
	}
	entries, err := r.logReader.Logs(ctx, executionID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.At.UTC().Format(time.RFC3339Nano))
		b.WriteByte(' ')
		b.WriteString(e.Level)
		b.WriteByte(' ')
		b.WriteString(e.Message)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func (r *Runner) Start() {
	r.queue.StartBg()
	r.logger.Info("task runner started", slog.String("component", "taskrunner"))
}

func (r *Runner) Shutdown(ctx context.Context) error {
	r.logger.Info("task runner shutting down", slog.String("component", "taskrunner"))
	return r.queue.ShutDown(ctx)
}

// TaskOption configures a registered task.
type TaskOption func(*taskOpts)

type taskOpts struct {
	singleton bool
}

// Singleton makes a task coalesce on trigger: while an instance of it is
// already waiting or running, triggering it again returns the in-flight
// execution's id instead of piling up a duplicate (see AddRun's reused return).
// Use it for tasks where two concurrent runs would only fight each other, such
// as the library scan.
func Singleton() TaskOption {
	return func(o *taskOpts) { o.singleton = true }
}

func (r *Runner) RegisterTask(fn func(ctx context.Context, log *slog.Logger) error, name string, maxParallelism int, opts ...TaskOption) {
	var o taskOpts
	for _, opt := range opts {
		opt(&o)
	}
	run := r.wrapTaskRun(name, fn)
	var topts []tempo.TaskOption
	if maxParallelism > 0 {
		topts = append(topts, tempo.WithMaxParallelism(maxParallelism))
	}
	if o.singleton {
		topts = append(topts, tempo.WithSingleton())
	}
	r.queue.RegisterRaw(name, run, topts...)
	r.logger.Info("task registered", slog.String("component", "taskrunner"), slog.String("task", name))
}

// wrapTaskRun adapts a task function to tempo's handler signature. tempo hands
// the task a *slog.Logger whose lines are routed to the configured LogSink
// (tagged with the execution id); we pass it straight through to fn. The
// progress reporter and raw params payload are unused for now.
func (r *Runner) wrapTaskRun(name string, fn func(ctx context.Context, log *slog.Logger) error) func(ctx context.Context, log *slog.Logger, _ tempo.Progress, _ []byte) error {
	return func(ctx context.Context, log *slog.Logger, _ tempo.Progress, _ []byte) error {
		r.logger.Info("task started", slog.String("component", "taskrunner"), slog.String("task", name))
		err := fn(ctx, log)
		if err != nil {
			r.logger.Error("task failed", slog.String("component", "taskrunner"), slog.String("task", name), slog.String("error", err.Error()))
			return err
		}
		r.logger.Info("task finished", slog.String("component", "taskrunner"), slog.String("task", name))
		return nil
	}
}

// AddRun enqueues a run of the named task and returns its execution id. The
// bool reports whether the trigger coalesced onto an already waiting/running
// instance of a Singleton task — when true, the returned id is that in-flight
// run's, and nothing new was enqueued.
func (r *Runner) AddRun(name string) (uuid.UUID, bool, error) {
	id, reused, err := r.queue.AddRaw(name, nil)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("enqueue task %q: %w", name, err)
	}
	return id, reused, nil
}

// AddRaw enqueues a run of the named task with a raw params payload, returning
// the execution id and whether it coalesced onto an already waiting/running
// singleton instance. It satisfies tempo's schedule.Enqueuer, so the Scheduler
// can enqueue fires directly onto the runner.
func (r *Runner) AddRaw(name string, params []byte) (uuid.UUID, bool, error) {
	return r.queue.AddRaw(name, params)
}

func (r *Runner) List() []tempo.TaskInfo {
	return r.queue.List()
}

type ExecutionInfo struct {
	ID        uuid.UUID  `json:"id"`
	TaskName  string     `json:"task_name"`
	Status    string     `json:"status"`
	QueuedAt  time.Time  `json:"queued_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time  `json:"ended_at"`
}

func (r *Runner) Executions() []ExecutionInfo {
	raw := r.queue.List()
	out := make([]ExecutionInfo, len(raw))
	for i, t := range raw {
		var startedAt *time.Time
		if !t.StartedAt.IsZero() {
			startedAt = &t.StartedAt
		}
		out[i] = ExecutionInfo{
			ID:        t.ID,
			TaskName:  t.Name,
			Status:    t.Status.Str(),
			QueuedAt:  t.QueuedAt,
			StartedAt: startedAt,
			EndedAt:   t.EndedAt,
		}
	}
	slices.SortFunc(out, func(a, b ExecutionInfo) int {
		if c := b.QueuedAt.Compare(a.QueuedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return out
}

func (r *Runner) Cancel(ctx context.Context, id uuid.UUID) error {
	err := r.queue.Cancel(ctx, id)
	if err != nil {
		return fmt.Errorf("cancel task %s: %w", id, err)
	}
	r.logger.Info("task canceled", slog.String("component", "taskrunner"), slog.String("id", id.String()))
	return nil
}
