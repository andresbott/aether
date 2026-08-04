package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-bumbu/config"
)

type AppCfg struct {
	Server       serverCfg
	Obs          obsCfg `config:"Observability"`
	Env          Env
	DataDir      string
	Msgs         []Msg
	TaskRunner   TaskRunnerCfg
	ArtistImages ArtistImagesCfg
	Auth         AuthCfg
}

// Auth method values for AuthCfg.Method.
const (
	AuthMethodNone   = "none"   // no authentication required (current behavior)
	AuthMethodNative = "native" // native users stored in the aether DB
)

// AuthCfg selects how users authenticate and seeds the initial admin.
// AdminPassword may be plaintext or a bcrypt hash (recognized by its "$2"
// prefix); like every config value it can come from config.yaml, env vars, or
// an "@<path>" file reference. The admin is only created while the user store
// is empty (idempotent bootstrap), so changing these values later has no
// effect on an already-seeded store.
type AuthCfg struct {
	Method        string
	AdminUser     string
	AdminPassword string
}

// isBcryptHash reports whether s looks like a bcrypt hash ($2a$/$2b$/$2y$...).
func isBcryptHash(s string) bool {
	return strings.HasPrefix(s, "$2")
}

type TaskRunnerCfg struct {
	Parallelism    int
	QueueSize      int
	HistorySize    int
	LogDir         string
	TagReadWorkers int
}

type ArtistImagesCfg struct {
	FanartApiKey     string
	TheAudioDBApiKey string
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
	return listenAddr(c.BindIp, c.Port)
}

// obsCfg configures the observability server (health, Prometheus metrics).
// Enabled false means the server is never started at all.
type obsCfg struct {
	Enabled bool
	BindIp  string
	Port    int
}

func (c obsCfg) Addr() string {
	return listenAddr(c.BindIp, c.Port)
}

func listenAddr(bindIP string, port int) string {
	if bindIP == "" {
		return ":" + strconv.Itoa(port)
	}
	return bindIP + ":" + strconv.Itoa(port)
}

type Msg struct {
	Level string
	Msg   string
}

const EnvBarPrefix = "AETHER"

// configSearchPaths is where the config file is looked up when no -c flag is
// given, in order of precedence: the working directory (development, `make
// run`) then the packaged location used by the deb + systemd unit. Missing
// files are skipped; if none exist the built-in defaults apply.
var configSearchPaths = []string{
	"./config.yaml",
	"/etc/aether/config.yaml",
}

// resolveConfigFile decides which config file to load. An explicit flag value
// is mandatory — a typo must fail loudly rather than silently fall back to
// defaults. Without a flag the search paths are probed and the first existing
// one wins; an empty return means "no config file, use defaults only".
func resolveConfigFile(flagValue string, searchPaths []string) (path string, mandatory bool) {
	if flagValue != "" {
		return flagValue, true
	}
	for _, p := range searchPaths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, false
		}
	}
	return "", false
}

var defaultCfg = AppCfg{
	DataDir: "./data",
	Server: serverCfg{
		BindIp: "",
		Port:   8075,
	},
	Obs: obsCfg{
		Enabled: false,
		BindIp:  "",
		Port:    9009,
	},
	Env: Env{
		LogLevel:   "info",
		Production: false,
	},
	TaskRunner: TaskRunnerCfg{
		Parallelism:    2,
		QueueSize:      20,
		HistorySize:    50,
		TagReadWorkers: 0,
	},
	ArtistImages: ArtistImagesCfg{
		FanartApiKey:     "",
		TheAudioDBApiKey: "",
	},
	Auth: AuthCfg{
		Method:        AuthMethodNone,
		AdminUser:     "admin",
		AdminPassword: "admin",
	},
}

// getAppCfg loads the configuration. An empty file means "defaults + env only".
// mandatory makes a missing file an error — used when the path came from -c.
func getAppCfg(file string, mandatory bool) (AppCfg, error) {
	configMsg := []Msg{}
	cfg := AppCfg{}
	opts := []any{
		config.Defaults{Item: defaultCfg},
		config.EnvFile{Path: ".env", Mandatory: false},
	}
	if file != "" {
		opts = append(opts, config.CfgFile{Path: file, Mandatory: mandatory})
	}
	opts = append(opts,
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
	_, err := config.Load(opts...)
	cfg.Msgs = configMsg
	if err != nil {
		return cfg, err
	}

	absPath, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return cfg, fmt.Errorf("failed to get absolute path: %w", err)
	}
	cfg.DataDir = absPath

	// API keys may be loaded from files (config value "@<path>"); file content
	// is returned verbatim, so trim surrounding whitespace/newlines.
	cfg.ArtistImages.FanartApiKey = strings.TrimSpace(cfg.ArtistImages.FanartApiKey)
	cfg.ArtistImages.TheAudioDBApiKey = strings.TrimSpace(cfg.ArtistImages.TheAudioDBApiKey)

	cfg.Auth.Method = strings.ToLower(strings.TrimSpace(cfg.Auth.Method))
	if cfg.Auth.Method != AuthMethodNone && cfg.Auth.Method != AuthMethodNative {
		return cfg, fmt.Errorf("invalid auth method %q: must be %q or %q",
			cfg.Auth.Method, AuthMethodNone, AuthMethodNative)
	}
	cfg.Auth.AdminUser = strings.TrimSpace(cfg.Auth.AdminUser)
	cfg.Auth.AdminPassword = strings.TrimSpace(cfg.Auth.AdminPassword)
	if cfg.Auth.Method == AuthMethodNative {
		if cfg.Auth.AdminUser == "" {
			return cfg, fmt.Errorf("auth method %q requires AdminUser", AuthMethodNative)
		}
		if cfg.Auth.AdminPassword == "" {
			return cfg, fmt.Errorf("auth method %q requires AdminPassword", AuthMethodNative)
		}
	}

	return cfg, nil
}
