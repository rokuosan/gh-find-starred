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

type stubStore struct {
	getFn func() ([]domain.Repository, error)
	setFn func([]domain.Repository) error
}

func (s *stubStore) Get() ([]domain.Repository, error) {
	return s.getFn()
}

func (s *stubStore) Set(data []domain.Repository) error {
	return s.setFn(data)
}

type stubFetcher struct {
	fetchFn func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error)
}

func (f *stubFetcher) FetchStarredRepositories(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
	return f.fetchFn(ctx, onProgress)
}

type stubSearcher struct {
	searchFn func(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error)
}

func (s *stubSearcher) Search(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error) {
	return s.searchFn(repos, query, limit)
}

func TestService_LoadRepositories_CacheHit(t *testing.T) {
	cached := []domain.Repository{{Name: "cached-repo", URL: "https://github.com/a/cached-repo"}}
	fetched := false

	svc := NewService(
		&stubFetcher{
			fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
				fetched = true
				return nil, nil
			},
		},
		&stubStore{
			getFn: func() ([]domain.Repository, error) { return cached, nil },
			setFn: func(r []domain.Repository) error {
				t.Fatal("Set should not be called on cache hit")
				return nil
			},
		},
		nil,
	)

	res, err := svc.LoadRepositories(context.Background(), false, nil)
	require.NoError(t, err)
	assert.True(t, res.FromCache)
	assert.NoError(t, res.CacheWarning)
	assert.Equal(t, cached, res.Repositories)
	assert.False(t, fetched)
}

func TestService_LoadRepositories_NoCacheBypass(t *testing.T) {
	fresh := []domain.Repository{{Name: "fresh-repo", URL: "https://github.com/a/fresh-repo"}}
	var stored []domain.Repository

	svc := NewService(
		&stubFetcher{
			fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
				return fresh, nil
			},
		},
		&stubStore{
			getFn: func() ([]domain.Repository, error) {
				t.Fatal("Get should not be called when noCache is true")
				return nil, nil
			},
			setFn: func(r []domain.Repository) error {
				stored = r
				return nil
			},
		},
		nil,
	)

	res, err := svc.LoadRepositories(context.Background(), true, nil)
	require.NoError(t, err)
	assert.False(t, res.FromCache)
	assert.Equal(t, fresh, res.Repositories)
	assert.Equal(t, fresh, stored)
}

func TestService_LoadRepositories_FallbackOnCacheErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cacheErr error
	}{
		{name: "miss", cacheErr: cache.ErrCacheMiss},
		{name: "expired", cacheErr: cache.ErrCacheExpired},
		{name: "corrupt", cacheErr: errors.New("corrupt json")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fresh := []domain.Repository{{Name: "repo", URL: "https://github.com/a/repo"}}
			var saved []domain.Repository
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
				&stubStore{
					getFn: func() ([]domain.Repository, error) { return nil, tc.cacheErr },
					setFn: func(r []domain.Repository) error {
						saved = r
						return nil
					},
				},
				nil,
			)

			res, err := svc.LoadRepositories(context.Background(), false, func(p domain.FetchProgress) {
				progressCount = p.FetchedCount
			})
			require.NoError(t, err)
			assert.False(t, res.FromCache)
			assert.Equal(t, 1, progressCount)
			assert.Equal(t, fresh, res.Repositories)
			assert.Equal(t, fresh, saved)
		})
	}
}

func TestService_LoadRepositories_CacheWriteWarningIsNonFatal(t *testing.T) {
	fresh := []domain.Repository{{Name: "repo", URL: "https://github.com/a/repo"}}
	writeErr := errors.New("disk full")

	svc := NewService(
		&stubFetcher{
			fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
				return fresh, nil
			},
		},
		&stubStore{
			getFn: func() ([]domain.Repository, error) { return nil, cache.ErrCacheMiss },
			setFn: func(r []domain.Repository) error { return writeErr },
		},
		nil,
	)

	res, err := svc.LoadRepositories(context.Background(), false, nil)
	require.NoError(t, err)
	assert.Equal(t, fresh, res.Repositories)
	assert.ErrorIs(t, res.CacheWarning, writeErr)
}

func TestService_LoadRepositories_FetchError(t *testing.T) {
	fetchErr := errors.New("network failure")
	svc := NewService(
		&stubFetcher{
			fetchFn: func(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
				return nil, fetchErr
			},
		},
		&stubStore{
			getFn: func() ([]domain.Repository, error) { return nil, cache.ErrCacheMiss },
			setFn: func(r []domain.Repository) error {
				t.Fatal("Set should not be called on fetch error")
				return nil
			},
		},
		nil,
	)

	_, err := svc.LoadRepositories(context.Background(), false, nil)
	assert.ErrorIs(t, err, fetchErr)
}

func TestService_SearchRepositories(t *testing.T) {
	want := []domain.SearchResult{{Repository: domain.Repository{Name: "hit"}, Score: 1.5}}
	svc := NewService(nil, nil, &stubSearcher{
		searchFn: func(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error) {
			assert.Equal(t, "query", query)
			assert.Equal(t, 5, limit)
			return want, nil
		},
	})

	got, err := svc.SearchRepositories([]domain.Repository{{Name: "hit"}}, "query", 5)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
