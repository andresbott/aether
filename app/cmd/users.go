package cmd

import (
	"fmt"
	"log/slog"

	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/go-bumbu/userauth/auth/cookieauth"
	"github.com/go-bumbu/userauth/auth/headerauth"
	loginflow "github.com/go-bumbu/userauth/flow/login"
	"github.com/go-bumbu/userauth/service/password"
	"github.com/go-bumbu/userauth/service/pat"
	patdb "github.com/go-bumbu/userauth/service/pat/store/db"
	throttledb "github.com/go-bumbu/userauth/service/throttle/store/db"
	"github.com/go-bumbu/userauth/service/user"
	userdb "github.com/go-bumbu/userauth/service/user/store/db"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// bcryptDifficulty is the shared cost for all password hashes aether creates
// (admin bootstrap, CLI hash/reset, users API). It is also the password
// service's cost, so a hash it upgrades on login lands at the same value.
const bcryptDifficulty = usersHandler.BcryptDifficulty

// newUserStore builds the native identity service on the shared aether DB.
// purgers are the satellite stores Delete cascades into (e.g. the PAT store),
// so removing a user removes what is keyed on their canonical ID; the CLI,
// which never deletes, passes none.
func newUserStore(db *gorm.DB, l *slog.Logger, purgers ...user.Purger) (*user.Service, error) {
	store, err := userdb.New(db)
	if err != nil {
		return nil, err
	}
	return user.NewService(store, user.Opts{
		DefaultEnabled: true,
		OnDelete:       purgers,
		Logger:         l,
	})
}

// bootstrapAdmin seeds the initial admin from config. It relies on
// Service.Bootstrap being idempotent: the admin is only created while the store
// has no users at all, so it is safe to call on every startup and stale config
// never resurrects deleted accounts. The configured password may be a bcrypt
// hash (e.g. produced by `aether user hash`) instead of plaintext; a plaintext
// value is hashed here, since the identity service only stores hashes.
func bootstrapAdmin(users *user.Service, cfg AuthCfg) (bool, error) {
	if cfg.AdminBootstrap.User == "" {
		// Nothing to seed: an operator who configured no admin gets an empty
		// store rather than a boot failure on an empty login ID.
		return false, nil
	}
	hash := cfg.AdminBootstrap.Pw
	if !isBcryptHash(hash) {
		h, err := bcrypt.GenerateFromPassword([]byte(hash), bcryptDifficulty)
		if err != nil {
			return false, fmt.Errorf("hash admin password: %w", err)
		}
		hash = string(h)
	}
	enabled := true
	seeded, err := users.Bootstrap(user.Draft{
		LoginID:      cfg.AdminBootstrap.User,
		PasswordHash: hash,
		Enabled:      &enabled,
		Groups:       []string{usersHandler.AdminGroup},
	})
	if err != nil {
		return false, fmt.Errorf("bootstrap admin user: %w", err)
	}
	return seeded, nil
}

// setupNativeAuth creates the identity and password services, seeds the initial
// admin and builds the cookie session manager when the auth method is native,
// returning them for the router (users CRUD, login endpoints, /api/v1 session
// guard). With auth method "none" it returns nils — nothing auth-related is
// created at all.
func setupNativeAuth(db *gorm.DB, dataDir string, cfg AuthCfg, l *slog.Logger) (*user.Service, *password.Service, *cookieauth.Manager, *pat.Service, error) {
	if cfg.Method != AuthMethodNative {
		return nil, nil, nil, nil, nil
	}
	patStore, err := patdb.New(db)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("token store: %w", err)
	}
	users, err := newUserStore(db, l, patStore)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("user store: %w", err)
	}
	passwords, err := password.NewService(password.Opts{Cost: bcryptDifficulty, Rehash: users, Logger: l})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("password service: %w", err)
	}
	seeded, err := bootstrapAdmin(users, cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if seeded {
		l.Info("seeded initial admin user",
			slog.String("component", "startup"), slog.String("user", cfg.AdminBootstrap.User))
	}
	sessions, err := newSessionManager(dataDir, l)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("session manager: %w", err)
	}
	tokens, err := newTokenService(patStore, users, dataDir, l)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return users, passwords, sessions, tokens, nil
}

