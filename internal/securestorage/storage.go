// Package securestorage provides a credential storage abstraction with
// platform-native keyring support and plaintext fallback. Matches the
// TypeScript utils/secureStorage/ layer.
package securestorage

import (
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

// ServiceName is the keychain service identifier, matching the TypeScript version.
const ServiceName = "claude-code"

// ErrNotFound is returned when a credential does not exist.
var ErrNotFound = errors.New("credential not found")

// SecureStorage provides CRUD operations for credential strings.
type SecureStorage interface {
	// Get retrieves the value associated with key.
	// Returns ErrNotFound if the key does not exist.
	Get(key string) (string, error)

	// Set stores a value under the given key.
	Set(key, value string) error

	// Delete removes the value associated with key.
	// Does not return an error if the key does not exist.
	Delete(key string) error
}

// New creates a SecureStorage backed by the platform keyring when available,
// falling back to plaintext file storage otherwise.
func New() SecureStorage {
	// Probe the keyring to see if it is functional. A successful Get or
	// ErrNotFound both indicate a working keyring daemon.
	_, err := keyring.Get(ServiceName, "__test_probe__")
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return &KeyringStorage{}
	}

	// Keyring unavailable — warn and fall back to plaintext.
	fmt.Fprintln(os.Stderr, "WARNING: Platform keyring unavailable, using plaintext credential storage")
	return NewPlaintextStorage()
}
