package app

import (
	"context"
	"errors"
	"testing"

	"github.com/rokuosan/gh-find-starred/internal/cache"
	"github.com/rokuosan/gh-find-starred/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubFetcher struct {
	fetchFn func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error)
}

func (f *stubFetcher) FetchStarredRepositories(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
	return f.fetchFn(ctx, onProgress)
}

type stubIndexStore struct {
	loadFn     func(ctx context.Context) (domain.IndexSearcher, int, error)
	saveFn     func(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error)
	fallbackFn func(repos []domain.Repository) domain.IndexSearcher
}

func (s *stubIndexStore) Load(ctx context.Context) (domain.IndexSearcher, int, error) {
	return s.loadFn(ctx)
}

func (s *stubIndexStore) Save(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error) {
	return s.saveFn(ctx, repos)
}

func (s *stubIndexStore) Fallback(repos []domain.Repository) domain.IndexSearcher {
	return s.fallbackFn(repos)
}

type stubSearcher struct {
	results []domain.SearchResult
}

func (s *stubSearcher) Search(ctx context.Context, query string, limit int) ([]domain.SearchResult, error) {
	return s.results, nil
}

func TestService_Prepare_CacheHit(t *testing.T) {
	cachedSearcher := &stubSearcher{results: []domain.SearchResult{{Repository: domain.Repository{Name: "cached"}}}}
	fetched := false

	svc := NewService(
		&stubFetcher{
			fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
				fetched = true
				return nil, nil
			},
		},
		&stubIndexStore{
			loadFn: func(ctx context.Context) (domain.IndexSearcher, int, error) {
				return cachedSearcher, 42, nil
			},
			saveFn: func(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error) {
				t.Fatal("Save should not be called on cache hit")
				return nil, nil
			},
		},
	)

	status, searcher, err := svc.Prepare(context.Background(), false, nil)
	require.NoError(t, err)
	assert.True(t, status.FromCache)
	assert.Equal(t, 42, status.RepoCount)
	assert.NoError(t, status.CacheWarning)
	assert.Same(t, cachedSearcher, searcher)
	assert.False(t, fetched)
}

func TestService_Prepare_NoCacheBypass(t *testing.T) {
	fresh := []domain.Repository{{Name: "fresh", URL: "https://github.com/a/fresh"}}
	savedSearcher := &stubSearcher{}
	var savedRepos []domain.Repository

	svc := NewService(
		&stubFetcher{
			fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
				return fresh, nil
			},
		},
		&stubIndexStore{
			loadFn: func(ctx context.Context) (domain.IndexSearcher, int, error) {
				t.Fatal("Load should not be called when noCache is true")
				return nil, 0, nil
			},
			saveFn: func(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error) {
				savedRepos = repos
				return savedSearcher, nil
			},
		},
	)

	status, searcher, err := svc.Prepare(context.Background(), true, nil)
	require.NoError(t, err)
	assert.False(t, status.FromCache)
	assert.Equal(t, 1, status.RepoCount)
	assert.Equal(t, fresh, savedRepos)
	assert.Same(t, savedSearcher, searcher)
}

func TestService_Prepare_FallbackOnLoadErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		loadErr error
	}{
		{name: "miss", loadErr: cache.ErrCacheMiss},
		{name: "expired", loadErr: cache.ErrCacheExpired},
		{name: "corrupt", loadErr: errors.New("corrupt index")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fresh := []domain.Repository{{Name: "repo", URL: "https://github.com/a/repo"}}
			savedSearcher := &stubSearcher{}
			var progressCount int

			svc := NewService(
				&stubFetcher{
					fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
						if onProgress != nil {
							onProgress(domain.FetchProgress{FetchedCount: 1})
						}
						return fresh, nil
					},
				},
				&stubIndexStore{
					loadFn: func(ctx context.Context) (domain.IndexSearcher, int, error) {
						return nil, 0, tc.loadErr
					},
					saveFn: func(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error) {
						return savedSearcher, nil
					},
				},
			)

			status, searcher, err := svc.Prepare(context.Background(), false, func(p domain.FetchProgress) {
				progressCount = p.FetchedCount
			})
			require.NoError(t, err)
			assert.False(t, status.FromCache)
			assert.Equal(t, 1, status.RepoCount)
			assert.Equal(t, 1, progressCount)
			assert.Same(t, savedSearcher, searcher)
		})
	}
}

func TestService_Prepare_SaveWarningUsesFallback(t *testing.T) {
	fresh := []domain.Repository{{Name: "repo", URL: "https://github.com/a/repo"}}
	writeErr := errors.New("disk full")
	fallbackSearcher := &stubSearcher{}

	svc := NewService(
		&stubFetcher{
			fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
				return fresh, nil
			},
		},
		&stubIndexStore{
			loadFn: func(ctx context.Context) (domain.IndexSearcher, int, error) {
				return nil, 0, cache.ErrCacheMiss
			},
			saveFn: func(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error) {
				return nil, writeErr
			},
			fallbackFn: func(repos []domain.Repository) domain.IndexSearcher {
				assert.Equal(t, fresh, repos)
				return fallbackSearcher
			},
		},
	)

	status, searcher, err := svc.Prepare(context.Background(), false, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, status.RepoCount)
	assert.False(t, status.FromCache)
	assert.ErrorIs(t, status.CacheWarning, writeErr)
	assert.Same(t, fallbackSearcher, searcher)
}

func TestService_Prepare_ContextCancelAndFetchError(t *testing.T) {
	t.Run("load canceled propagates without fetching", func(t *testing.T) {
		svc := NewService(
			&stubFetcher{
				fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
					t.Fatal("fetcher should not be called when Load is canceled")
					return nil, nil
				},
			},
			&stubIndexStore{
				loadFn: func(ctx context.Context) (domain.IndexSearcher, int, error) {
					return nil, 0, context.Canceled
				},
			},
		)

		_, _, err := svc.Prepare(context.Background(), false, nil)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("save canceled propagates without fallback", func(t *testing.T) {
		svc := NewService(
			&stubFetcher{
				fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
					return []domain.Repository{{Name: "a"}}, nil
				},
			},
			&stubIndexStore{
				loadFn: func(ctx context.Context) (domain.IndexSearcher, int, error) {
					return nil, 0, cache.ErrCacheMiss
				},
				saveFn: func(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error) {
					return nil, context.Canceled
				},
				fallbackFn: func(repos []domain.Repository) domain.IndexSearcher {
					t.Fatal("Fallback should not be called on cancel")
					return nil
				},
			},
		)

		_, _, err := svc.Prepare(context.Background(), false, nil)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("fetch error", func(t *testing.T) {
		fetchErr := errors.New("network failure")
		svc := NewService(
			&stubFetcher{
				fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
					return nil, fetchErr
				},
			},
			&stubIndexStore{
				loadFn: func(ctx context.Context) (domain.IndexSearcher, int, error) {
					return nil, 0, cache.ErrCacheMiss
				},
				saveFn: func(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error) {
					t.Fatal("Save should not be called on fetch error")
					return nil, nil
				},
			},
		)

		_, _, err := svc.Prepare(context.Background(), false, nil)
		assert.ErrorIs(t, err, fetchErr)
	})
}
