package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-bumbu/userauth/service/pat"
)

// patCipherKeysFile holds the AES-256 key that encrypts recoverable PAT
// secrets (user+token type) at rest, under DataDir next to session.keys.
// Losing it makes every user+token PAT unverifiable (revoke and re-create);
// leaking it reduces those tokens to plaintext for whoever also has the DB.
const patCipherKeysFile = "pat.keys"

// patCipherKeyID tags ciphertexts with the key that produced them, for
// future rotation. A single static id until rotation is actually built.
const patCipherKeyID = "k1"

const patCipherKeyLen = 32

// loadPATCipher returns the SecretCipher for recoverable PATs, generating
// and persisting a fresh key on first start. A file of the wrong size is an
// error, not silently regenerated: overwriting would orphan every existing
// user+token PAT and hide whatever corrupted it.
func loadPATCipher(dataDir string) (pat.SecretCipher, error) {
	path := filepath.Join(dataDir, patCipherKeysFile)
	key, err := os.ReadFile(path) //nolint:gosec // path is built from the validated DataDir
	if err == nil {
		if len(key) != patCipherKeyLen {
			return nil, fmt.Errorf("pat cipher key file %s has %d bytes, want %d; delete it to invalidate all user+token PATs and regenerate",
				path, len(key), patCipherKeyLen)
		}
		return pat.NewAESGCMCipher(key, patCipherKeyID)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read pat cipher key: %w", err)
	}

	key = make([]byte, patCipherKeyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate pat cipher key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("write pat cipher key: %w", err)
	}
	return pat.NewAESGCMCipher(key, patCipherKeyID)
}
