package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rokuosan/gh-find-starred/pkg/api"
	"github.com/rokuosan/gh-find-starred/pkg/ui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gh-find-starred",
	Short: "gh-find-starred is a GitHub CLI extension to find your starred repositories.",
	Long:  ``,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		p := tea.NewProgram(initialModel(args))
		if _, err := p.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {}

type model struct {
	fetch        ui.FetchingModel
	search       ui.SearchModel
	repositories api.GitHubRepositories
}

func initialModel(args []string) model {
	// モデルを初期化
	return model{
		fetch:  ui.NewDefaultFetchingModel(),
		search: ui.NewDefaultSearchModel(args),
	}
}

func (m model) Init() tea.Cmd {
	return m.fetch.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	case ui.FetchMsg:
		fetchModel, fetchCmd := m.fetch.Update(msg)
		m.fetch = fetchModel.(ui.FetchingModel)
		if m.fetch.Status == ui.FetchStatusCompleted {
			m.search.Repositories = m.fetch.Repositories
			return m, m.search.Init()
		}
		return m, fetchCmd
	case ui.SearchMsg:
		searchModel, searchCmd := m.search.Update(msg)
		m.search = searchModel.(ui.SearchModel)
		return m, searchCmd
	case spinner.TickMsg:
		if m.fetch.Status == ui.FetchStatusLoading {
			fetchModel, fetchCmd := m.fetch.Update(msg)
			m.fetch = fetchModel.(ui.FetchingModel)
			return m, fetchCmd
		}
		if m.search.Loading {
			searchModel, searchCmd := m.search.Update(msg)
			m.search = searchModel.(ui.SearchModel)
			return m, searchCmd
		}
	}

	return m, nil
}

func (m model) View() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n", m.fetch.View()))
	if m.fetch.Status == ui.FetchStatusCompleted {
		sb.WriteString(fmt.Sprintf("%s\n", m.search.View()))
	}
	return sb.String()
}
