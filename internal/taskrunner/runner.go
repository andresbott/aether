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
	// LogSink is used as the task log sink when LogDir is empty; ignored otherwise.
	LogSink  tempo.TaskLogSink
	LogLevel slog.Level
	// LogDir, if set, takes precedence over LogSink: NewRunner builds a
	// filelog.Store rooted here and any injected LogSink is ignored.
	LogDir string
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
	singleton      bool
	exclusionGroup string
}

// Singleton makes a task coalesce on trigger: while an instance of it is
// already waiting or running, triggering it again returns the in-flight
// execution's id instead of piling up a duplicate (see AddRun's reused return).
// Use it for tasks where two concurrent runs would only fight each other, such
// as the library scan.
func Singleton() TaskOption {
	return func(o *taskOpts) { o.singleton = true }
}

// ExclusionGroup puts the task in a named tempo exclusion group: at most one
// task in the group runs at a time, across all task names in the group. Use it
// to serialize different tasks that must not touch the same resource
// concurrently — here, "scan" and "reindex" over SQLite's single write lock.
func ExclusionGroup(name string) TaskOption {
	return func(o *taskOpts) { o.exclusionGroup = name }
}

// tempoOptions renders the accumulated task options — plus a max-parallelism
// bound — into tempo's option list, shared by the raw (RegisterTask) and typed
// (Register) register paths.
func (o taskOpts) tempoOptions(maxParallelism int) []tempo.TaskOption {
	var topts []tempo.TaskOption
	if maxParallelism > 0 {
		topts = append(topts, tempo.WithMaxParallelism(maxParallelism))
	}
	if o.singleton {
		topts = append(topts, tempo.WithSingleton())
	}
	if o.exclusionGroup != "" {
		topts = append(topts, tempo.WithExclusionGroup(o.exclusionGroup))
	}
	return topts
}

func (r *Runner) RegisterTask(fn func(ctx context.Context, log *slog.Logger) error, name string, maxParallelism int, opts ...TaskOption) {
	var o taskOpts
	for _, opt := range opts {
		opt(&o)
	}
	run := r.wrapTaskRun(name, fn)
	r.queue.RegisterRaw(name, run, o.tempoOptions(maxParallelism)...)
	r.logger.Info("task registered", slog.String("component", "taskrunner"), slog.String("task", name))
}

// runWithLog runs a task body between the wrapper's start/finish/fail log
// lines, shared by the raw (RegisterTask) and typed (Register) paths.
func (r *Runner) runWithLog(_ context.Context, name string, call func() error) error {
	r.logger.Info("task started", slog.String("component", "taskrunner"), slog.String("task", name))
	if err := call(); err != nil {
		r.logger.Error("task failed", slog.String("component", "taskrunner"), slog.String("task", name), slog.String("error", err.Error()))
		return err
	}
	r.logger.Info("task finished", slog.String("component", "taskrunner"), slog.String("task", name))
	return nil
}

// wrapTaskRun adapts a task function to tempo's handler signature. tempo hands
// the task a *slog.Logger whose lines are routed to the configured LogSink
// (tagged with the execution id); we pass it straight through to fn. The
// progress reporter and raw params payload are unused for now.
func (r *Runner) wrapTaskRun(name string, fn func(ctx context.Context, log *slog.Logger) error) func(ctx context.Context, log *slog.Logger, _ tempo.Progress, _ []byte) error {
	return func(ctx context.Context, log *slog.Logger, _ tempo.Progress, _ []byte) error {
		return r.runWithLog(ctx, name, func() error { return fn(ctx, log) })
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
	id, reused, err := r.queue.AddRaw(name, params)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("enqueue task %q: %w", name, err)
	}
	return id, reused, nil
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

// Register adds a typed task: tempo JSON-decodes the enqueued params into T
// before fn runs (an empty payload yields a zero-value T). It preserves the
// same start/finish/fail logging as RegisterTask. Package-level rather than a
// method because Go methods cannot have type parameters.
func Register[T any](r *Runner, fn func(ctx context.Context, log *slog.Logger, params T) error, name string, maxParallelism int, opts ...TaskOption) {
	var o taskOpts
	for _, opt := range opts {
		opt(&o)
	}
	tempo.Register[T](r.queue, name, func(ctx context.Context, log *slog.Logger, _ tempo.Progress, p T) error {
		return r.runWithLog(ctx, name, func() error { return fn(ctx, log, p) })
	}, o.tempoOptions(maxParallelism)...)
	r.logger.Info("task registered", slog.String("component", "taskrunner"), slog.String("task", name))
}

// Enqueue enqueues a run of a typed task with JSON-encoded params, returning the
// execution id and whether it coalesced onto an in-flight singleton instance.
func Enqueue[T any](r *Runner, name string, params T) (uuid.UUID, bool, error) {
	id, reused, err := tempo.Enqueue[T](r.queue, name, params)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("enqueue task %q: %w", name, err)
	}
	return id, reused, nil
}
