# Aether Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootstrap a working Go + Vue.js music server skeleton with config, HTTP server, health endpoint, SPA serving, and observability.

**Architecture:** Cobra CLI entry point → config loader (go-bumbu/config) → Gorilla Mux router with go-bumbu/http middleware → embedded Vue 3 SPA at root `/`, API at `/api/v1`, observability server on separate port. SQLite via GORM for database. Same patterns as etna-finance.

**Tech Stack:** Go 1.25+, Gorilla Mux, Cobra, GORM/SQLite, Prometheus, slog | Vue 3, TypeScript, PrimeVue, Pinia, TanStack Query, Axios, Vite

**Reference project:** `/home/odo/datos/edit/programacion/bumbu/etna-finance`

---

## File Map

### Go Backend

| File | Responsibility |
|------|---------------|
| `main.go` | Entry point: `cmd.Execute()` |
| `app/cmd/root.go` | Cobra root command, registers `start` and `version` |
| `app/cmd/config.go` | `AppCfg` struct, defaults, `getAppCfg()` loader |
| `app/cmd/logger.go` | TTY-aware slog logger setup |
| `app/cmd/server.go` | `start` command: config → DB → router → serve |
| `app/metainfo/meta.go` | Build metadata (version, commit, build time) |
| `app/router/main.go` | `MainAppHandler`, Gorilla Mux setup, SPA + API attachment |
| `app/router/api_v1.go` | API v1 subrouter with health endpoint |
| `app/router/handlers/admin.go` | Observability handler (`/metrics`) |
| `app/router/handlers/health.go` | Health check handler |
| `app/spa/spa.go` | `//go:embed` SPA files, handler factory |
| `app/spa/files/ui/.gitkeep` | Placeholder for embedded SPA build |

### Vue Frontend

| File | Responsibility |
|------|---------------|
| `webui/package.json` | Dependencies and scripts |
| `webui/tsconfig.json` | TypeScript config |
| `webui/vite.config.ts` | Vite config with dev proxy |
| `webui/index.html` | HTML entry point |
| `webui/.env` | Vite env vars (`VITE_SERVER_URL_V1`) |
| `webui/src/main.ts` | Vue app bootstrap (PrimeVue, Pinia, Router, Query) |
| `webui/src/App.vue` | Root component with router-view |
| `webui/src/router/index.ts` | Vue Router: single `/` route |
| `webui/src/lib/api/client.ts` | Axios instance, `baseURL: /api/v1` |
| `webui/src/views/HomeView.vue` | "Aether" heading + health check display |
| `webui/src/theme.js` | PrimeVue Lara theme preset |
| `webui/src/assets/style.css` | Minimal global styles |

### Root Files

| File | Responsibility |
|------|---------------|
| `go.mod` | Go module definition |
| `config.yaml` | Reference config |
| `.gitignore` | Ignore rules |
| `Makefile` | Build/run/test targets |

---

## Task 1: Go Module and Root Files

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `.gitignore`
- Create: `config.yaml`
- Create: `Makefile`

- [ ] **Step 1: Initialize go.mod**

```
go mod init github.com/andresbott/aether
```

This creates the `go.mod` with `module github.com/andresbott/aether`. The Go version will match whatever is installed.

- [ ] **Step 2: Create main.go**

Create `main.go`:

```go
package main

import (
	"github.com/andresbott/aether/app/cmd"
)

func main() {
	cmd.Execute()
}
```

- [ ] **Step 3: Create .gitignore**

Create `.gitignore`:

```
# Go
/data/
*.db
*.db-shm
*.db-wal

# Node
webui/node_modules/
webui/dist/

# IDE
.idea/

# Build
/dist/

# Env
.env

# SPA embedded files (rebuilt from webui/)
app/spa/files/ui/*
!app/spa/files/ui/.gitkeep
```

- [ ] **Step 4: Create config.yaml**

Create `config.yaml`:

```yaml
Server:
  Port: 8075
  BindIp: ""

Observability:
  Port: 9009
  BindIp: ""

DataDir: "./data"

Env:
  LogLevel: "info"
  Production: false
```

