package cmd

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	usersHandler "github.com/andresbott/aether/app/router/handlers/users"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth/service/user"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// createUser adds a user with a plaintext password by hashing it, standing in
// for the identity service's hash-only CreateUser in tests.
func createUser(t *testing.T, users *user.Service, login, pw string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := users.CreateUser(user.Draft{LoginID: login, PasswordHash: string(hash)}); err != nil {
		t.Fatal(err)
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), dbFile)), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// assertAdminGroup checks that the user is a member of exactly the admin group.
func assertAdminGroup(t *testing.T, users *user.Service, id string) {
	t.Helper()
	groups, err := users.GetGroups(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0] != usersHandler.AdminGroup {
		t.Errorf("bootstrapped admin must be in the admin group, got %v", groups)
	}
}

func TestBootstrapAdmin(t *testing.T) {
	t.Run("seeds admin with plaintext password", func(t *testing.T) {
		users, err := newUserStore(openTestDB(t), nil)
		if err != nil {
			t.Fatal(err)
		}
		cfg := AuthCfg{Method: AuthMethodNative, AdminBootstrap: AdminBootstrapCfg{User: "admin", Pw: "admin"}}

		seeded, err := bootstrapAdmin(users, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if !seeded {
			t.Fatal("expected first bootstrap to seed the store")
		}
		usr, err := users.GetUserByLogin("admin")
		if err != nil {
			t.Fatal(err)
		}
		if !usr.Enabled {
			t.Error("bootstrapped admin should be enabled")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(usr.HashPw), []byte("admin")); err != nil {
			t.Errorf("stored hash does not verify the configured password: %v", err)
		}
		assertAdminGroup(t, users, usr.ID)
	})

	t.Run("stores a pre-hashed password as-is", func(t *testing.T) {
		users, err := newUserStore(openTestDB(t), nil)
		if err != nil {
			t.Fatal(err)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		cfg := AuthCfg{Method: AuthMethodNative, AdminBootstrap: AdminBootstrapCfg{User: "admin", Pw: string(hash)}}

		if _, err := bootstrapAdmin(users, cfg); err != nil {
			t.Fatal(err)
		}
		usr, err := users.GetUserByLogin("admin")
		if err != nil {
			t.Fatal(err)
		}
		if usr.HashPw != string(hash) {
			t.Errorf("pre-hashed password was not stored verbatim: got %q", usr.HashPw)
		}
	})

	t.Run("is a no-op once users exist", func(t *testing.T) {
		users, err := newUserStore(openTestDB(t), nil)
		if err != nil {
			t.Fatal(err)
		}
		cfg := AuthCfg{Method: AuthMethodNative, AdminBootstrap: AdminBootstrapCfg{User: "admin", Pw: "admin"}}
		if _, err := bootstrapAdmin(users, cfg); err != nil {
			t.Fatal(err)
		}
		// deleting the seeded admin and re-bootstrapping must not resurrect it
		// once another user exists
		createUser(t, users, "other", "pw")
		admin, err := users.GetUserByLogin("admin")
		if err != nil {
			t.Fatal(err)
		}
		if err := users.Delete(admin.ID); err != nil {
			t.Fatal(err)
		}

		seeded, err := bootstrapAdmin(users, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if seeded {
			t.Error("bootstrap must be a no-op while any user exists")
		}
		if _, err := users.GetUserByLogin("admin"); err == nil {
			t.Error("deleted admin must not be resurrected")
		}
	})
}

// TestSetupAuthLoginThrottle checks that the LoginThrottle config flag governs
// whether setupAuth wires a brute-force guard for native login.
func TestSetupAuthLoginThrottle(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	baseCfg := func() AuthCfg {
		return AuthCfg{Method: AuthMethodNative, AdminBootstrap: AdminBootstrapCfg{User: "admin", Pw: "admin"}}
	}

	t.Run("enabled by default", func(t *testing.T) {
		deps, err := setupAuth(openTestDB(t), t.TempDir(), baseCfg(), logger)
		if err != nil {
			t.Fatal(err)
		}
		if deps.LoginGuard == nil {
			t.Fatal("login throttle defaults to on, want a non-nil guard")
		}
	})

	t.Run("disabled by config", func(t *testing.T) {
		cfg := baseCfg()
		off := false
		cfg.LoginThrottle.Enabled = &off
		deps, err := setupAuth(openTestDB(t), t.TempDir(), cfg, logger)
		if err != nil {
			t.Fatal(err)
		}
		if deps.LoginGuard != nil {
			t.Fatal("login throttle disabled by config, want a nil guard")
		}
	})
}

func TestResetUserPassword(t *testing.T) {
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, dbFile)), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	users, err := newUserStore(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	createUser(t, users, "alice", "old-pw")
	cfg := AppCfg{DataDir: dataDir}

	t.Run("updates the password of an existing user", func(t *testing.T) {
		if err := resetUserPassword(cfg, "alice", "new-pw"); err != nil {
			t.Fatal(err)
		}
		usr, err := users.GetUserByLogin("alice")
		if err != nil {
			t.Fatal(err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(usr.HashPw), []byte("new-pw")); err != nil {
			t.Errorf("new password does not verify: %v", err)
		}
	})

	t.Run("fails for a missing user", func(t *testing.T) {
		err := resetUserPassword(cfg, "nobody", "pw")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected a not-found error, got %v", err)
		}
	})

	t.Run("fails when the database does not exist", func(t *testing.T) {
		err := resetUserPassword(AppCfg{DataDir: t.TempDir()}, "alice", "pw")
		if err == nil || !strings.Contains(err.Error(), "database not found") {
			t.Errorf("expected a database-not-found error, got %v", err)
		}
	})
}

// loadAuthCfg writes a minimal config file and loads it through getAppCfg, so
// every case below exercises the real validation path rather than a struct
// literal.
func loadAuthCfg(t *testing.T, yaml string) (AppCfg, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return getAppCfg(path, true)
}

func TestAuthConfigValidation(t *testing.T) {
	load := loadAuthCfg

	t.Run("defaults to none with admin:admin", func(t *testing.T) {
		cfg, err := load(t, "DataDir: ./data\n")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Auth.Method != AuthMethodNone {
			t.Errorf("default method = %q, want %q", cfg.Auth.Method, AuthMethodNone)
		}
		if cfg.Auth.AdminBootstrap.User != "admin" || cfg.Auth.AdminBootstrap.Pw != "admin" {
			t.Errorf("default admin = %q:%q, want admin:admin",
				cfg.Auth.AdminBootstrap.User, cfg.Auth.AdminBootstrap.Pw)
		}
	})

	t.Run("accepts native and normalizes case", func(t *testing.T) {
		cfg, err := load(t, "Auth:\n  Method: Native\n")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Auth.Method != AuthMethodNative {
			t.Errorf("method = %q, want %q", cfg.Auth.Method, AuthMethodNative)
		}
	})

	t.Run("rejects unknown method", func(t *testing.T) {
		_, err := load(t, "Auth:\n  Method: ldap\n")
		if err == nil || !strings.Contains(err.Error(), "invalid auth method") {
			t.Errorf("expected invalid-method error, got %v", err)
		}
	})

	t.Run("native requires a non-empty admin password", func(t *testing.T) {
		_, err := load(t, "Auth:\n  Method: native\n  AdminBootstrap:\n    Pw: \" \"\n")
		if err == nil || !strings.Contains(err.Error(), "AdminBootstrap.Pw") {
			t.Errorf("expected AdminBootstrap.Pw error, got %v", err)
		}
	})

}

func TestProxyHeaderConfigValidation(t *testing.T) {
	load := loadAuthCfg

	t.Run("accepts proxy-header with header defaults", func(t *testing.T) {
		cfg, err := load(t, "Auth:\n  Method: proxy-header\n")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Auth.Method != AuthMethodProxyHeader {
			t.Errorf("method = %q, want %q", cfg.Auth.Method, AuthMethodProxyHeader)
		}
		ph := cfg.Auth.ProxyHeader
		if ph.UserHeader != "Remote-User" || ph.GroupsHeader != "Remote-Groups" || ph.AdminGroup != "aether-admin" {
			t.Errorf("proxy header defaults = %+v, want Remote-User/Remote-Groups/aether-admin", ph)
		}
	})

	t.Run("proxy-header does not require admin credentials", func(t *testing.T) {
		_, err := load(t, "Auth:\n  Method: proxy-header\n  AdminBootstrap:\n    User: \" \"\n    Pw: \" \"\n")
		if err != nil {
			t.Errorf("proxy-header with blank admin credentials: %v", err)
		}
	})

	t.Run("proxy-header accepts CIDRs and bare IPs", func(t *testing.T) {
		cfg, err := load(t, "Auth:\n  Method: proxy-header\n  ProxyHeader:\n    TrustedProxies:\n      - 10.0.0.0/8\n      - 192.168.1.5\n")
		if err != nil {
			t.Fatal(err)
		}
		prefixes, err := parseTrustedProxies(cfg.Auth.ProxyHeader.TrustedProxies)
		if err != nil {
			t.Fatal(err)
		}
		if len(prefixes) != 2 || prefixes[0].String() != "10.0.0.0/8" || prefixes[1].String() != "192.168.1.5/32" {
			t.Errorf("parsed prefixes = %v, want [10.0.0.0/8 192.168.1.5/32]", prefixes)
		}
	})

	t.Run("proxy-header rejects malformed trusted proxies", func(t *testing.T) {
		_, err := load(t, "Auth:\n  Method: proxy-header\n  ProxyHeader:\n    TrustedProxies:\n      - not-a-cidr\n")
		if err == nil || !strings.Contains(err.Error(), "invalid trusted proxy") {
			t.Errorf("expected trusted-proxy error, got %v", err)
		}
	})

	t.Run("proxy-header requires a user header", func(t *testing.T) {
		_, err := load(t, "Auth:\n  Method: proxy-header\n  ProxyHeader:\n    UserHeader: \" \"\n")
		if err == nil || !strings.Contains(err.Error(), "UserHeader") {
			t.Errorf("expected UserHeader error, got %v", err)
		}
	})
}
