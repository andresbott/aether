package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

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

func TestBootstrapAdmin(t *testing.T) {
	t.Run("seeds admin with plaintext password", func(t *testing.T) {
		users, err := newUserStore(openTestDB(t))
		if err != nil {
			t.Fatal(err)
		}
		cfg := AuthCfg{Method: AuthMethodNative, AdminUser: "admin", AdminPassword: "admin"}

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
	})

	t.Run("stores a pre-hashed password as-is", func(t *testing.T) {
		users, err := newUserStore(openTestDB(t))
		if err != nil {
			t.Fatal(err)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		cfg := AuthCfg{Method: AuthMethodNative, AdminUser: "admin", AdminPassword: string(hash)}

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
		users, err := newUserStore(openTestDB(t))
		if err != nil {
			t.Fatal(err)
		}
		cfg := AuthCfg{Method: AuthMethodNative, AdminUser: "admin", AdminPassword: "admin"}
		if _, err := bootstrapAdmin(users, cfg); err != nil {
			t.Fatal(err)
		}
		// deleting the seeded admin and re-bootstrapping must not resurrect it
		// once another user exists
		if err := users.Create("other", "pw"); err != nil {
			t.Fatal(err)
		}
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

func TestResetUserPassword(t *testing.T) {
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, dbFile)), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	users, err := newUserStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := users.Create("alice", "old-pw"); err != nil {
		t.Fatal(err)
	}
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

func TestAuthConfigValidation(t *testing.T) {
	// write a minimal config file per case and load it through getAppCfg so the
	// validation path is the real one
	load := func(t *testing.T, yaml string) (AppCfg, error) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		return getAppCfg(path, true)
	}

	t.Run("defaults to none with admin:admin", func(t *testing.T) {
		cfg, err := load(t, "DataDir: ./data\n")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Auth.Method != AuthMethodNone {
			t.Errorf("default method = %q, want %q", cfg.Auth.Method, AuthMethodNone)
		}
		if cfg.Auth.AdminUser != "admin" || cfg.Auth.AdminPassword != "admin" {
			t.Errorf("default admin = %q:%q, want admin:admin", cfg.Auth.AdminUser, cfg.Auth.AdminPassword)
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
		_, err := load(t, "Auth:\n  Method: native\n  AdminPassword: \" \"\n")
		if err == nil || !strings.Contains(err.Error(), "AdminPassword") {
			t.Errorf("expected AdminPassword error, got %v", err)
		}
	})
}
