package cmd

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/config"
	graphql "github.com/cli/shurcooL-graphql"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gh-find-starred",
	Short: "gh-find-starred is a GitHub CLI extension to find your starred repositories.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Cache: " + config.CacheDir())
		fmt.Println("Config: " + config.ConfigDir())

		repo := GetStarredRepositories("")
		fmt.Printf("Got: %d\n", len(repo))
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {}

type Repository struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	Readme string `json:"readme"`
}

func GetStarredRepositories(after string) []Repository {
	c, err := api.DefaultGraphQLClient()
	if err != nil {
		panic(err)
	}

	// Query
	var query struct {
		Viewer struct {
			StarredRepositories struct {
				Nodes []struct {
					Name string
					Url  string

					Object struct {
						Blob struct {
							Text string
						} `graphql:"... on Blob"`
					} `graphql:"object(expression: $expression)"`
				}
				PageInfo struct {
					HasNextPage bool
					EndCursor   string
				}
			} `graphql:"starredRepositories(after: $after, first: 100)"`
		} `graphql:"viewer"`
		RateLimit struct {
			Cost      int    `graphql:"cost"`
			Limit     int    `graphql:"limit"`
			Remaining int    `graphql:"remaining"`
			ResetAt   string `graphql:"resetAt"`
		} `graphql:"rateLimit"`
	}

	// Variables
	variables := map[string]interface{}{
		"expression": graphql.String("HEAD:README.md"),
		"after":      graphql.String(after),
	}

	// Execute
	err = c.Query("repositories", &query, variables)
	if err != nil {
		panic(err)
	}

	repos := make([]Repository, len(query.Viewer.StarredRepositories.Nodes))
	// Shwo rate limit
	fmt.Printf("Rate limit: %d/%d\n", query.RateLimit.Remaining, query.RateLimit.Limit)
	if !query.Viewer.StarredRepositories.PageInfo.HasNextPage {
		// Return repositories if there are no more pages
		for i, node := range query.Viewer.StarredRepositories.Nodes {
			repos[i] = Repository{
				Name:   node.Name,
				Url:    node.Url,
				Readme: node.Object.Blob.Text,
			}
		}
		return repos
	} else {
		// Recursively get more repositories
		after = query.Viewer.StarredRepositories.PageInfo.EndCursor
		return append(repos, GetStarredRepositories(after)...)
	}
}
