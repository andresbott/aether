package cmd

import (
	"fmt"
	"net/netip"
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
	AuthMethodNone        = "none"         // no authentication required (current behavior)
	AuthMethodNative      = "native"       // native users stored in the aether DB
	AuthMethodProxyHeader = "proxy-header" // identity headers from a trusted reverse proxy (e.g. Authelia)
)

// AuthCfg selects how users authenticate and seeds the initial admin.
// AdminBootstrap applies to method "native" only; ProxyHeader applies to
// method "proxy-header" only.
type AuthCfg struct {
	Method         string
	AdminBootstrap AdminBootstrapCfg
	ProxyHeader    ProxyHeaderCfg
}

// AdminBootstrapCfg seeds the initial admin user for method "native". Pw may
// be plaintext or a bcrypt hash (recognized by its "$2" prefix); like every
// config value it can come from config.yaml, env vars, or an "@<path>" file
// reference. The admin is only created while the user store is empty
// (idempotent bootstrap), so changing these values later has no effect on an
// already-seeded store.
type AdminBootstrapCfg struct {
	User string
	Pw   string
}

// ProxyHeaderCfg configures the proxy-header auth method: which injected
// headers carry identity, which group grants admin, and which peers are
// allowed to assert identity headers at all. Users are provisioned on first
// sight of a new identity; roles are read live from the groups header, never
// from the DB (the proxy's identity provider is authoritative).
type ProxyHeaderCfg struct {
	// UserHeader carries the authenticated login (Authelia: Remote-User,
	// oauth2-proxy: X-Forwarded-User).
	UserHeader string
	// GroupsHeader carries the comma-separated group list.
	GroupsHeader string
	// AdminGroup is the group whose membership grants the admin role.
	AdminGroup string
	// TrustedProxies are CIDR prefixes allowed to assert identity headers.
	// Empty means every peer is trusted — the deployment alone must guarantee
	// the proxy is the only path in (a loud startup warning reminds of this).
	TrustedProxies []string
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
		Method: AuthMethodNone,
		AdminBootstrap: AdminBootstrapCfg{
			User: "admin",
			Pw:   "admin",
		},
		ProxyHeader: ProxyHeaderCfg{
			UserHeader:   "Remote-User",
			GroupsHeader: "Remote-Groups",
			AdminGroup:   "aether-admin",
		},
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
	if cfg.Auth.Method != AuthMethodNone && cfg.Auth.Method != AuthMethodNative &&
		cfg.Auth.Method != AuthMethodProxyHeader {
		return cfg, fmt.Errorf("invalid auth method %q: must be %q, %q or %q",
			cfg.Auth.Method, AuthMethodNone, AuthMethodNative, AuthMethodProxyHeader)
	}
	cfg.Auth.AdminBootstrap.User = strings.TrimSpace(cfg.Auth.AdminBootstrap.User)
	cfg.Auth.AdminBootstrap.Pw = strings.TrimSpace(cfg.Auth.AdminBootstrap.Pw)
	if cfg.Auth.Method == AuthMethodNative {
		if cfg.Auth.AdminBootstrap.User == "" {
			return cfg, fmt.Errorf("auth method %q requires AdminBootstrap.User", AuthMethodNative)
		}
		if cfg.Auth.AdminBootstrap.Pw == "" {
			return cfg, fmt.Errorf("auth method %q requires AdminBootstrap.Pw", AuthMethodNative)
		}
	}
	if cfg.Auth.Method == AuthMethodProxyHeader {
		ph := &cfg.Auth.ProxyHeader
		ph.UserHeader = strings.TrimSpace(ph.UserHeader)
		ph.GroupsHeader = strings.TrimSpace(ph.GroupsHeader)
		ph.AdminGroup = strings.TrimSpace(ph.AdminGroup)
		if ph.UserHeader == "" {
			return cfg, fmt.Errorf("auth method %q requires ProxyHeader.UserHeader", AuthMethodProxyHeader)
		}
		if ph.AdminGroup == "" {
			return cfg, fmt.Errorf("auth method %q requires ProxyHeader.AdminGroup", AuthMethodProxyHeader)
		}
		if _, err := parseTrustedProxies(ph.TrustedProxies); err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

// parseTrustedProxies parses the configured CIDR list. A bare IP is accepted
// as a single-address prefix (192.168.1.5 == 192.168.1.5/32).
func parseTrustedProxies(cidrs []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if !strings.Contains(c, "/") {
			addr, err := netip.ParseAddr(c)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy %q: %w", c, err)
			}
			out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: %w", c, err)
		}
		out = append(out, p)
	}
	return out, nil
}
