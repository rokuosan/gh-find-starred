package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStore_GetMiss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	store := NewFileStore[[]string](path, time.Hour)

	_, err := store.Get()
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestFileStore_SetAndGet_CreatesNestedDirectory(t *testing.T) {
	now := time.Date(2026, 11, 27, 1, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "nested", "dir", "cache.json")
	store := NewFileStore[[]string](path, time.Hour, WithClock[[]string](func() time.Time {
		return now
	}))

	want := []string{"alpha", "beta"}
	require.NoError(t, store.Set(want))

	now = now.Add(30 * time.Minute)
	got, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestFileStore_GetExpired(t *testing.T) {
	now := time.Date(2026, 11, 27, 1, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "cache.json")
	store := NewFileStore[[]string](path, time.Hour, WithClock[[]string](func() time.Time {
		return now
	}))

	require.NoError(t, store.Set([]string{"item"}))

	// Exactly at expiration boundary -> expired.
	now = now.Add(time.Hour)
	_, err := store.Get()
	assert.ErrorIs(t, err, ErrCacheExpired)
}

func TestFileStore_GetCorruptAndMissingExpiresAt(t *testing.T) {
	t.Run("corrupt json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "corrupt.json")
		require.NoError(t, os.WriteFile(path, []byte("{not-json"), 0o644))

		store := NewFileStore[[]string](path, time.Hour)
		_, err := store.Get()
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrCacheMiss)
		assert.NotErrorIs(t, err, ErrCacheExpired)
	})

	t.Run("missing expires_at", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "no-expiry.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"data":["a"]}`), 0o644))

		store := NewFileStore[[]string](path, time.Hour)
		_, err := store.Get()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing expires_at")
	})
}

func TestFileStore_AtomicOverwriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	now := time.Date(2026, 11, 27, 1, 0, 0, 0, time.UTC)
	store := NewFileStore[[]string](path, time.Hour, WithClock[[]string](func() time.Time {
		return now
	}))

	require.NoError(t, store.Set([]string{"first"}))
	require.NoError(t, store.Set([]string{"second"}))

	got, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, []string{"second"}, got)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "cache.json", entries[0].Name())
	assert.False(t, strings.HasPrefix(entries[0].Name(), ".cache-"))
}