- [ ] **Step 5: Create Makefile**

Create `Makefile`:

```makefile
COMMIT_SHA_SHORT ?= $(shell git rev-parse --short=12 HEAD)
PWD_DIR := ${CURDIR}

default: help

#==========================================================================================
##@ Testing
#==========================================================================================
test: ## run fast go tests
	@go test ./... -cover

ui-test: ## run webui unit tests
	@cd webui && npm test

lint: ## run go linter
	@golangci-lint run

#==========================================================================================
##@ Running
#==========================================================================================
run: ## start the GO service with debug logging
	@AETHER_ENV_LOGLEVEL="debug" go run main.go start

run-ui: package-ui run ## build the UI and start the GO service

#==========================================================================================
##@ Building
#==========================================================================================
package-ui: build-ui ## build the web and copy into Go package
	rm -rf ./app/spa/files/ui*
	mkdir -p ./app/spa/files/ui
	cp -r ./webui/dist/* ./app/spa/files/ui/
	touch ./app/spa/files/ui/.gitkeep

build-ui: ## build the Vue.js frontend
	@cd webui && \
	npm install && \
	npm run build

clean: ## clean build artifacts
	@rm -rf dist

#==========================================================================================
#  Help
#==========================================================================================
.PHONY: help
help: # Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
```

- [ ] **Step 6: Commit**

```bash
git add main.go go.mod .gitignore config.yaml Makefile
git commit -m "bootstrap: add go module, main.go, config, makefile, gitignore"
```

---

## Task 2: Metainfo and Config

**Files:**
- Create: `app/metainfo/meta.go`
- Create: `app/cmd/config.go`
- Create: `app/cmd/logger.go`

- [ ] **Step 1: Create metainfo package**

Create `app/metainfo/meta.go`:

```go
package metainfo

import "time"

func init() {
	if BuildTime == "" {
		BuildTime = time.Now().Format(time.RFC3339)
	}
}

var Version = "dev-build"
var BuildTime = ""
var ShaVer = "undefined"
```

- [ ] **Step 2: Create config.go**

Create `app/cmd/config.go`:

```go
package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/go-bumbu/config"
)

type AppCfg struct {
	Server  serverCfg
	Obs     serverCfg `config:"Observability"`
	Env     Env
	DataDir string
	Msgs    []Msg
}

type Env struct {
	LogLevel   string
	Production bool
}

type serverCfg struct {
	BindIp string
	Port   int
}

func (c serverCfg) Addr() string {
	if c.BindIp == "" {
		return ":" + strconv.Itoa(c.Port)
	}
	return c.BindIp + ":" + strconv.Itoa(c.Port)
}

type Msg struct {
	Level string
	Msg   string
}

const EnvBarPrefix = "AETHER"

var defaultCfg = AppCfg{
	DataDir: "./data",
	Server: serverCfg{
		BindIp: "",
		Port:   8075,
	},
	Obs: serverCfg{
		BindIp: "",
		Port:   9009,
	},
	Env: Env{
		LogLevel:   "info",
		Production: false,
	},
}

func getAppCfg(file string) (AppCfg, error) {
	configMsg := []Msg{}
	cfg := AppCfg{}
	_, err := config.Load(
		config.Defaults{Item: defaultCfg},
		config.EnvFile{Path: ".env", Mandatory: false},
		config.CfgFile{Path: file, Mandatory: false},
		config.EnvVar{Prefix: EnvBarPrefix},
		config.Unmarshal{Item: &cfg},
		config.Writer{Fn: func(level, msg string) {
			if level == config.InfoLevel {
				configMsg = append(configMsg, Msg{Level: "info", Msg: msg})
			}
			if level == config.DebugLevel {
				configMsg = append(configMsg, Msg{Level: "debug", Msg: msg})
			}
		}},
	)
	cfg.Msgs = configMsg
	if err != nil {
		return cfg, err
	}

	absPath, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return cfg, fmt.Errorf("failed to get absolute path: %w", err)
	}
	cfg.DataDir = absPath

	return cfg, nil
}
```

