package github

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SearchModel struct {
	Spinner      spinner.Model
	Repositories Repositories
	Result       SearchRepositoryResult
	SearchQuery  []string
	Loading      bool
}

type SearchFinishedMsg struct {
	Result SearchRepositoryResult
}

func NewDefaultSearchModel(query []string) SearchModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
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
	case SearchFinishedMsg:
		m.Result = msg.Result
		m.Loading = false
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
		sb.WriteString(fmt.Sprintf("%f %s %s\n", r.Point, r.Repository.Name, r.Repository.Url))
	}
	return sb.String()

}

func (m SearchModel) Search() tea.Msg {
	result := m.Repositories.SearchByBleve(m.SearchQuery, &SearchRepositoryOptions{
		IncludeName:        true,
		IncludeDescription: true,
		IncludeREADME:      true,
	})
	return SearchFinishedMsg{Result: result}
}
