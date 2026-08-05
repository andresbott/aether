package cmd

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-bumbu/userauth/auth/cookieauth"
)

// Session cookie policy for the native auth mode. Without "remember me" a
// session lasts sessionDur from login, fixed. With it, activity renews the
// rolling window until maxSessionDur forces a re-login.
const (
	sessionDur    = 24 * time.Hour
	maxSessionDur = 30 * 24 * time.Hour
)

// sessionKeysFile holds the securecookie hash+block keys under DataDir.
// Persisting them keeps sessions valid across server restarts.
const sessionKeysFile = "session.keys"

// sessionCookieName is the name of aether's session cookie.
const sessionCookieName = "_aether_login"

const (
	sessionHashKeyLen  = 64
	sessionBlockKeyLen = 32
)

// loadSessionKeys returns the cookie hash and block keys from dataDir,
// generating and persisting fresh ones on first start. A file of the wrong
// size is an error, not silently regenerated: overwriting would log every
// user out and hide whatever corrupted it.
func loadSessionKeys(dataDir string) (hashKey, blockKey []byte, err error) {
	path := filepath.Join(dataDir, sessionKeysFile)
	keys, err := os.ReadFile(path) //nolint:gosec // path is built from the validated DataDir
	if err == nil {
		if len(keys) != sessionHashKeyLen+sessionBlockKeyLen {
			return nil, nil, fmt.Errorf("session keys file %s has %d bytes, want %d; delete it to regenerate (logs everyone out)",
				path, len(keys), sessionHashKeyLen+sessionBlockKeyLen)
		}
		return keys[:sessionHashKeyLen], keys[sessionHashKeyLen:], nil
	}
	if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("read session keys: %w", err)
	}

	keys = make([]byte, sessionHashKeyLen+sessionBlockKeyLen)
	if _, err := rand.Read(keys); err != nil {
		return nil, nil, fmt.Errorf("generate session keys: %w", err)
	}
	if err := os.WriteFile(path, keys, 0o600); err != nil {
		return nil, nil, fmt.Errorf("write session keys: %w", err)
	}
	return keys[:sessionHashKeyLen], keys[sessionHashKeyLen:], nil
}

// newSessionManager builds the cookie session manager used to guard /api/v1
// in native mode. AllowRenew is on so the login payload's sessionRenew
// ("remember me") opts a session into rolling renewal.
func newSessionManager(dataDir string, l *slog.Logger) (*cookieauth.Manager, error) {
	hashKey, blockKey, err := loadSessionKeys(dataDir)
	if err != nil {
		return nil, err
	}
	store, err := cookieauth.NewCookieStore(hashKey, blockKey)
	if err != nil {
		return nil, fmt.Errorf("cookie store: %w", err)
	}
	// Aether is commonly served plain-HTTP on a LAN, where a Secure cookie is
	// dropped by the browser and login silently never sticks. Lax keeps the
	// cookie off cross-site requests (CSRF) while allowing normal navigation.
	store.Options.Secure = false
	store.Options.SameSite = http.SameSiteLaxMode

	return cookieauth.New(cookieauth.Cfg{
		Store:         store,
		SessionDur:    sessionDur,
		MaxSessionDur: maxSessionDur,
		AllowRenew:    true,
		CookieName:    sessionCookieName,
		Logger:        l,
	})
}
