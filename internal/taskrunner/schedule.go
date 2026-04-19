package taskrunner

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Schedule struct {
	ID             uint      `json:"id"`
	TaskName       string    `json:"task_name"`
	CronExpression string    `json:"cron_expression"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type dbSchedule struct {
	ID             uint   `gorm:"primaryKey"`
	TaskName       string `gorm:"uniqueIndex:idx_task_schedule_task;not null"`
	CronExpression string `gorm:"not null"`
	Enabled        bool   `gorm:"default:true"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (dbSchedule) TableName() string { return "task_schedules" }

func dbToSchedule(d dbSchedule) Schedule {
	return Schedule{
		ID:             d.ID,
		TaskName:       d.TaskName,
		CronExpression: d.CronExpression,
		Enabled:        d.Enabled,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

var ErrScheduleNotFound = errors.New("schedule not found")

type ScheduleStore struct {
	db *gorm.DB
}

func NewScheduleStore(db *gorm.DB) (*ScheduleStore, error) {
	if db == nil {
		return nil, errors.New("db cannot be nil")
	}
	if err := db.AutoMigrate(&dbSchedule{}); err != nil {
		return nil, err
	}
	return &ScheduleStore{db: db}, nil
}

func (s *ScheduleStore) List(ctx context.Context) ([]Schedule, error) {
	var list []dbSchedule
	if err := s.db.WithContext(ctx).Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]Schedule, len(list))
	for i := range list {
		out[i] = dbToSchedule(list[i])
	}
	return out, nil
}

func (s *ScheduleStore) ListEnabled(ctx context.Context) ([]Schedule, error) {
	var list []dbSchedule
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]Schedule, len(list))
	for i := range list {
		out[i] = dbToSchedule(list[i])
	}
	return out, nil
}

func (s *ScheduleStore) GetByTaskName(ctx context.Context, taskName string) (Schedule, error) {
	var d dbSchedule
	err := s.db.WithContext(ctx).Where("task_name = ?", taskName).First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Schedule{}, ErrScheduleNotFound
		}
		return Schedule{}, err
	}
	return dbToSchedule(d), nil
}

func (s *ScheduleStore) GetByID(ctx context.Context, id uint) (Schedule, error) {
	var d dbSchedule
	err := s.db.WithContext(ctx).First(&d, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Schedule{}, ErrScheduleNotFound
		}
		return Schedule{}, err
	}
	return dbToSchedule(d), nil
}

func (s *ScheduleStore) Create(ctx context.Context, sch Schedule) (Schedule, error) {
	d := dbSchedule{
		TaskName:       sch.TaskName,
		CronExpression: sch.CronExpression,
		Enabled:        sch.Enabled,
	}
	if err := s.db.WithContext(ctx).Create(&d).Error; err != nil {
		return Schedule{}, err
	}
	return dbToSchedule(d), nil
}

func (s *ScheduleStore) Update(ctx context.Context, sch Schedule) error {
	if sch.ID == 0 {
		return errors.New("schedule id is required for update")
	}
	return s.db.WithContext(ctx).Model(&dbSchedule{}).Where("id = ?", sch.ID).
		Updates(map[string]interface{}{
			"cron_expression": sch.CronExpression,
			"enabled":         sch.Enabled,
		}).Error
}

func (s *ScheduleStore) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&dbSchedule{}, id).Error
}

func (s *ScheduleStore) DeleteByTaskName(ctx context.Context, taskName string) error {
	res := s.db.WithContext(ctx).Where("task_name = ?", taskName).Delete(&dbSchedule{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrScheduleNotFound
	}
	return nil
}

func (s *ScheduleStore) UpsertByTaskName(ctx context.Context, taskName, cronExpression string, enabled bool) (Schedule, error) {
	var d dbSchedule
	err := s.db.WithContext(ctx).Unscoped().Where("task_name = ?", taskName).First(&d).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return Schedule{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		d.TaskName = taskName
		d.CronExpression = cronExpression
		d.Enabled = enabled
		if err := s.db.WithContext(ctx).Create(&d).Error; err != nil {
			return Schedule{}, err
		}
		return dbToSchedule(d), nil
	}
	d.CronExpression = cronExpression
	d.Enabled = enabled
	d.DeletedAt = gorm.DeletedAt{}
	if err := s.db.WithContext(ctx).Unscoped().Save(&d).Error; err != nil {
		return Schedule{}, err
	}
	return dbToSchedule(d), nil
}
