package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/andresbott/aether/app/metainfo"
	"github.com/andresbott/aether/app/router"
	"github.com/andresbott/aether/app/router/handlers"
	"github.com/andresbott/aether/app/tasks"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const dbFile = "aether.db"

func serverCmd() *cobra.Command {
	var configFile = "./config.yaml"
	cmd := &cobra.Command{
		Use:   "start",
		Short: "start the aether server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(configFile)
		},
	}
	cmd.Flags().StringVarP(&configFile, "config", "c", configFile, "config file")
	return cmd
}

func runServer(configFile string) error {
	cfg, err := getAppCfg(configFile)
	if err != nil {
		return err
	}

	l, err := defaultLogger(GetLogLevel(cfg.Env.LogLevel))
	if err != nil {
		return err
	}

	l.Info("App startup",
		slog.String("component", "startup"),
		slog.String("version", metainfo.Version),
		slog.String("Build Date", metainfo.BuildTime),
		slog.String("commit", metainfo.ShaVer),
	)
	for _, m := range cfg.Msgs {
		if m.Level == "info" {
			l.Info(m.Msg, slog.String("component", "config"))
		} else {
			l.Debug(m.Msg, slog.String("component", "config"))
		}
	}

	err = initDataDir(cfg.DataDir)
	if err != nil {
		return err
	}
	l.Info("using data directory", slog.String("path", cfg.DataDir))

	gormLog := gormlogger.NewSlogLogger(
		l.With(slog.String("component", "gorm")),
		gormlogger.Config{
			IgnoreRecordNotFoundError: true,
			LogLevel:                  gormlogger.Warn,
		},
	)
	db, err := gorm.Open(sqlite.Open(filepath.Join(cfg.DataDir, dbFile)), &gorm.Config{
		Logger: gormLog,
	})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(10)
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	// Migrate domain models
	if err := model.Migrate(db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	dataStore := store.New(db)

	scanCfg := scanner.Config{TagReadWorkers: cfg.TaskRunner.TagReadWorkers}

	// Task runner
	logDir := cfg.TaskRunner.LogDir
	if logDir == "" {
		logDir = filepath.Join(cfg.DataDir, "task-logs")
	}
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{
		Parallelism: cfg.TaskRunner.Parallelism,
		QueueSize:   cfg.TaskRunner.QueueSize,
		HistorySize: cfg.TaskRunner.HistorySize,
		Logger:      l,
		DB:          db,
		LogDir:      logDir,
		LogLevel:    GetLogLevel(cfg.Env.LogLevel),
	})
	if err != nil {
		return fmt.Errorf("task runner: %w", err)
	}

	// Tag reader
	tagReader := tags.NewFallbackReader(tags.TaglibReader{}, tags.FFProbeReader{})

	// Register tasks
	runner.RegisterTask(tasks.NewScanTaskFn(scanCfg, dataStore, tagReader, l, false), tasks.ScanTaskName, 1)
	runner.RegisterTask(tasks.NewScanTaskFn(scanCfg, dataStore, tagReader, l, true), tasks.ScanFullTaskName, 1)
	runner.Start()

	taskLogReader := taskrunner.NewFileTaskLogReader(logDir)

	routerCfg := router.Cfg{
		Logger:        l,
		TaskRunner:    runner,
		TaskLogGetter: taskLogReader,
		Store:         dataStore,
	}
	mainAppHandler, err := router.New(routerCfg)
	if err != nil {
		return fmt.Errorf("unable to initialize main app handler: %v", err)
	}

	mainSrv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           mainAppHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	obsSrv := &http.Server{
		Addr:              cfg.Obs.Addr(),
		Handler:           handlers.Admin(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	g, gctx := errgroup.WithContext(rootCtx)
	g.Go(func() error { return serveHTTP(gctx, mainSrv, l, "server") })
	g.Go(func() error { return serveHTTP(gctx, obsSrv, l, "observability") })
	g.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return runner.Shutdown(shutdownCtx)
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		rootCancel()
	}()

	return g.Wait()
}

func serveHTTP(ctx context.Context, srv *http.Server, l *slog.Logger, component string) error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("%s listen: %w", component, err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	l.Info(component+" server started", slog.String("component", component), slog.String("addr", srv.Addr))
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		l.Warn(component+" server shutdown error", slog.String("component", component), slog.String("error", err.Error()))
	}
	l.Info(component+" server stopped", slog.String("component", component))
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}


func initDataDir(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(absPath, 0750); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	return nil
}
