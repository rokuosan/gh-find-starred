package github

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/config"
	gql "github.com/cli/shurcooL-graphql"
)

type Repository struct {
	Name        string `json:"name"`
	Url         string `json:"url"`
	Description string `json:"description"`
	Readme      string `json:"readme"`
}

type GetStarredRepositoriesQuery struct {
	Viewer struct {
		StarredRepositories struct {
			Nodes []struct {
				Name        string
				Url         string
				Description string

				Object struct {
					Blob struct {
						Text string
					} `graphql:"... on Blob"`
				} `graphql:"object(expression: $expression)"`
			}
			PageInfo PageInfo `graphql:"pageInfo"`
		} `graphql:"starredRepositories(after: $after, first: 100)"`
	} `graphql:"viewer"`
	RateLimit RateLimit `graphql:"rateLimit"`
}

func (q GetStarredRepositoriesQuery) Repositories() []Repository {
	repos := make([]Repository, len(q.Viewer.StarredRepositories.Nodes))
	for i, node := range q.Viewer.StarredRepositories.Nodes {
		repos[i] = Repository{
			Name:        node.Name,
			Url:         node.Url,
			Description: node.Description,
			Readme:      node.Object.Blob.Text,
		}
	}
	return repos
}

func (q GetStarredRepositoriesQuery) PageInfo() PageInfo {
	return q.Viewer.StarredRepositories.PageInfo
}

type GetStarredRepositoriesResult struct {
	Repositories []Repository
	PageInfo     PageInfo
}

func GetStarredRepositories(after string) (GetStarredRepositoriesResult, error) {
	c, err := api.DefaultGraphQLClient()
	if err != nil {
		return GetStarredRepositoriesResult{}, err
	}

	var query GetStarredRepositoriesQuery
	err = c.Query("StarredRepositories", &query, map[string]interface{}{
		"expression": gql.String("HEAD:README.md"),
		"after":      gql.String(after),
	})

	if err != nil {
		return GetStarredRepositoriesResult{}, err
	}

	return GetStarredRepositoriesResult{
		Repositories: query.Repositories(),
		PageInfo:     query.PageInfo(),
	}, nil
}

type Repositories []Repository

type SearchRepositoryOptions struct {
	IncludeName        bool
	IncludeDescription bool
	IncludeREADME      bool
}

type SearchRepositoryResult []SearchRepositoryResultItem

type SearchRepositoryResultItem struct {
	Repository Repository
	Point      int
}

func (r Repositories) Search(query string, opt *SearchRepositoryOptions) SearchRepositoryResult {
	var results SearchRepositoryResult
	for _, repo := range r {
		if repo.Name == query {
			r := SearchRepositoryResultItem{
				Repository: repo,
				Point:      100,
			}
			results = append(results, r)
		}
	}
	return results
}

func GetStarredRepositoriesFromCache() ([]Repository, error) {
	path := fmt.Sprintf("%s/starred_repositories.json", config.CacheDir())
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var cacheData struct {
		ExpiresAt string       `json:"expires_at"`
		CreatedAt string       `json:"created_at"`
		Data      []Repository `json:"data"`
	}

	err = decoder.Decode(&cacheData)
	if err != nil {
		return nil, err
	}
	expiresAt, err := time.Parse(time.RFC3339, cacheData.ExpiresAt)
	if err != nil {
		return nil, err
	}

	if time.Now().After(expiresAt) {
		return nil, nil
	}

	respositories := make([]Repository, len(cacheData.Data))
	for i, d := range cacheData.Data {
		respositories[i] = d
	}

	return respositories, nil
}
