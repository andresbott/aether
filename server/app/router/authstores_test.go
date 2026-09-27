package router

import (
	"testing"

	"github.com/go-bumbu/userauth"
	"github.com/go-bumbu/userauth/service/password"
	"github.com/go-bumbu/userauth/service/pat"
	patdb "github.com/go-bumbu/userauth/service/pat/store/db"
	"github.com/go-bumbu/userauth/service/user"
	userdb "github.com/go-bumbu/userauth/service/user/store/db"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// newTestAuthStores builds the identity, password and PAT services on db the
// way production wires them (userauth v0.8.0), for router tests. cipher may be
// nil, restricting the PAT service to hash-only tokens. Passwords are hashed at
// bcrypt.MinCost to keep the suite fast.
func newTestAuthStores(t *testing.T, db *gorm.DB, cipher pat.SecretCipher) (*user.Service, *password.Service, *pat.Service) {
	t.Helper()
	userStore, err := userdb.New(db)
	if err != nil {
		t.Fatal(err)
	}
	patStore, err := patdb.New(db)
	if err != nil {
		t.Fatal(err)
	}
	users, err := user.NewService(userStore, user.Opts{DefaultEnabled: true, OnDelete: []user.Purger{patStore}})
	if err != nil {
		t.Fatal(err)
	}
	passwords, err := password.NewService(password.Opts{Cost: bcrypt.MinCost, Rehash: users})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pat.NewService(patStore, users, pat.Opts{Prefix: "aether", Cipher: cipher})
	if err != nil {
		t.Fatal(err)
	}
	return users, passwords, tokens
}

// mkTestUser creates an enabled user with a plaintext password (hashed at
// MinCost) and optional group memberships, returning the created record.
func mkTestUser(t *testing.T, users *user.Service, login, pw string, groups ...string) userauth.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	u, err := users.CreateUser(user.Draft{LoginID: login, PasswordHash: string(hash), Enabled: &enabled, Groups: groups})
	if err != nil {
		t.Fatal(err)
	}
	return u
}
