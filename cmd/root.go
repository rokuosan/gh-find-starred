package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/go-gh/v2/pkg/config"
	"github.com/rokuosan/gh-find-starred/internal/github"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gh-find-starred",
	Short: "gh-find-starred is a GitHub CLI extension to find your starred repositories.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		p := tea.NewProgram(initialModel())
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

type RepositoryListItem github.Repository

func (i RepositoryListItem) Title() string       { return i.Name }
func (i RepositoryListItem) FilterValue() string { return i.Name }

type errMsg error

type FetchRepositoryMessage github.GetStarredRepositoriesResult

type model struct {
	spinner  spinner.Model
	quitting bool
	err      error
	loading  bool
	repos    []github.Repository
	cursor   string
}

func initialModel() model {
	// スピナーを初期化
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// モデルを初期化
	return model{
		spinner: s,
		loading: true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.GetRepositoriesFromGitHub,
	)
}

func (m model) GetRepositoriesFromGitHub() tea.Msg {
	// キャッシュを取得
	dir := config.CacheDir()
	path := fmt.Sprintf("%s/starred_repositories.json", dir)
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		var cacheData struct {
			ExpiresAt string              `json:"expires_at"`
			CreatedAt string              `json:"created_at"`
			Data      []github.Repository `json:"data"`
		}
		if err := json.NewDecoder(f).Decode(&cacheData); err == nil {
			// キャッシュが有効であればリポジトリを返す
			if expiresAt, err := time.Parse(time.RFC3339, cacheData.ExpiresAt); err == nil {
				if time.Now().Before(expiresAt) {
					repos := make([]github.Repository, len(cacheData.Data))
					for i, d := range cacheData.Data {
						repos[i] = d
					}
					return FetchRepositoryMessage(github.GetStarredRepositoriesResult{
						Repositories: repos,
						PageInfo:     github.PageInfo{HasNextPage: false},
					})
				}
			}
		}
	}

	// 現在のカーソルからリポジトリを取得する
	result, err := github.GetStarredRepositories(m.cursor)
	if err != nil {
		return errMsg(err)
	}
	// 取得したリポジトリを返す
	return FetchRepositoryMessage(result)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		}

	case FetchRepositoryMessage:
		var cmd tea.Cmd
		m.repos = append(m.repos, msg.Repositories...)
		m.cursor = msg.PageInfo.EndCursor

		if msg.PageInfo.HasNextPage {
			cmd = m.GetRepositoriesFromGitHub
		}

		return m, cmd

	case errMsg:
		m.err = msg
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	}

	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	if m.quitting {
		return "\n"
	}
	if m.loading {
		count := len(m.repos)
		str := fmt.Sprintf("%s Collecting your starred repositories: %d\n", m.spinner.View(), count)
		return str
	}

	return ""
}

type CacheData struct {
	ExpiresAt string        `json:"expires_at"`
	CreatedAt string        `json:"created_at"`
	Data      []interface{} `json:"data"`
}

func Cache(path string, data []github.Repository) error {
	now := time.Now().Format(time.RFC3339)
	expiresAt := time.Now().Add(time.Hour * 24).Format(time.RFC3339)
	cacheData := CacheData{
		ExpiresAt: expiresAt,
		CreatedAt: now,
		Data:      make([]interface{}, len(data)),
	}
	for i, d := range data {
		cacheData.Data[i] = d
	}

	// ファイルを作成
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// ファイルに書き込む
	enc := json.NewEncoder(f)
	if err := enc.Encode(cacheData); err != nil {
		return err
	}

	return nil
}
