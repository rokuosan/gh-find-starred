package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

type RepositoryListItem github.Repository

func (i RepositoryListItem) Title() string       { return i.Name }
func (i RepositoryListItem) FilterValue() string { return i.Name }

type errMsg error

type FetchRepositoryMessage struct {
	Result     github.GetStarredRepositoriesResult
	IsCacheHit bool
}

type SearchMsg struct {
	Result github.SearchRepositoryResult
}

type model struct {
	spinner      spinner.Model
	quitting     bool
	err          error
	loading      bool
	repos        []github.Repository
	cursor       string
	isCacheHit   bool
	words        []string
	searching    bool
	searchResult github.SearchRepositoryResult
}

func initialModel(args []string) model {
	// スピナーを初期化
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// モデルを初期化
	return model{
		spinner: s,
		loading: true,
		words:   args,
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
	if len(m.repos) == 0 {
		if repos, err := github.GetStarredRepositoriesFromCache(); err == nil {
			return FetchRepositoryMessage{
				Result: github.GetStarredRepositoriesResult{
					Repositories: repos,
					PageInfo:     github.PageInfo{},
				},
				IsCacheHit: true,
			}
		}
	}

	// 現在のカーソルからリポジトリを取得する
	result, err := github.GetStarredRepositories(m.cursor)
	if err != nil {
		return errMsg(err)
	}
	// 取得したリポジトリを返す
	return FetchRepositoryMessage{
		Result: result,
	}
}

func (m model) SearchRepositories() tea.Msg {
	// 検索結果を返す
	repositories := github.Repositories(m.repos)
	result := repositories.Search(m.words, &github.SearchRepositoryOptions{
		IncludeName:        true,
		IncludeDescription: true,
		IncludeREADME:      true,
	})
	return SearchMsg{Result: result}
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
		var cmd tea.Cmd = nil
		m.repos = append(m.repos, msg.Result.Repositories...)
		m.cursor = msg.Result.PageInfo.EndCursor
		if msg.IsCacheHit {
			m.isCacheHit = true
		}

		if msg.Result.PageInfo.HasNextPage {
			cmd = m.GetRepositoriesFromGitHub
		} else {
			m.loading = false
			if !m.isCacheHit {
				path := fmt.Sprintf("%s/starred_repositories.json", config.CacheDir())
				if err := Cache(path, m.repos); err != nil {
					return m, func() tea.Msg { return errMsg(err) }
				}
			}
		}

		// 検索を実行
		if cmd == nil {
			cmd = m.SearchRepositories
			m.searching = true
		}

		return m, cmd

	case SearchMsg:
		m.searching = false
		m.searchResult = msg.Result
		return m, tea.Quit

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
	if m.searching {
		return fmt.Sprintf("%s Searching...\n", m.spinner.View())
	}

	var b strings.Builder
	for _, r := range m.searchResult {
		b.WriteString(fmt.Sprintf("%3d %s(%s)\n", r.Point, r.Repository.Name, r.Repository.Url))
	}
	return b.String()
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
