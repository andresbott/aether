package cmd

import (
	"fmt"
	"log/slog"

	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/go-bumbu/userauth/auth/cookieauth"
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
		LoginID:    cfg.AdminUser,
		Pw:         cfg.AdminPassword,
		PwIsHashed: isBcryptHash(cfg.AdminPassword),
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
func setupNativeAuth(db *gorm.DB, dataDir string, cfg AuthCfg, l *slog.Logger) (*userdb.Store, *cookieauth.Manager, error) {
	if cfg.Method != AuthMethodNative {
		return nil, nil, nil
	}
	users, err := newUserStore(db)
	if err != nil {
		return nil, nil, fmt.Errorf("user store: %w", err)
	}
	seeded, err := bootstrapAdmin(users, cfg)
	if err != nil {
		return nil, nil, err
	}
	if seeded {
		l.Info("seeded initial admin user",
			slog.String("component", "startup"), slog.String("user", cfg.AdminUser))
	}
	sessions, err := newSessionManager(dataDir, l)
	if err != nil {
		return nil, nil, fmt.Errorf("session manager: %w", err)
	}
	return users, sessions, nil
}
