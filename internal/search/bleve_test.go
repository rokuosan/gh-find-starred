package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rokuosan/gh-find-starred/internal/cache"
	"github.com/rokuosan/gh-find-starred/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDiskStore(t *testing.T, ttl time.Duration, clock func() time.Time) (*DiskStore, cache.Store[IndexMeta], string) {
	t.Helper()
	dir := t.TempDir()
	metaStore := cache.NewFileStore[IndexMeta](
		filepath.Join(dir, "meta.json"),
		ttl,
		cache.WithClock[IndexMeta](clock),
	)
	store := NewDiskStore(dir, metaStore, WithClock(clock))
	return store, metaStore, dir
}

func TestDiskStore_SaveLoadSearch_RoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)
	store, _, _ := newTestDiskStore(t, time.Hour, func() time.Time { return now })

	repos := []domain.Repository{
		{
			Name:        "bubbletea",
			URL:         "https://github.com/charmbracelet/bubbletea",
			Description: "A powerful little TUI framework",
			Readme:      "Build terminal applications in Go.",
		},
		{
			Name:        "cli-notes",
			URL:         "https://github.com/example/cli-notes",
			Description: "Simple markdown notes",
			Readme:      "Includes a secret-readme-keyword viewer.",
		},
		{
			Name:        "unrelated",
			URL:         "https://github.com/example/unrelated",
			Description: "Database migration utility",
			Readme:      "PostgreSQL and MySQL migrations.",
		},
	}

	searcher, err := store.Save(context.Background(), repos)
	require.NoError(t, err)
	require.NotNil(t, searcher)

	// Load from disk and verify count + search across name/description/readme.
	loaded, count, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(repos), count)

	results, err := loaded.Search(context.Background(), "bubbletea", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "bubbletea", results[0].Repository.Name)
	assert.Equal(t, "https://github.com/charmbracelet/bubbletea", results[0].Repository.URL)
	assert.Equal(t, "A powerful little TUI framework", results[0].Repository.Description)
	// README is indexed-only, not stored in search results.
	assert.Empty(t, results[0].Repository.Readme)

	readmeResults, err := loaded.Search(context.Background(), "secret-readme-keyword", 10)
	require.NoError(t, err)
	require.Len(t, readmeResults, 1)
	assert.Equal(t, "cli-notes", readmeResults[0].Repository.Name)
	assert.Empty(t, readmeResults[0].Repository.Readme)
}

func TestDiskStore_DuplicateNamesLimitAndEdgeCases(t *testing.T) {
	now := time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)
	store, _, _ := newTestDiskStore(t, time.Hour, func() time.Time { return now })

	repos := []domain.Repository{
		{Name: "dotfiles", URL: "https://github.com/alice/dotfiles", Description: "Neovim setup"},
		{Name: "dotfiles", URL: "https://github.com/bob/dotfiles", Description: "Tmux setup"},
		{Name: "dotfiles", URL: "https://github.com/carol/dotfiles", Description: "Zsh setup"},
	}

	searcher, err := store.Save(context.Background(), repos)
	require.NoError(t, err)

	all, err := searcher.Search(context.Background(), "dotfiles", 0)
	require.NoError(t, err)
	require.Len(t, all, 3)

	limited, err := searcher.Search(context.Background(), "dotfiles", 2)
	require.NoError(t, err)
	require.Len(t, limited, 2)

	blank, err := searcher.Search(context.Background(), "   ", 10)
	require.NoError(t, err)
	assert.Nil(t, blank)

	_, err = searcher.Search(context.Background(), `name:"unclosed`, 10)
	require.Error(t, err)
}

