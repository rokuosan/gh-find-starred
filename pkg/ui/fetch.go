package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
}

type FetchMsg struct {
	Repositories api.GitHubRepositories
	PageInfo     api.PageInfo
	Err          error
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
			return m, tea.Quit
		}
		m.Repositories = append(m.Repositories, msg.Repositories...)
		m.cursor = msg.PageInfo.EndCursor
		if msg.PageInfo.HasNextPage {
			return m, m.GetRepositoriesFromGitHub
		}
		// 全てのリポジトリを取得した場合は完了
		m.Status = FetchStatusCompleted
		return m, nil

	default:
		return m, nil
	}
}

func (m FetchingModel) View() string {
	switch m.Status {
	case FetchStatusFailed:
		return fmt.Sprintf("✗ Failed to fetch starred repositories: %s\n\nError:\n%v", m.Spinner.View(), m.err)
	case FetchStatusLoading:
		return fmt.Sprintf("%s Fetching starred repositories: %d", m.Spinner.View(), len(m.Repositories))
	}

	return fmt.Sprintf("✓ Fetched starred repositories: %d", len(m.Repositories))
}

func (m FetchingModel) GetRepositoriesFromGitHub() tea.Msg {
	// 初回実行のみキャッシュを取得してみる
	if len(m.Repositories) == 0 {
		if repos, err := api.GetStarredRepositoriesFromCache(); err == nil {
			return FetchMsg{
				Repositories: repos,
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
