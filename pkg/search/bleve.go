package search

import (
	"sort"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/rokuosan/gh-find-starred/pkg/api"
)

type SearchRepositoryResult struct {
	Repository api.GitHubRepository
	Score      float64
}

func BleveSearch(repositories api.GitHubRepositories, query []string) []SearchRepositoryResult {
	mapping := bleve.NewIndexMapping()
	index, err := bleve.NewMemOnly(mapping)
	if err != nil {
		panic(err)
	}

	nameToRepo := make(map[string]api.GitHubRepository)
	for _, repo := range repositories {
		nameToRepo[repo.Name] = repo
		index.Index(repo.Name, repo)
	}

	var results []SearchRepositoryResult
	q := bleve.NewQueryStringQuery(strings.Join(query, " "))
	searchRequest := bleve.NewSearchRequest(q)
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		panic(err)
	}

	for _, hit := range searchResult.Hits {
		results = append(results, SearchRepositoryResult{
			Repository: nameToRepo[hit.ID],
			Score:      hit.Score,
		})
	}

	// ポイントの高い順にソート
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}
