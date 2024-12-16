package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
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

func (s SearchRepositoryResult) Repositories() Repositories {
	repos := make([]Repository, len(s))
	for i, item := range s {
		repos[i] = item.Repository
	}
	return repos
}

type SearchRepositoryResultItem struct {
	Repository Repository
	Point      float64
}

// SearchByBleve は、bleveを使ってリポジトリを検索する
// Repositories.Search と同じように検索文字列に一致するリポジトリを返すが、
// bleveを使って検索を行う。
// 計算やポイントなどは全て異なるため、Repositories.Search とは別のメソッドとしている
func (r Repositories) SearchByBleve(query []string, opt *SearchRepositoryOptions) SearchRepositoryResult {
	mapping := bleve.NewIndexMapping()
	index, err := bleve.NewMemOnly(mapping)
	if err != nil {
		panic(err)
	}

	nameToRepo := make(map[string]Repository)
	for _, repo := range r {
		nameToRepo[repo.Name] = repo
		index.Index(repo.Name, repo)
	}

	var results SearchRepositoryResult
	q := bleve.NewQueryStringQuery(strings.Join(query, " "))
	searchRequest := bleve.NewSearchRequest(q)
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		panic(err)
	}

	for _, hit := range searchResult.Hits {
		repo := nameToRepo[hit.ID]
		point := hit.Score

		results = append(results, SearchRepositoryResultItem{
			Repository: repo,
			Point:      point,
		})
	}

	// ポイントの高い順にソート
	sort.Slice(results, func(i, j int) bool {
		return results[i].Point > results[j].Point
	})

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
		return nil, errors.New("cache expired")
	}

	respositories := make([]Repository, len(cacheData.Data))
	for i, d := range cacheData.Data {
		respositories[i] = d
	}

	return respositories, nil
}