// authDeps is everything the router needs to enforce the configured auth
// method. Exactly one mode's fields are populated (or none, for method "none"):
// Sessions and Passwords are native-only, HeaderAuth proxy-only, and
// Users/Tokens are the shared halves both authenticated modes build.
type authDeps struct {
	Users      *user.Service
	Passwords  *password.Service
	Sessions   *cookieauth.Manager
	Tokens     *pat.Service
	HeaderAuth *headerauth.HeaderHandler
	LoginGuard loginflow.Guard
}

// setupAuth builds the auth dependencies for whichever method is configured.
// Both setups are consulted because each is a no-op outside its own mode; the
// method then decides which one's stores win.
func setupAuth(db *gorm.DB, dataDir string, cfg AuthCfg, l *slog.Logger) (authDeps, error) {
	users, passwords, sessions, tokens, err := setupNativeAuth(db, dataDir, cfg, l)
	if err != nil {
		return authDeps{}, err
	}
	proxyUsers, proxyTokens, headerAuth, err := setupProxyAuth(db, dataDir, cfg, l)
	if err != nil {
		return authDeps{}, err
	}
	if cfg.Method == AuthMethodProxyHeader {
		users, tokens = proxyUsers, proxyTokens
	}
	deps := authDeps{Users: users, Passwords: passwords, Sessions: sessions, Tokens: tokens, HeaderAuth: headerAuth}
	// Brute-force backoff applies to the native login endpoint only: proxy-header
	// delegates login to the IdP and "none" has no login to guard.
	if cfg.Method == AuthMethodNative && cfg.LoginThrottle.enabled() {
		guard, err := newLoginThrottle(db)
		if err != nil {
			return authDeps{}, err
		}
		deps.LoginGuard = guard
	}
	return deps, nil
}

// newLoginThrottle builds the per-account login backoff on the shared aether DB
// (persistent so the state survives restarts and cannot be reset by crashing
// the server). It uses the library's escalating-delay defaults.
func newLoginThrottle(db *gorm.DB) (loginflow.Guard, error) {
	store, err := throttledb.New(db)
	if err != nil {
		return nil, fmt.Errorf("login throttle store: %w", err)
	}
	return loginflow.ThrottleGuard{Throttle: &loginflow.Throttle{Store: store}}, nil
}

// newTokenService builds the PAT service on the shared token store — the same
// token layer in every authenticated mode (docs/agents/authentication.md). The
// cipher enables recoverable (user+token) PATs; hash-only apikey PATs work
// without it.
func newTokenService(patStore *patdb.Store, users *user.Service, dataDir string, l *slog.Logger) (*pat.Service, error) {
	cipher, err := loadPATCipher(dataDir)
	if err != nil {
		return nil, fmt.Errorf("pat cipher: %w", err)
	}
	tokens, err := pat.NewService(patStore, users, pat.Opts{Prefix: "aether", Cipher: cipher, Logger: l})
	if err != nil {
		return nil, fmt.Errorf("token service: %w", err)
	}
	return tokens, nil
}

// setupProxyAuth creates the pieces for the proxy-header mode: the same identity
// service and PAT service as native (users are provisioned on first sight of a
// new identity, so no admin bootstrap and no password service) plus the header
// handler that validates proxy-injected identity. No session manager — the
// proxy owns the session.
func setupProxyAuth(db *gorm.DB, dataDir string, cfg AuthCfg, l *slog.Logger) (*user.Service, *pat.Service, *headerauth.HeaderHandler, error) {
	if cfg.Method != AuthMethodProxyHeader {
		return nil, nil, nil, nil
	}
	patStore, err := patdb.New(db)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("token store: %w", err)
	}
	users, err := newUserStore(db, l, patStore)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("user store: %w", err)
	}
	tokens, err := newTokenService(patStore, users, dataDir, l)
	if err != nil {
		return nil, nil, nil, err
	}
	trusted, err := parseTrustedProxies(cfg.ProxyHeader.TrustedProxies)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(trusted) == 0 {
		l.Warn("proxy-header auth with no TrustedProxies configured: every peer may assert identity headers; "+
			"the deployment MUST guarantee aether is unreachable except through the authenticating proxy",
			slog.String("component", "startup"))
	}
	headerAuth := headerauth.New(headerauth.Cfg{
		UserHeader:     cfg.ProxyHeader.UserHeader,
		GroupsHeader:   cfg.ProxyHeader.GroupsHeader,
		ParseGroups:    true,
		TrustedProxies: trusted,
		Logger:         l,
	})
	return users, tokens, headerAuth, nil
}
