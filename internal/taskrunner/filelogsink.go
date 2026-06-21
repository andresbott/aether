package taskrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-bumbu/tempo"
	"github.com/google/uuid"
)

type TaskLogGetter interface {
	GetTaskLog(ctx context.Context, executionID uuid.UUID) (string, error)
}

func NewFileTaskLogReader(dir string) TaskLogGetter {
	return &fileTaskLogReader{dir: dir}
}

type fileTaskLogReader struct {
	dir string
}

func (f *fileTaskLogReader) GetTaskLog(ctx context.Context, executionID uuid.UUID) (string, error) {
	path := filepath.Join(f.dir, executionID.String()+".log")
	b, err := os.ReadFile(path) //nolint:gosec // path constructed from trusted uuid
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

type TaskLogCleaner interface {
	RemoveTaskLogs(ctx context.Context, ids []uuid.UUID) error
}

type FileTaskLogSink struct {
	dir string
	mu  sync.Mutex
}

func NewFileTaskLogSink(dir string) (*FileTaskLogSink, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("task log dir: %w", err)
	}
	return &FileTaskLogSink{dir: dir}, nil
}

func (f *FileTaskLogSink) Append(ctx context.Context, taskID uuid.UUID, level string, msg string) (err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := filepath.Join(f.dir, taskID.String()+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644) //nolint:gosec // path from trusted uuid, 0644 intentional for log files
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	line := fmt.Sprintf("%s %s %s\n", time.Now().UTC().Format(time.RFC3339Nano), level, msg)
	_, err = file.WriteString(line)
	return err
}

func (f *FileTaskLogSink) RemoveTaskLogs(ctx context.Context, ids []uuid.UUID) error {
	for _, id := range ids {
		path := filepath.Join(f.dir, id.String()+".log")
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

var (
	_ tempo.TaskLogSink = (*FileTaskLogSink)(nil)
	_ TaskLogCleaner    = (*FileTaskLogSink)(nil)
)