- [ ] **Step 3: Create logger.go**

Create `app/cmd/logger.go`:

```go
package cmd

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/phsym/console-slog"
	slogformatter "github.com/samber/slog-formatter"
)

func GetLogLevel(in string) slog.Level {
	in = strings.ToUpper(in)
	switch in {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR", "ERR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func defaultLogger(level slog.Level) (*slog.Logger, error) {
	useTty := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	var defaultHandler slog.Handler
	if useTty {
		consoleHan := console.NewHandler(os.Stdout, &console.HandlerOptions{
			Level:      level,
			TimeFormat: time.Kitchen,
		})

		var fmts []slogformatter.Formatter
		errFmt := slogformatter.ErrorFormatter("error")
		fmts = append(fmts, errFmt)
		timeFmt := slogformatter.TimeFormatter(time.RFC3339, time.Now().Location())
		fmts = append(fmts, timeFmt)

		defaultHandler = slogformatter.NewFormatterHandler(fmts...)(consoleHan)
	} else {
		jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})

		var fmts []slogformatter.Formatter
		timeFmt := slogformatter.TimeFormatter(time.RFC3339, time.UTC)
		fmts = append(fmts, timeFmt)
		defaultHandler = slogformatter.NewFormatterHandler(fmts...)(jsonHandler)
	}
	logger := slog.New(defaultHandler)
	return logger, nil
}

func SilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}
```

- [ ] **Step 4: Get dependencies**

```bash
cd /home/odo/.datos/edit/programacion/bumbu/Aether && go get github.com/go-bumbu/config github.com/mattn/go-isatty github.com/phsym/console-slog github.com/samber/slog-formatter github.com/spf13/cobra
```

- [ ] **Step 5: Commit**

```bash
git add app/metainfo/ app/cmd/config.go app/cmd/logger.go go.mod go.sum
git commit -m "add metainfo, config loader, and logger"
```

---

## Task 3: Cobra CLI (root + version + server stub)

**Files:**
- Create: `app/cmd/root.go`
- Create: `app/cmd/server.go`

- [ ] **Step 1: Create root.go**

Create `app/cmd/root.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/andresbott/aether/app/metainfo"
	"github.com/spf13/cobra"
)

func Execute() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aether",
		Short: "aether: music server",
	}

	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		_ = cmd.Help()
		return nil
	})

	cmd.AddCommand(
		serverCmd(),
		versionCmd(),
	)

	return cmd
}

func versionCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "version",
		Short: "print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Version: %s\n", metainfo.Version)
			fmt.Printf("Build date: %s\n", metainfo.BuildTime)
			fmt.Printf("Commit sha: %s\n", metainfo.ShaVer)
			fmt.Printf("Compiler: %s\n", runtime.Version())
		},
	}
	return &cmd
}
```

- [ ] **Step 2: Create server.go**

Create `app/cmd/server.go`:

```go
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

	routerCfg := router.Cfg{
		Logger: l,
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
```

- [ ] **Step 3: Get remaining Go dependencies**

```bash
cd /home/odo/.datos/edit/programacion/bumbu/Aether && go get github.com/glebarez/sqlite gorm.io/gorm github.com/gorilla/mux github.com/prometheus/client_golang golang.org/x/sync github.com/go-bumbu/http
```

- [ ] **Step 4: Commit**

```bash
git add app/cmd/root.go app/cmd/server.go go.mod go.sum
git commit -m "add cobra CLI with start and version commands"
```

---

## Task 4: Router, Handlers, and SPA Embedding

**Files:**
- Create: `app/router/main.go`
- Create: `app/router/api_v1.go`
- Create: `app/router/handlers/admin.go`
- Create: `app/router/handlers/health.go`
- Create: `app/spa/spa.go`
- Create: `app/spa/files/ui/.gitkeep`

- [ ] **Step 1: Create the SPA embedding**

Create `app/spa/files/ui/.gitkeep` (empty file).

