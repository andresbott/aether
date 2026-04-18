# Aether Bootstrap Design

Music server with Go backend, Vue.js frontend SPA, and stable API.

## Scope

Minimal skeleton: config loading, running HTTP server, one placeholder API endpoint (`GET /api/v1/health`), SPA serving with a "Hello Aether" Vue app. Just enough to prove end-to-end wiring works.

## Directory Structure

```
Aether/
├── app/
│   ├── cmd/           # Cobra CLI: root, server, version commands
│   ├── router/        # Gorilla Mux router, middleware, route attachment
│   │   └── handlers/  # HTTP handler funcs (health, admin/observability)
│   ├── spa/           # Embedded SPA files (go:embed)
│   │   └── files/ui/  # Vue build output copied here
│   └── metainfo/      # Build metadata (version, commit, build time)
│
├── internal/          # Empty for now — domain packages go here later
│
├── webui/             # Vue 3 SPA
│   └── src/
│       ├── router/    # Vue Router
│       ├── store/     # Pinia stores
│       ├── lib/api/   # Axios API client
│       ├── components/
│       ├── views/
│       └── assets/
│
├── main.go            # Entry point: cmd.Execute()
├── go.mod
├── config.yaml        # Runtime config (YAML)
├── .gitignore
└── Makefile
```

## Configuration

Config struct:

```go
type AppCfg struct {
    Server  serverCfg   // Port 8075, BindIp ""
    Obs     serverCfg   // Observability port 9009
    Env     Env         // LogLevel, Production flag
    DataDir string      // "./data"
}
```

Loading pipeline (using `go-bumbu/config`):

1. Hardcoded defaults
2. `.env` file (optional)
3. `config.yaml` (optional)
4. Environment variables with prefix `AETHER_`

Default ports: main server 8075, observability 9009.

## Go Backend

### Entry Point

`main.go` calls `cmd.Execute()`.

### Cobra CLI (`app/cmd/`)

- `root.go` — root command `aether`, registers subcommands: `start`, `version`
- `server.go` — `start` command, accepts `--config` flag, runs `runServer()`
- `config.go` — `AppCfg` struct, defaults, `getAppCfg()` loader
- `logger.go` — TTY-aware slog setup (console-slog for terminal, JSON for production)

### Server Startup Flow (`runServer`)

1. Load config
2. Setup structured logger
3. Create data directory (`./data/`)
4. Open SQLite database (WAL mode, busy timeout, max 10 open conns)
5. Create router (API + SPA)
6. Start main server (port 8075) and obs server (port 9009) concurrently via `errgroup`
7. Signal handler for graceful shutdown

### Router (`app/router/`)

- Gorilla Mux with `go-bumbu/http/middleware` (JSON errors, Prometheus histogram, logging)
- `GET /api/v1/health` — returns `{"status": "ok"}`
- `GET /*` — SPA catch-all serving embedded Vue build at root `/`

### Observability (`app/router/handlers/admin.go`)

- Separate HTTP server on port 9009
- `/metrics` — Prometheus handler
- `/` — links page

### SPA Embedding (`app/spa/`)

- `//go:embed files/ui/*` with `go-bumbu/http/handlers/spa`
- SPA served at root `/`

### Go Dependencies

- `github.com/gorilla/mux`
- `github.com/spf13/cobra`
- `github.com/go-bumbu/config`
- `github.com/go-bumbu/http` (middleware + SPA handler)
- `github.com/glebarez/sqlite`
- `gorm.io/gorm`
- `github.com/prometheus/client_golang`
- `golang.org/x/sync` (errgroup)
- `github.com/mattn/go-isatty`
- `github.com/phsym/console-slog`
- `github.com/samber/slog-formatter`

## Vue.js Frontend

### Dependencies

- `vue` ^3.5, `vue-router` ^4.5, `pinia` ^2.2
- `@tanstack/vue-query` ^5.71, `axios` ^1.7
- `primevue` ^4.2, `@primevue/themes` ^4.2, `primeflex` ^3.3
- `@tabler/icons-webfont` ^3.40, `zod` ^3.24
- TypeScript, Vite, vitest (dev deps)

### Structure

- `main.ts` — Vue app setup: PrimeVue, Pinia, Vue Router, TanStack Query, dark mode detection
- `App.vue` — minimal shell with router-view
- `router/index.ts` — single route: `/` → `HomeView.vue`
- `views/HomeView.vue` — "Aether" heading, calls `GET /api/v1/health` and displays result
- `lib/api/client.ts` — Axios instance with `baseURL: /api/v1`
- `store/` — empty Pinia store setup

### Vite Config

- Dev server proxy: `/api` → `http://localhost:8075`
- `base: "/"`
- `@` alias to `src/`

No auth guards, no echarts, no complex components.

## Makefile

```makefile
test          # go test ./...
ui-test       # cd webui && npm test
lint          # golangci-lint run
run           # AETHER_ENV_LOGLEVEL=debug go run main.go start
run-ui        # package-ui + run
build-ui      # cd webui && npm install && npm run build
package-ui    # build-ui, copy dist/ → app/spa/files/ui/
```

## .gitignore

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
