package securestorage

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/khaledmoayad/clawgo/internal/config"
)

// PlaintextStorage implements SecureStorage by storing credentials as individual
// files under a directory with restricted permissions. Used as a fallback when
// no platform keyring is available (matching the TypeScript plainTextStorage.ts).
type PlaintextStorage struct {
	dir string
}

// NewPlaintextStorage creates a PlaintextStorage using
// ~/.claude/credentials/ (or CLAUDE_CONFIG_DIR override) as the backing directory.
// The directory is created with 0700 permissions if it does not exist.
func NewPlaintextStorage() *PlaintextStorage {
	dir := filepath.Join(config.ConfigDir(), "credentials")
	_ = os.MkdirAll(dir, 0700)
	return &PlaintextStorage{dir: dir}
}

// NewPlaintextStorageWithDir creates a PlaintextStorage backed by the given
// directory. Exported for use by other internal packages in tests.
func NewPlaintextStorageWithDir(dir string) *PlaintextStorage {
	_ = os.MkdirAll(dir, 0700)
	return &PlaintextStorage{dir: dir}
}

// Get reads the credential file for the given key.
// Returns ErrNotFound if the file does not exist.
func (p *PlaintextStorage) Get(key string) (string, error) {
	data, err := os.ReadFile(p.path(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNotFound
		}
		return "", err
	}
	return string(data), nil
}

// Set writes the value to a credential file with 0600 permissions.
func (p *PlaintextStorage) Set(key, value string) error {
	return os.WriteFile(p.path(key), []byte(value), 0600)
}

// Delete removes the credential file for the given key.
// Returns nil if the file does not exist.
func (p *PlaintextStorage) Delete(key string) error {
	err := os.Remove(p.path(key))
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// path returns the file path for a credential key.
func (p *PlaintextStorage) path(key string) string {
	return filepath.Join(p.dir, key+".json")
}