func TestDiskStore_EmptyReposSaveLoadSearch(t *testing.T) {
	now := time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)
	store, _, _ := newTestDiskStore(t, time.Hour, func() time.Time { return now })

	_, err := store.Save(context.Background(), nil)
	require.NoError(t, err)

	loaded, count, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	results, err := loaded.Search(context.Background(), "anything", 10)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestDiskStore_LoadInvalidAndExpiredStates(t *testing.T) {
	t.Run("expired metadata", func(t *testing.T) {
		now := time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)
		store, _, _ := newTestDiskStore(t, time.Hour, func() time.Time { return now })

		_, err := store.Save(context.Background(), []domain.Repository{{Name: "a", URL: "https://github.com/u/a"}})
		require.NoError(t, err)

		now = now.Add(time.Hour)
		_, _, err = store.Load(context.Background())
		assert.ErrorIs(t, err, cache.ErrCacheExpired)
	})

	t.Run("missing bleve directory", func(t *testing.T) {
		now := time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)
		store, metaStore, dir := newTestDiskStore(t, time.Hour, func() time.Time { return now })

		_, err := store.Save(context.Background(), []domain.Repository{{Name: "a", URL: "https://github.com/u/a"}})
		require.NoError(t, err)

		meta, err := metaStore.Get()
		require.NoError(t, err)
		require.NoError(t, os.RemoveAll(filepath.Join(dir, meta.IndexDir)))

		_, _, err = store.Load(context.Background())
		require.Error(t, err)
	})

	t.Run("path traversal index_dir rejected", func(t *testing.T) {
		now := time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)
		store, metaStore, _ := newTestDiskStore(t, time.Hour, func() time.Time { return now })

		require.NoError(t, metaStore.Set(IndexMeta{
			SchemaVersion: CurrentSchemaVersion,
			IndexDir:      "../escape.bleve",
			RepoCount:     1,
		}))

		_, _, err := store.Load(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid index directory")
	})

	t.Run("unsupported schema version rejected", func(t *testing.T) {
		now := time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)
		store, metaStore, _ := newTestDiskStore(t, time.Hour, func() time.Time { return now })

		require.NoError(t, metaStore.Set(IndexMeta{
			SchemaVersion: CurrentSchemaVersion + 1,
			IndexDir:      "index-1-abcd.bleve",
			RepoCount:     1,
		}))

		_, _, err := store.Load(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported index schema version")
	})
}

func TestDiskStore_OverwriteWithFrozenClockRemovesStaleIndex(t *testing.T) {
	frozen := time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)
	store, metaStore, dir := newTestDiskStore(t, time.Hour, func() time.Time { return frozen })

	_, err := store.Save(context.Background(), []domain.Repository{{Name: "first", URL: "https://github.com/u/first"}})
	require.NoError(t, err)

	meta1, err := metaStore.Get()
	require.NoError(t, err)

	_, err = store.Save(context.Background(), []domain.Repository{{Name: "second", URL: "https://github.com/u/second"}})
	require.NoError(t, err)

	meta2, err := metaStore.Get()
	require.NoError(t, err)
	assert.NotEqual(t, meta1.IndexDir, meta2.IndexDir)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var indexDirs []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "index-") {
			indexDirs = append(indexDirs, entry.Name())
		}
	}
	assert.Equal(t, []string{meta2.IndexDir}, indexDirs)

	loaded, count, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	res, err := loaded.Search(context.Background(), "second", 10)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "second", res[0].Repository.Name)
}

func TestDiskStore_CanceledSaveCleansUpPartialIndex(t *testing.T) {
	now := time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)
	store, _, dir := newTestDiskStore(t, time.Hour, func() time.Time { return now })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Save(ctx, []domain.Repository{{Name: "a", URL: "https://github.com/u/a"}})
	assert.ErrorIs(t, err, context.Canceled)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestDiskStore_FallbackMemoryIndexParity(t *testing.T) {
	store, _, _ := newTestDiskStore(t, time.Hour, time.Now)
	repos := []domain.Repository{
		{
			Name:        "gh-find-starred",
			URL:         "https://github.com/rokuosan/gh-find-starred",
			Description: "Search starred repos",
			Readme:      "Powered by bleve full-text search.",
		},
	}

	mem := store.Fallback(repos)
	res, err := mem.Search(context.Background(), "bleve", 10)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "gh-find-starred", res[0].Repository.Name)
	assert.Empty(t, res[0].Repository.Readme)
}
