package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rokuosan/gh-find-starred/pkg/api"
	"github.com/rokuosan/gh-find-starred/pkg/search"
)

type SearchMsg struct {
	Result []search.SearchRepositoryResult
}

type SearchModel struct {
	Spinner      spinner.Model
	Repositories api.GitHubRepositories
	Result       []search.SearchRepositoryResult
	SearchQuery  []string
	Loading      bool
}

func NewDefaultSearchModel(query []string) SearchModel {
	s := spinner.New(spinner.WithSpinner(spinner.Points))
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return SearchModel{
		SearchQuery: query,
		Spinner:     s,
		Loading:     true,
	}
}

func (m SearchModel) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		m.Search,
	)
}

func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	case SearchMsg:
		m.Loading = false
		m.Result = msg.Result
		return m, tea.Quit
	}

	return m, nil
}

func (m SearchModel) View() string {
	if m.Loading {
		return fmt.Sprintf("%s Searching...", m.Spinner.View())
	}

	var sb strings.Builder
	for _, r := range m.Result {
		sb.WriteString(fmt.Sprintf("%.1f%% %s %s\n", r.Score*100, r.Repository.Name, r.Repository.Url))
	}
	return sb.String()
}

func (m SearchModel) Search() tea.Msg {
	result := search.BleveSearch(m.Repositories, m.SearchQuery)
	return SearchMsg{Result: result}
}
