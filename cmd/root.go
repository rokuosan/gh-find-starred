package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/cli/go-gh/v2/pkg/config"
	"github.com/rokuosan/gh-find-starred/internal/app"
	"github.com/rokuosan/gh-find-starred/internal/cache"
	"github.com/rokuosan/gh-find-starred/internal/domain"
	"github.com/rokuosan/gh-find-starred/internal/github"
	"github.com/rokuosan/gh-find-starred/internal/search"
	"github.com/rokuosan/gh-find-starred/internal/ui"
	"github.com/spf13/cobra"
)

const defaultCacheTTL = 24 * time.Hour

var errDisplayed = errors.New("error already displayed by TUI")

func newRootCmd() *cobra.Command {
	var (
		limit   int
		noCache bool
	)

	rootCmd := &cobra.Command{
		Use:           "gh-find-starred [flags] <words...>",
		Short:         "gh-find-starred is a GitHub CLI extension to find your starred repositories.",
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 {
				return fmt.Errorf("--limit must be at least 1 (got %d)", limit)
			}
			cmd.SilenceUsage = true

			client, err := github.NewDefaultClient()
			if err != nil {
				return err
			}

			cachePath := filepath.Join(config.CacheDir(), "gh-find-starred", "starred_repositories.json")
			store := cache.NewFileStore[[]domain.Repository](cachePath, defaultCacheTTL)
			searcher := search.NewBleveSearcher()
			svc := app.NewService(client, store, searcher)

			model := ui.NewModel(cmd.Context(), svc, ui.Options{
				Query:   strings.Join(args, " "),
				Limit:   limit,
				NoCache: noCache,
			})

			finalModel, err := tea.NewProgram(model, tea.WithContext(cmd.Context())).Run()
			if err != nil {
				return err
			}
			if fm, ok := finalModel.(ui.Model); ok && fm.Err() != nil {
				return errDisplayed
			}

			return nil
		},
	}

	rootCmd.Flags().IntVarP(&limit, "limit", "l", 10, "Limit the number of search results displayed")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "Bypass the local repository cache and fetch from GitHub")

	return rootCmd
}

// Execute runs the root CLI command.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		if !errors.Is(err, errDisplayed) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
