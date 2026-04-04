package securestorage

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// KeyringStorage implements SecureStorage using the platform-native credential
// store (macOS Keychain, Linux D-Bus Secret Service, Windows Credential Manager)
// via the go-keyring library.
type KeyringStorage struct{}

// Get retrieves the credential for the given key from the platform keyring.
// Returns ErrNotFound if the key does not exist.
func (k *KeyringStorage) Get(key string) (string, error) {
	val, err := keyring.Get(ServiceName, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return val, nil
}

// Set stores the credential under the given key in the platform keyring.
func (k *KeyringStorage) Set(key, value string) error {
	return keyring.Set(ServiceName, key, value)
}

// Delete removes the credential for the given key from the platform keyring.
// Returns nil if the key does not exist.
func (k *KeyringStorage) Delete(key string) error {
	err := keyring.Delete(ServiceName, key)
	if err != nil && errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