Create `app/spa/spa.go`:

```go
package spa

import (
	"embed"
	"net/http"

	handlers "github.com/go-bumbu/http/handlers/spa"
)

//go:embed files/ui/*
var UiFiles embed.FS

func App(path string) (http.Handler, error) {
	return handlers.NewSpaHAndler(
		UiFiles,
		"files/ui",
		path,
	)
}
```

- [ ] **Step 2: Create health handler**

Create `app/router/handlers/health.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
)

func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
}
```

- [ ] **Step 3: Create admin/observability handler**

Create `app/router/handlers/admin.go`:

```go
package handlers

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Admin() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		content := `<a href="/metrics">/metrics</a>`
		fmt.Fprint(w, content)
	})
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}
```

- [ ] **Step 4: Create API v1 subrouter**

Create `app/router/api_v1.go`:

```go
package router

import (
	"net/http"

	"github.com/andresbott/aether/app/router/handlers"
	"github.com/gorilla/mux"
)

func (h *MainAppHandler) attachApiV1(r *mux.Router) {
	r.Path("/health").Methods(http.MethodGet).Handler(handlers.HealthHandler())

	r.PathPrefix("").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "wrong api call", http.StatusBadRequest)
	})
}
```

- [ ] **Step 5: Create main router**

Create `app/router/main.go`:

```go
package router

import (
	"log/slog"
	"net/http"

	"github.com/andresbott/aether/app/spa"
	"github.com/go-bumbu/http/middleware"
	"github.com/gorilla/mux"
)

type Cfg struct {
	Logger *slog.Logger
}

type MainAppHandler struct {
	router *mux.Router
	logger *slog.Logger
}

func (h *MainAppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func New(cfg Cfg) (*MainAppHandler, error) {
	r := mux.NewRouter()
	app := MainAppHandler{
		router: r,
		logger: cfg.Logger,
	}

	prodMid := middleware.New(middleware.Cfg{
		JsonErrors:  true,
		GenericErrs: false,
		Logger:      cfg.Logger,
		PromHisto:   middleware.NewPromHistogram("", nil, nil),
	})
	r.Use(prodMid.Middleware)

	app.attachApiV1(app.router.PathPrefix("/api/v1").Subrouter())

	if err := app.attachSpa(app.router.PathPrefix("/").Subrouter(), "/"); err != nil {
		return nil, err
	}

	return &app, nil
}

func (h *MainAppHandler) attachSpa(r *mux.Router, path string) error {
	spaHandler, err := spa.App(path)
	if err != nil {
		return err
	}
	r.Methods(http.MethodGet).PathPrefix(path).Handler(spaHandler)
	return nil
}
```

- [ ] **Step 6: Verify Go compiles**

```bash
cd /home/odo/.datos/edit/programacion/bumbu/Aether && go build ./...
```

Expected: builds without errors. If there are missing dependencies, run `go mod tidy`.

- [ ] **Step 7: Commit**

```bash
git add app/router/ app/spa/ go.mod go.sum
git commit -m "add router with health endpoint, SPA embedding, and observability"
```

---

## Task 5: Verify Backend Runs

- [ ] **Step 1: Run the server**

```bash
cd /home/odo/.datos/edit/programacion/bumbu/Aether && go run main.go start &
sleep 2
```

- [ ] **Step 2: Test health endpoint**

```bash
curl -s http://localhost:8075/api/v1/health
```

Expected: `{"status":"ok"}`

- [ ] **Step 3: Test observability endpoint**

```bash
curl -s http://localhost:9009/metrics | head -5
```

Expected: Prometheus metrics output.

- [ ] **Step 4: Test version command**

```bash
cd /home/odo/.datos/edit/programacion/bumbu/Aether && go run main.go version
```

Expected output like:
```
Version: dev-build
Build date: 2026-04-18T...
Commit sha: undefined
Compiler: go1.25.4
```

- [ ] **Step 5: Stop the server**

Kill the background server process.

---

## Task 6: Vue.js Frontend Setup

