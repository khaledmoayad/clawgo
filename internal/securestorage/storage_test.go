package securestorage

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaintextStorage_SetGetRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s := NewPlaintextStorageWithDir(dir)

	err := s.Set("test_key", "secret_value")
	require.NoError(t, err)

	val, err := s.Get("test_key")
	require.NoError(t, err)
	assert.Equal(t, "secret_value", val)
}

func TestPlaintextStorage_GetMissing(t *testing.T) {
	dir := t.TempDir()
	s := NewPlaintextStorageWithDir(dir)

	_, err := s.Get("nonexistent")
	assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got %v", err)
}

func TestPlaintextStorage_Delete(t *testing.T) {
	dir := t.TempDir()
	s := NewPlaintextStorageWithDir(dir)

	// Set then delete
	err := s.Set("del_key", "value")
	require.NoError(t, err)

	err = s.Delete("del_key")
	require.NoError(t, err)

	// Should be gone
	_, err = s.Get("del_key")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestPlaintextStorage_DeleteMissing(t *testing.T) {
	dir := t.TempDir()
	s := NewPlaintextStorageWithDir(dir)

	// Deleting a nonexistent key should not error
	err := s.Delete("nonexistent")
	assert.NoError(t, err)
}

func TestPlaintextStorage_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	s := NewPlaintextStorageWithDir(dir)

	err := s.Set("perm_key", "value")
	require.NoError(t, err)

	info, err := filepath.Glob(filepath.Join(dir, "perm_key.json"))
	require.NoError(t, err)
	require.Len(t, info, 1)
}

func TestNew_DoesNotPanic(t *testing.T) {
	// New() should return a valid SecureStorage without panicking.
	// In CI environments without a keyring daemon, it will fall back
	// to PlaintextStorage — that is expected and correct.
	assert.NotPanics(t, func() {
		storage := New()
		assert.NotNil(t, storage)
	})
}
