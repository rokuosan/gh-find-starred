package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rokuosan/gh-find-starred/internal/github"
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
	fetch        github.FetchingModel
	search       github.SearchModel
	repositories github.Repositories
}

func initialModel(args []string) model {
	// モデルを初期化
	return model{
		fetch:  github.NewDefaultFetchingModel(),
		search: github.NewDefaultSearchModel(args),
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
	case github.FetchCompletedMsg:
		m.search.Repositories = msg.Repositories
		fetchModel, fetchCmd := m.fetch.Update(msg)
		m.fetch = fetchModel.(github.FetchingModel)
		return m, tea.Batch(fetchCmd, m.search.Init())
	case github.FetchMsg:
		fetchModel, fetchCmd := m.fetch.Update(msg)
		m.fetch = fetchModel.(github.FetchingModel)
		return m, fetchCmd
	case github.FetchFailedMsg:
		fetchModel, fetchCmd := m.fetch.Update(msg)
		m.fetch = fetchModel.(github.FetchingModel)
		return m, fetchCmd
	case github.SearchFinishedMsg:
		searchModel, searchCmd := m.search.Update(msg)
		m.search = searchModel.(github.SearchModel)
		return m, searchCmd
	case spinner.TickMsg:
		if m.fetch.Status == github.FetchStatusLoading {
			fetchModel, fetchCmd := m.fetch.Update(msg)
			m.fetch = fetchModel.(github.FetchingModel)
			return m, fetchCmd
		}
		if m.search.Loading {
			searchModel, searchCmd := m.search.Update(msg)
			m.search = searchModel.(github.SearchModel)
			return m, searchCmd
		}
	}

	return m, nil
}

func (m model) View() string {
	return fmt.Sprintf("%s\n%s", m.fetch.View(), m.search.View())
}