**Files:**
- Create: `webui/package.json`
- Create: `webui/tsconfig.json`
- Create: `webui/vite.config.ts`
- Create: `webui/index.html`
- Create: `webui/.env`

- [ ] **Step 1: Create package.json**

Create `webui/package.json`:

```json
{
  "name": "aether-ui",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview",
    "test": "vue-tsc --noEmit && vitest run",
    "test:watch": "vitest",
    "type-check": "npx vue-tsc --noEmit",
    "format": "prettier --write src/"
  },
  "dependencies": {
    "@primevue/themes": "^4.2.4",
    "@tabler/icons-webfont": "^3.40.0",
    "@tanstack/vue-query": "^5.71.5",
    "axios": "^1.7.8",
    "pinia": "^2.2.8",
    "primeflex": "^3.3.1",
    "primevue": "^4.2.5",
    "vue": "^3.5.13",
    "vue-router": "^4.5.0",
    "zod": "^3.24.1"
  },
  "devDependencies": {
    "@types/node": "^24.4.0",
    "@vitejs/plugin-vue": "^5.2.1",
    "@vitest/coverage-v8": "^3.2.4",
    "@vue/test-utils": "^2.4.6",
    "jsdom": "^25.0.1",
    "prettier": "^3.4.1",
    "sass": "^1.82.0",
    "typescript": "^5.9.2",
    "vite": "^6.0.2",
    "vitest": "^3.2.4",
    "vue-tsc": "^3.0.6"
  }
}
```

- [ ] **Step 2: Create tsconfig.json**

Create `webui/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "Node",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "esModuleInterop": true,
    "lib": ["ESNext", "DOM"],
    "types": ["node"],
    "skipLibCheck": true,
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src/**/*.ts", "src/**/*.d.ts", "src/**/*.vue"],
  "exclude": ["node_modules"]
}
```

- [ ] **Step 3: Create vite.config.ts**

Create `webui/vite.config.ts`:

```typescript
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  base: '/',
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8075',
        changeOrigin: true,
        secure: false,
        cookieDomainRewrite: { '*': '' }
      }
    }
  }
})
```

- [ ] **Step 4: Create index.html**

Create `webui/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Aether</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 5: Create .env**

Create `webui/.env`:

```
VITE_SERVER_URL_V1=/api/v1
```

- [ ] **Step 6: Install npm dependencies**

```bash
cd /home/odo/.datos/edit/programacion/bumbu/Aether/webui && npm install
```

- [ ] **Step 7: Commit**

```bash
git add webui/package.json webui/package-lock.json webui/tsconfig.json webui/vite.config.ts webui/index.html webui/.env
git commit -m "add Vue.js frontend scaffolding with Vite, PrimeVue, and dev proxy"
```

---

## Task 7: Vue.js Application Code

**Files:**
- Create: `webui/src/main.ts`
- Create: `webui/src/App.vue`
- Create: `webui/src/theme.js`
- Create: `webui/src/assets/style.css`
- Create: `webui/src/router/index.ts`
- Create: `webui/src/lib/api/client.ts`
- Create: `webui/src/views/HomeView.vue`

- [ ] **Step 1: Create theme.js**

Create `webui/src/theme.js`:

```javascript
import { definePreset } from '@primevue/themes'
import Lara from '@primevue/themes/lara'

const CustomTheme = definePreset(Lara, {})

export default CustomTheme
```

- [ ] **Step 2: Create style.css**

Create `webui/src/assets/style.css`:

```css
body {
  margin: 0;
  font-family: var(--font-family);
}

#app {
  min-height: 100vh;
}
```

- [ ] **Step 3: Create API client**

Create `webui/src/lib/api/client.ts`:

```typescript
import axios from 'axios'

const API_BASE_URL = import.meta.env.VITE_SERVER_URL_V1

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json'
  }
})
```

- [ ] **Step 4: Create Vue Router**

Create `webui/src/router/index.ts`:

```typescript
import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory('/'),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('@/views/HomeView.vue')
    }
  ]
})

