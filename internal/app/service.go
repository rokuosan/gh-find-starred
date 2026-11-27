package app

import (
	"context"
	"errors"

	"github.com/rokuosan/gh-find-starred/internal/cache"
	"github.com/rokuosan/gh-find-starred/internal/domain"
)

// RepositoryFetcher fetches starred repositories from an upstream source.
type RepositoryFetcher interface {
	FetchStarredRepositories(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error)
}

// RepositorySearcher searches repositories for a given query string.
type RepositorySearcher interface {
	Search(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error)
}

// Service coordinates repository loading, caching, and searching without any UI dependencies.
type Service struct {
	fetcher  RepositoryFetcher
	store    cache.Store[[]domain.Repository]
	searcher RepositorySearcher
}

// NewService constructs an application Service.
func NewService(fetcher RepositoryFetcher, store cache.Store[[]domain.Repository], searcher RepositorySearcher) *Service {
	return &Service{
		fetcher:  fetcher,
		store:    store,
		searcher: searcher,
	}
}

// LoadRepositories loads starred repositories from cache when fresh (unless noCache is true),
// falling back to fetching from GitHub and persisting the updated cache.
func (s *Service) LoadRepositories(ctx context.Context, noCache bool, onProgress func(domain.FetchProgress)) (domain.FetchResult, error) {
	if !noCache && s.store != nil {
		if repos, err := s.store.Get(); err == nil {
			return domain.FetchResult{
				Repositories: repos,
				FromCache:    true,
			}, nil
		}
	}

	if s.fetcher == nil {
		return domain.FetchResult{}, errors.New("repository fetcher is not configured")
	}

	repos, err := s.fetcher.FetchStarredRepositories(ctx, onProgress)
	if err != nil {
		return domain.FetchResult{}, err
	}

	result := domain.FetchResult{
		Repositories: repos,
	}
	if s.store != nil {
		if err := s.store.Set(repos); err != nil {
			result.CacheWarning = err
		}
	}

	return result, nil
}

// SearchRepositories searches the provided repositories and limits the number of results.
func (s *Service) SearchRepositories(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error) {
	if s.searcher == nil {
		return nil, errors.New("repository searcher is not configured")
	}
	return s.searcher.Search(repos, query, limit)
}
