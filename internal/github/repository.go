package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

func (r Repositories) Search(query []string, opt *SearchRepositoryOptions) SearchRepositoryResult {
	// TODO: implement search logic
	// 動作のために仮実装しておく
	// 検索文字列の一致度に応じてポイントをつける
	// 一文字あたり1ポイントを基準として、以下の条件でポイントを加算する
	// - リポジトリ名に一致した場合: 10ポイント
	// - リポジトリの説明文に一致した場合: 3ポイント
	// - READMEに一致した場合: 1ポイント
	// つまり、「example」という検索文字列があった場合、
	// リポジトリ名に「example」が完全一致した場合は 7x10 = 70ポイント
	// リポジトリの説明文に「example」が含まれる場合は 7x3 = 21ポイント
	// READMEに「example」が含まれる場合は 7x1 = 7ポイント
	// というようにポイントを計算する
	// ただし、説明とREADMEの文字列については、その単語が複数回ある場合も考慮する
	// 例えば、説明文に「example example example」という文字列がある場合、
	// それは3回「example」という単語が含まれているとして、3x7x3=63ポイントとなる

	var results SearchRepositoryResult
	for _, repo := range r {
		var point int
		for _, q := range query {
			if opt.IncludeName {
				if repo.Name == q {
					// リポジトリ名が完全一致するか
					point += len(repo.Name) * 10
				} else {
					// リポジトリ名に検索文字列が含まれるか
					for i := 0; i < len(repo.Name)-len(q)+1; i++ {
						if repo.Name[i:i+len(q)] == q {
							point += len(q) * 7
						}
					}
				}
			}
			if opt.IncludeDescription {
				// 説明文に検索文字列が含まれるか
				for i := 0; i < len(repo.Description)-len(q)+1; i++ {
					if repo.Description[i:i+len(q)] == q {
						point += len(q) * 3
					}
				}
			}
			if opt.IncludeREADME {
				// READMEに検索文字列が含まれるか
				for i := 0; i < len(repo.Readme)-len(q)+1; i++ {
					if repo.Readme[i:i+len(q)] == q {
						point += len(q) * 1
					}
				}
			}
		}
		if point > 0 {
			results = append(results, SearchRepositoryResultItem{
				Repository: repo,
				Point:      float64(point),
			})
		}
	}

	// ポイントの高い順にソート
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Point < results[j].Point {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
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
	for _, q := range query {
		q := bleve.NewQueryStringQuery(q)
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
	}

	// ポイントの高い順にソート
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Point < results[j].Point {
				results[i], results[j] = results[j], results[i]
			}
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
		return nil, errors.New("cache expired")
	}

	respositories := make([]Repository, len(cacheData.Data))
	for i, d := range cacheData.Data {
		respositories[i] = d
	}

	return respositories, nil
}
