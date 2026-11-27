package ui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/rokuosan/gh-find-starred/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubService struct {
	loadFn   func(ctx context.Context, noCache bool, onProgress func(domain.FetchProgress)) (domain.FetchResult, error)
	searchFn func(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error)
}

func (s *stubService) LoadRepositories(ctx context.Context, noCache bool, onProgress func(domain.FetchProgress)) (domain.FetchResult, error) {
	return s.loadFn(ctx, noCache, onProgress)
}

func (s *stubService) SearchRepositories(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error) {
	return s.searchFn(repos, query, limit)
}

func TestModel_FullLifecycle_ProgressAndResults(t *testing.T) {
	repos := []domain.Repository{
		{
			Name:        "bubbletea",
			URL:         "https://github.com/charmbracelet/bubbletea",
			Description: "A TUI\nframework",
		},
	}

	svc := &stubService{
		loadFn: func(ctx context.Context, noCache bool, onProgress func(domain.FetchProgress)) (domain.FetchResult, error) {
			assert.False(t, noCache)
			onProgress(domain.FetchProgress{FetchedCount: 1})
			return domain.FetchResult{Repositories: repos}, nil
		},
		searchFn: func(gotRepos []domain.Repository, query string, limit int) ([]domain.SearchResult, error) {
			assert.Equal(t, repos, gotRepos)
			assert.Equal(t, "tui", query)
			assert.Equal(t, 10, limit)
			return []domain.SearchResult{
				{Repository: repos[0], Score: 1.25},
			}, nil
		},
	}

	m := NewModel(context.Background(), svc, Options{Query: "tui", Limit: 10})
	assert.Contains(t, m.View().Content, "Fetching starred repositories: 0")

	// Start loader goroutine and consume progress + done messages.
	_ = m.startLoadCmd()()

	msg1 := waitForEvent(m.ctx, m.events)()
	next, cmd1 := m.Update(msg1)
	m = next.(Model)
	require.NotNil(t, cmd1)
	assert.Equal(t, StateFetching, m.State())
	assert.Contains(t, m.View().Content, "Fetching starred repositories: 1")

	msg2 := cmd1()
	next, cmd2 := m.Update(msg2)
	m = next.(Model)
	require.NotNil(t, cmd2)
	assert.Equal(t, StateSearching, m.State())
	assert.Contains(t, m.View().Content, "Searching...")
	assert.NotContains(t, m.View().Content, "(error)")

	// Execute searchCmd from the batch via searchCmd directly.
	searchMsg := m.searchCmd(repos)()
	next, quitCmd := m.Update(searchMsg)
	m = next.(Model)
	assert.Equal(t, StateDone, m.State())
	require.NotNil(t, quitCmd)
	assert.IsType(t, tea.QuitMsg{}, quitCmd())

	view := m.View().Content
	assert.Contains(t, view, "Fetched starred repositories: 1")
	assert.Contains(t, view, "bubbletea")
	assert.Contains(t, view, "https://github.com/charmbracelet/bubbletea")
	assert.Contains(t, view, "A TUI framework")
	assert.Len(t, m.Results(), 1)
}

func TestModel_CachedAndWarningAndEmptyMatch(t *testing.T) {
	warnErr := errors.New("permission denied")
	svc := &stubService{
		loadFn: func(ctx context.Context, noCache bool, onProgress func(domain.FetchProgress)) (domain.FetchResult, error) {
			return domain.FetchResult{
				Repositories: []domain.Repository{{Name: "repo"}},
				FromCache:    true,
				CacheWarning: warnErr,
			}, nil
		},
		searchFn: func(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error) {
			return nil, nil
		},
	}

	m := NewModel(context.Background(), svc, Options{Query: "missing", Limit: 5})
	_ = m.startLoadCmd()()

	next, _ := m.Update(waitForEvent(m.ctx, m.events)())
	m = next.(Model)
	next, _ = m.Update(m.searchCmd(m.fetchResult.Repositories)())
	m = next.(Model)

	assert.Equal(t, StateDone, m.State())
	view := m.View().Content
	assert.Contains(t, view, "(from cache)")
	assert.Contains(t, view, "permission denied")
	assert.Contains(t, view, `No repositories matched "missing".`)
}

func TestModel_FetchAndSearchErrors(t *testing.T) {
	t.Run("fetch error", func(t *testing.T) {
		wantErr := errors.New("graphql unavailable")
		m := NewModel(context.Background(), &stubService{}, Options{Query: "go"})

		next, quitCmd := m.Update(fetchDoneMsg{err: wantErr})
		m = next.(Model)
		assert.Equal(t, StateError, m.State())
		assert.ErrorIs(t, m.Err(), wantErr)
		assert.Contains(t, m.View().Content, "graphql unavailable")
		require.NotNil(t, quitCmd)
		assert.IsType(t, tea.QuitMsg{}, quitCmd())
	})

	t.Run("search error", func(t *testing.T) {
		wantErr := errors.New("invalid query syntax")
		m := NewModel(context.Background(), &stubService{}, Options{Query: "bad"})

		next, quitCmd := m.Update(searchDoneMsg{err: wantErr})
		m = next.(Model)
		assert.Equal(t, StateError, m.State())
		assert.ErrorIs(t, m.Err(), wantErr)
		assert.Contains(t, m.View().Content, "invalid query syntax")
		require.NotNil(t, quitCmd)
		assert.IsType(t, tea.QuitMsg{}, quitCmd())
	})
}

func TestModel_CancelOnKeyPress(t *testing.T) {
	m := NewModel(context.Background(), &stubService{}, Options{Query: "go"})
	next, quitCmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = next.(Model)

	assert.Equal(t, StateCanceled, m.State())
	assert.True(t, m.Canceled())
	assert.ErrorIs(t, m.ctx.Err(), context.Canceled)
	assert.Contains(t, m.View().Content, "Canceled.")
	require.NotNil(t, quitCmd)
	assert.IsType(t, tea.QuitMsg{}, quitCmd())
}
