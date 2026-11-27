package search

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/rokuosan/gh-find-starred/internal/domain"
)

// Searcher queries a slice of repositories and returns ranked matches.
type Searcher interface {
	Search(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error)
}

// BleveSearcher performs in-memory full-text search using Bleve.
type BleveSearcher struct{}

// NewBleveSearcher creates a new BleveSearcher.
func NewBleveSearcher() *BleveSearcher {
	return &BleveSearcher{}
}

// Search indexes repos in memory and executes a Bleve query-string search.
// A limit <= 0 returns all matches.
func (s *BleveSearcher) Search(repos []domain.Repository, query string, limit int) ([]domain.SearchResult, error) {
	if len(repos) == 0 || strings.TrimSpace(query) == "" {
		return nil, nil
	}

	mapping := bleve.NewIndexMapping()
	index, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return nil, fmt.Errorf("create bleve index: %w", err)
	}
	defer index.Close()

	idToRepo := make(map[string]domain.Repository, len(repos))
	for i, repo := range repos {
		id := repo.URL
		if id == "" {
			id = fmt.Sprintf("%s#%d", repo.Name, i)
		}
		idToRepo[id] = repo
		if err := index.Index(id, repo); err != nil {
			return nil, fmt.Errorf("index repository %q: %w", repo.Name, err)
		}
	}

	q := bleve.NewQueryStringQuery(query)
	req := bleve.NewSearchRequest(q)
	if limit > 0 {
		req.Size = limit
	} else {
		req.Size = len(repos)
	}

	res, err := index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("execute search: %w", err)
	}

	results := make([]domain.SearchResult, 0, len(res.Hits))
	for _, hit := range res.Hits {
		repo, ok := idToRepo[hit.ID]
		if !ok {
			continue
		}
		results = append(results, domain.SearchResult{
			Repository: repo,
			Score:      hit.Score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Repository.Name != results[j].Repository.Name {
			return results[i].Repository.Name < results[j].Repository.Name
		}
		return results[i].Repository.URL < results[j].Repository.URL
	})

	return results, nil
}