export default router
```

- [ ] **Step 5: Create HomeView**

Create `webui/src/views/HomeView.vue`:

```vue
<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { apiClient } from '@/lib/api/client'

const { data, isLoading, isError } = useQuery({
  queryKey: ['health'],
  queryFn: async () => {
    const response = await apiClient.get('/health')
    return response.data
  }
})
</script>

<template>
  <div class="p-4">
    <h1>Aether</h1>
    <p>Music Server</p>
    <div class="mt-4">
      <h3>API Health</h3>
      <p v-if="isLoading">Checking...</p>
      <p v-else-if="isError" class="text-red-500">API unreachable</p>
      <p v-else class="text-green-500">{{ data?.status }}</p>
    </div>
  </div>
</template>
```

- [ ] **Step 6: Create App.vue**

Create `webui/src/App.vue`:

```vue
<script setup lang="ts">
</script>

<template>
  <router-view />
</template>
```

- [ ] **Step 7: Create main.ts**

Create `webui/src/main.ts`:

```typescript
import { createApp } from 'vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import App from './App.vue'
// @ts-expect-error theme.js has no type declarations
import CustomTheme from '@/theme.js'

import 'primeflex/primeflex.css'
import '@tabler/icons-webfont/dist/tabler-icons-300.min.css'
import '@/assets/style.css'

import PrimeVue from 'primevue/config'

const app = createApp(App)

const applyTheme = () => {
  const darkModeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  const htmlElement = document.documentElement

  if (darkModeMediaQuery.matches) {
    htmlElement.classList.add('dark-mode')
  } else {
    htmlElement.classList.remove('dark-mode')
  }
}

applyTheme()
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', applyTheme)

app.use(PrimeVue, {
  theme: {
    preset: CustomTheme,
    options: {
      prefix: 'c',
      darkModeSelector: '.dark-mode',
      cssLayer: false
    }
  }
})

import { createPinia } from 'pinia'
app.use(createPinia())

import router from './router'
app.use(router)

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 3,
      staleTime: 1000 * 60 * 5,
      gcTime: 1000 * 60 * 30
    },
    mutations: {
      retry: false
    }
  }
})

app.use(VueQueryPlugin, { queryClient })

app.mount('#app')
```

- [ ] **Step 8: Verify frontend builds**

```bash
cd /home/odo/.datos/edit/programacion/bumbu/Aether/webui && npm run build
```

Expected: build succeeds, output in `webui/dist/`.

- [ ] **Step 9: Commit**

```bash
git add webui/src/
git commit -m "add Vue.js app with PrimeVue, router, health check view"
```

---

## Task 8: End-to-End Verification

- [ ] **Step 1: Package UI into Go binary**

```bash
cd /home/odo/.datos/edit/programacion/bumbu/Aether && make package-ui
```

Expected: `app/spa/files/ui/` contains `index.html` and `assets/` directory.

- [ ] **Step 2: Start the full server**

```bash
cd /home/odo/.datos/edit/programacion/bumbu/Aether && go run main.go start &
sleep 2
```

- [ ] **Step 3: Test health endpoint**

```bash
curl -s http://localhost:8075/api/v1/health
```

Expected: `{"status":"ok"}`

- [ ] **Step 4: Test SPA serving**

```bash
curl -s http://localhost:8075/ | head -5
```

Expected: HTML containing `<div id="app">` and `<title>Aether</title>`.

- [ ] **Step 5: Test observability**

```bash
curl -s http://localhost:9009/metrics | head -5
```

Expected: Prometheus metrics output.

- [ ] **Step 6: Open in browser**

Open `http://localhost:8075/` in a browser. Verify:
- "Aether" heading is visible
- "Music Server" text is visible
- API Health shows "ok" (green)

- [ ] **Step 7: Stop the server**

Kill the background server process.

- [ ] **Step 8: Final commit if any changes were needed**

If any fixes were required during verification, commit them:

```bash
git add -A
git commit -m "fix: adjustments from end-to-end verification"
```
