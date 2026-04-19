package taskrunner

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/go-bumbu/tempo"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type dbTaskExecution struct {
	ID        string    `gorm:"primaryKey;type:text;column:id"`
	Name      string    `gorm:"not null;index;column:name"`
	Status    int       `gorm:"not null;column:status"`
	QueuedAt  time.Time `gorm:"not null;index;column:queued_at"`
	StartedAt time.Time `gorm:"column:started_at"`
	EndedAt   time.Time `gorm:"column:ended_at"`
}

func (dbTaskExecution) TableName() string { return "task_executions" }

type TaskExecutionStore struct {
	db      *gorm.DB
	logger  *slog.Logger
	cleaner TaskLogCleaner
}

func NewTaskExecutionStore(db *gorm.DB, logger *slog.Logger, cleaner TaskLogCleaner) (*TaskExecutionStore, error) {
	if db == nil {
		return nil, nil
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if err := db.AutoMigrate(&dbTaskExecution{}); err != nil {
		return nil, err
	}
	ctx := context.Background()
	now := time.Now()
	res := db.WithContext(ctx).Model(&dbTaskExecution{}).
		Where("status = ?", int(tempo.TaskStatusRunning)).
		Updates(map[string]interface{}{
			"status":   int(tempo.TaskStatusFailed),
			"ended_at": now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected > 0 {
		logger.Warn("marked leftover running task(s) as failed on startup (e.g. after crash)",
			slog.String("component", "taskrunner"),
			slog.Int64("count", res.RowsAffected))
	}
	return &TaskExecutionStore{db: db, logger: logger, cleaner: cleaner}, nil
}

func (s *TaskExecutionStore) SaveTask(ctx context.Context, task tempo.TaskInfo) error {
	row := dbTaskExecution{
		ID:        task.ID.String(),
		Name:      task.Name,
		Status:    int(task.Status),
		QueuedAt:  task.QueuedAt,
		StartedAt: task.StartedAt,
		EndedAt:   task.EndedAt,
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *TaskExecutionStore) RemoveTasks(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	s.logger.Debug("RemoveTasks called", slog.String("component", "taskrunner"), slog.Int("count", len(ids)), slog.Any("ids", ids))
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = id.String()
	}
	if err := s.db.WithContext(ctx).Where("id IN ?", strIDs).Delete(&dbTaskExecution{}).Error; err != nil {
		return err
	}
	if s.cleaner != nil {
		if err := s.cleaner.RemoveTaskLogs(ctx, ids); err != nil {
			return err
		}
	}
	return nil
}

func (s *TaskExecutionStore) List(ctx context.Context) ([]tempo.TaskInfo, error) {
	var rows []dbTaskExecution
	if err := s.db.WithContext(ctx).Order("queued_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]tempo.TaskInfo, len(rows))
	for i := range rows {
		id, _ := uuid.Parse(rows[i].ID)
		out[i] = tempo.TaskInfo{
			ID:        id,
			Name:      rows[i].Name,
			Status:    tempo.TaskStatus(rows[i].Status),
			QueuedAt:  rows[i].QueuedAt,
			StartedAt: rows[i].StartedAt,
			EndedAt:   rows[i].EndedAt,
		}
	}
	return out, nil
}

var (
	_ tempo.TaskStatePersistence   = (*TaskExecutionStore)(nil)
	_ tempo.RecoverablePersistence = (*TaskExecutionStore)(nil)
)
