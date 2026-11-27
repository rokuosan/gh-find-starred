package search

import (
	"testing"

	"github.com/rokuosan/gh-find-starred/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBleveSearcher_MatchesFieldsAndOrdersByScore(t *testing.T) {
	searcher := NewBleveSearcher()
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
			Readme:      "Includes a bubbletea powered viewer for terminal notes.",
		},
		{
			Name:        "unrelated",
			URL:         "https://github.com/example/unrelated",
			Description: "Database migration utility",
			Readme:      "PostgreSQL and MySQL migrations.",
		},
	}

	results, err := searcher.Search(repos, "bubbletea", 10)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "https://github.com/charmbracelet/bubbletea", results[0].Repository.URL)
	assert.GreaterOrEqual(t, results[0].Score, results[1].Score)
}

func TestBleveSearcher_DuplicateNamesWithDistinctURLs(t *testing.T) {
	searcher := NewBleveSearcher()
	repos := []domain.Repository{
		{
			Name:        "dotfiles",
			URL:         "https://github.com/alice/dotfiles",
			Description: "Neovim and zsh setup",
		},
		{
			Name:        "dotfiles",
			URL:         "https://github.com/bob/dotfiles",
			Description: "Neovim and tmux setup",
		},
	}

	results, err := searcher.Search(repos, "dotfiles", 10)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, []string{
		"https://github.com/alice/dotfiles",
		"https://github.com/bob/dotfiles",
	}, []string{
		results[0].Repository.URL,
		results[1].Repository.URL,
	})
}

func TestBleveSearcher_RespectsLimit(t *testing.T) {
	searcher := NewBleveSearcher()
	repos := []domain.Repository{
		{Name: "go-one", URL: "https://github.com/a/go-one", Description: "golang tool"},
		{Name: "go-two", URL: "https://github.com/a/go-two", Description: "golang library"},
		{Name: "go-three", URL: "https://github.com/a/go-three", Description: "golang helper"},
	}

	results, err := searcher.Search(repos, "golang", 2)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	allResults, err := searcher.Search(repos, "golang", 0)
	require.NoError(t, err)
	assert.Len(t, allResults, 3)
}

func TestBleveSearcher_EmptyInputsAndInvalidQuery(t *testing.T) {
	searcher := NewBleveSearcher()
	repos := []domain.Repository{
		{Name: "alpha", URL: "https://github.com/a/alpha", Description: "alpha"},
	}

	res, err := searcher.Search(nil, "alpha", 10)
	require.NoError(t, err)
	assert.Nil(t, res)

	res, err = searcher.Search(repos, "   ", 10)
	require.NoError(t, err)
	assert.Nil(t, res)

	_, err = searcher.Search(repos, `name:"unclosed`, 10)
	require.Error(t, err)
}
