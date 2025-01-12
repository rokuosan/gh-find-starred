package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/go-gh/v2/pkg/config"
	"github.com/rokuosan/gh-find-starred/internal/cache"
	"github.com/rokuosan/gh-find-starred/pkg/api"
)

type FetchStatus int

const (
	FetchStatusFailed FetchStatus = iota - 1
	FetchStatusLoading
	FetchStatusCompleted
)

type FetchingModel struct {
	Spinner           spinner.Model
	Status            FetchStatus
	Repositories      api.GitHubRepositories
	repositoryService api.RepositoryService
	err               error
	cursor            string
	fromCache         bool
}

type FetchMsg struct {
	Repositories api.GitHubRepositories
	PageInfo     api.PageInfo
	Err          error
	fromCache    bool
}

func NewDefaultFetchingModel() FetchingModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return FetchingModel{
		Spinner:           s,
		Repositories:      api.GitHubRepositories{},
		repositoryService: api.NewRepositoryService(),
		Status:            FetchStatusLoading,
	}
}

func (m FetchingModel) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		m.GetRepositoriesFromGitHub,
	)
}

func (m FetchingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.Status == FetchStatusLoading {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case FetchMsg:
		if msg.Err != nil {
			m.Status = FetchStatusFailed
			m.err = msg.Err
			return m, tea.Quit
		}
		m.Repositories = append(m.Repositories, msg.Repositories...)
		m.cursor = msg.PageInfo.EndCursor
		if msg.PageInfo.HasNextPage {
			return m, m.GetRepositoriesFromGitHub
		}
		// 全てのリポジトリを取得した場合は完了
		m.Status = FetchStatusCompleted
		// 取得したリポジトリをキャッシュする
		m.fromCache = msg.fromCache
		if !msg.fromCache {
			path := fmt.Sprintf("%s/starred_repositories.json", config.CacheDir())
			if err := cache.Cache(path, m.Repositories); err != nil {
				m.Status = FetchStatusFailed
				m.err = err
				return m, tea.Quit
			}
		}

		return m, nil

	default:
		return m, nil
	}
}

func (m FetchingModel) View() string {
	switch m.Status {
	case FetchStatusFailed:
		return fmt.Sprintf("✗ Failed to fetch starred repositories:\nError:\n%v", m.err)
	case FetchStatusLoading:
		return fmt.Sprintf("%s Fetching starred repositories: %d", m.Spinner.View(), len(m.Repositories))
	}

	msg := fmt.Sprintf("✓ Fetched starred repositories: %d", len(m.Repositories))
	if m.fromCache {
		msg += " (from cache)"
	}
	return msg
}

func (m FetchingModel) GetRepositoriesFromGitHub() tea.Msg {
	// 初回実行のみキャッシュを取得してみる
	if len(m.Repositories) == 0 {
		if repos, err := api.GetStarredRepositoriesFromCache(); err == nil {
			return FetchMsg{
				Repositories: repos,
				fromCache:    true,
			}
		}
	}

	// 現在のカーソルからリポジトリを取得する
	result, err := m.repositoryService.FindStarredRepositories(m.cursor)
	if err != nil {
		return FetchMsg{Err: err}
	}
	// 取得したリポジトリを返す
	return FetchMsg{
		Repositories: result.Repositories,
		PageInfo:     result.PageInfo,
	}
}
