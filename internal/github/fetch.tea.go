package github

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/go-gh/v2/pkg/config"
)

type FetchingModel struct {
	Spinner      spinner.Model
	Repositories Repositories
	Status       FetchStatus
	cursor       string
}

type FetchStatus int

const (
	FetchStatusFailed FetchStatus = iota - 1
	FetchStatusLoading
	FetchStatusCompleted
)

type FetchMsg struct {
	Repositories Repositories
	PageInfo     PageInfo
}

type FetchFailedMsg struct {
	Err error
}

type FetchCompletedMsg struct {
	Repositories Repositories
}

func NewDefaultFetchingModel() FetchingModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return FetchingModel{
		Spinner:      s,
		Repositories: Repositories([]Repository{}),
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
		return m.onTick(msg)
	case FetchMsg:
		return m.onFetch(msg)
	case FetchFailedMsg:
		return m.onFailed(msg)
	case FetchCompletedMsg:
		return m.onComplete(msg)
	default:
		return m, nil
	}
}

func (m FetchingModel) View() string {
	switch m.Status {
	case FetchStatusFailed:
		return "Failed to fetch starred repositories"
	case FetchStatusLoading:
		return fmt.Sprintf("%s Fetching starred repositories: %d", m.Spinner.View(), len(m.Repositories))
	}

	return fmt.Sprintf("✓ Fetched starred repositories: %d", len(m.Repositories))
}

func (m FetchingModel) onTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.Spinner, cmd = m.Spinner.Update(msg)
	return m, cmd
}

func (m FetchingModel) onFetch(msg FetchMsg) (tea.Model, tea.Cmd) {
	m.Repositories = append(m.Repositories, msg.Repositories...)
	m.cursor = msg.PageInfo.EndCursor
	m.Status = FetchStatusLoading

	if !msg.PageInfo.HasNextPage {
		return m, func() tea.Msg { return FetchCompletedMsg{Repositories: m.Repositories} }
	}

	return m, m.GetRepositoriesFromGitHub
}

func (m FetchingModel) onFailed(_ FetchFailedMsg) (tea.Model, tea.Cmd) {
	m.Spinner.Spinner = spinner.Spinner{}
	m.Status = FetchStatusFailed
	return m, tea.Quit
}

func (m FetchingModel) onComplete(msg FetchCompletedMsg) (tea.Model, tea.Cmd) {
	// キャッシュに保存
	path := fmt.Sprintf("%s/starred_repositories.json", config.CacheDir())
	if err := Cache(path, msg.Repositories); err != nil {
		return m, func() tea.Msg { return FetchFailedMsg{Err: err} }
	}

	m.Repositories = msg.Repositories
	m.Status = FetchStatusCompleted

	return m, nil
}

func (m FetchingModel) GetRepositoriesFromGitHub() tea.Msg {
	// 初回実行のみキャッシュを取得してみる
	if len(m.Repositories) == 0 {
		if repos, err := GetStarredRepositoriesFromCache(); err == nil {
			return FetchCompletedMsg{
				Repositories: repos,
			}
		}
	}

	// 現在のカーソルからリポジトリを取得する
	result, err := GetStarredRepositories(m.cursor)
	if err != nil {
		return FetchFailedMsg{Err: err}
	}
	// 取得したリポジトリを返す
	return FetchMsg{
		Repositories: result.Repositories,
		PageInfo:     result.PageInfo,
	}
}
