package cmd

import (
	"fmt"
	"log/slog"

	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/go-bumbu/userauth/auth/cookieauth"
	"github.com/go-bumbu/userauth/auth/headerauth"
	"github.com/go-bumbu/userauth/service/pat"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"gorm.io/gorm"
)

// bcryptDifficulty is the shared cost for all password hashes aether creates
// (admin bootstrap, CLI hash/reset, users API).
const bcryptDifficulty = usersHandler.BcryptDifficulty

// newUserStore builds the native user store on the shared aether DB.
func newUserStore(db *gorm.DB) (*userdb.Store, error) {
	return userdb.New(db, userdb.Opts{
		BcryptDifficulty: bcryptDifficulty,
		DefaultEnabled:   true,
	})
}

// bootstrapAdmin seeds the initial admin from config. It relies on
// Store.Bootstrap being idempotent: the admin is only created while the store
// has no users at all, so it is safe to call on every startup and stale
// config never resurrects deleted accounts. The configured password may be a
// bcrypt hash (e.g. produced by `aether user hash`) instead of plaintext.
func bootstrapAdmin(users *userdb.Store, cfg AuthCfg) (bool, error) {
	admin := userdb.User{
		LoginID:    cfg.AdminBootstrap.User,
		Pw:         cfg.AdminBootstrap.Pw,
		PwIsHashed: isBcryptHash(cfg.AdminBootstrap.Pw),
		Enabled:    true,
		Groups:     []string{usersHandler.AdminGroup},
	}
	seeded, err := users.Bootstrap(admin)
	if err != nil {
		return false, fmt.Errorf("bootstrap admin user: %w", err)
	}
	return seeded, nil
}

// setupNativeAuth creates the user store, seeds the initial admin and builds
// the cookie session manager when the auth method is native, returning both
// for the router (users CRUD, login endpoints, /api/v1 session guard). With
// auth method "none" it returns nils — nothing auth-related is created at all.
func setupNativeAuth(db *gorm.DB, dataDir string, cfg AuthCfg, l *slog.Logger) (*userdb.Store, *cookieauth.Manager, *pat.Service, error) {
	if cfg.Method != AuthMethodNative {
		return nil, nil, nil, nil
	}
	users, err := newUserStore(db)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("user store: %w", err)
	}
	seeded, err := bootstrapAdmin(users, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	if seeded {
		l.Info("seeded initial admin user",
			slog.String("component", "startup"), slog.String("user", cfg.AdminBootstrap.User))
	}
	sessions, err := newSessionManager(dataDir, l)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("session manager: %w", err)
	}
	tokens, err := newTokenService(users, dataDir, l)
	if err != nil {
		return nil, nil, nil, err
	}
	return users, sessions, tokens, nil
}

// authDeps is everything the router needs to enforce the configured auth
// method. Exactly one mode's fields are populated (or none, for method "none"):
// Sessions is native-only, HeaderAuth proxy-only, and Users/Tokens are the
// shared halves both authenticated modes build.
type authDeps struct {
	Users      *userdb.Store
	Sessions   *cookieauth.Manager
	Tokens     *pat.Service
	HeaderAuth *headerauth.HeaderHandler
}

// setupAuth builds the auth dependencies for whichever method is configured.
// Both setups are consulted because each is a no-op outside its own mode; the
// method then decides which one's stores win.
func setupAuth(db *gorm.DB, dataDir string, cfg AuthCfg, l *slog.Logger) (authDeps, error) {
	users, sessions, tokens, err := setupNativeAuth(db, dataDir, cfg, l)
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
	return authDeps{Users: users, Sessions: sessions, Tokens: tokens, HeaderAuth: headerAuth}, nil
}

// newTokenService builds the PAT service on the user store — the same token
// layer in every authenticated mode (docs/agents/authentication.md). The
// cipher enables recoverable (user+token) PATs; hash-only apikey PATs work
// without it.
func newTokenService(users *userdb.Store, dataDir string, l *slog.Logger) (*pat.Service, error) {
	cipher, err := loadPATCipher(dataDir)
	if err != nil {
		return nil, fmt.Errorf("pat cipher: %w", err)
	}
	tokens, err := pat.NewService(users.PATStore(), users, pat.Opts{Prefix: "aether", Cipher: cipher, Logger: l})
	if err != nil {
		return nil, fmt.Errorf("token service: %w", err)
	}
	return tokens, nil
}

// setupProxyAuth creates the pieces for the proxy-header mode: the same user
// store and PAT service as native (users are provisioned on first sight of a
// new identity, so no admin bootstrap) plus the header handler that validates
// proxy-injected identity. No session manager — the proxy owns the session.
func setupProxyAuth(db *gorm.DB, dataDir string, cfg AuthCfg, l *slog.Logger) (*userdb.Store, *pat.Service, *headerauth.HeaderHandler, error) {
	if cfg.Method != AuthMethodProxyHeader {
		return nil, nil, nil, nil
	}
	users, err := newUserStore(db)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("user store: %w", err)
	}
	tokens, err := newTokenService(users, dataDir, l)
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
