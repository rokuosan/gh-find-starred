package app

import (
	"context"
	"errors"

	"github.com/rokuosan/gh-find-starred/internal/domain"
)

// RepositoryFetcher fetches starred repositories from an upstream source.
type RepositoryFetcher interface {
	FetchStarredRepositories(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error)
}

// IndexStore manages loading, persisting, and falling back for repository search indexes.
type IndexStore interface {
	Load(ctx context.Context) (domain.IndexSearcher, int, error)
	Save(ctx context.Context, repos []domain.Repository) (domain.IndexSearcher, error)
	Fallback(repos []domain.Repository) domain.IndexSearcher
}

// Service coordinates index loading, GitHub fetching, and search persistence without UI dependencies.
type Service struct {
	fetcher    RepositoryFetcher
	indexStore IndexStore
}

// NewService constructs an application Service.
func NewService(fetcher RepositoryFetcher, indexStore IndexStore) *Service {
	return &Service{
		fetcher:    fetcher,
		indexStore: indexStore,
	}
}

// Prepare returns a ready IndexSearcher, loading from the persistent index cache when
// valid (unless noCache is true) or fetching from GitHub and rebuilding the index.
func (s *Service) Prepare(ctx context.Context, noCache bool, onProgress func(domain.FetchProgress)) (domain.IndexStatus, domain.IndexSearcher, error) {
	if !noCache && s.indexStore != nil {
		searcher, count, err := s.indexStore.Load(ctx)
		if err == nil {
			return domain.IndexStatus{
				RepoCount: count,
				FromCache: true,
			}, searcher, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return domain.IndexStatus{}, nil, err
		}
	}

	if s.fetcher == nil {
		return domain.IndexStatus{}, nil, errors.New("repository fetcher is not configured")
	}

	repos, err := s.fetcher.FetchStarredRepositories(ctx, onProgress)
	if err != nil {
		return domain.IndexStatus{}, nil, err
	}

	status := domain.IndexStatus{
		RepoCount: len(repos),
	}
	if s.indexStore == nil {
		return status, nil, errors.New("index store is not configured")
	}

	searcher, err := s.indexStore.Save(ctx, repos)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return domain.IndexStatus{}, nil, err
		}
		status.CacheWarning = err
		return status, s.indexStore.Fallback(repos), nil
	}

	return status, searcher, nil
}
