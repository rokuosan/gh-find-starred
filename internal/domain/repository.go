package domain

import "context"

// Repository represents a starred GitHub repository and its README content.
type Repository struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Readme      string `json:"readme"`
}

// PageInfo holds GraphQL pagination metadata.
type PageInfo struct {
	HasNextPage     bool   `graphql:"hasNextPage" json:"hasNextPage"`
	HasPreviousPage bool   `graphql:"hasPreviousPage" json:"hasPreviousPage"`
	StartCursor     string `graphql:"startCursor" json:"startCursor"`
	EndCursor       string `graphql:"endCursor" json:"endCursor"`
}

// FetchProgress reports incremental fetch progress to callers.
type FetchProgress struct {
	FetchedCount int
}

// IndexStatus describes the prepared repository index state.
type IndexStatus struct {
	RepoCount    int
	FromCache    bool
	CacheWarning error
}

// SearchResult pairs a matched repository with its relevance score.
type SearchResult struct {
	Repository Repository
	Score      float64
}

// IndexSearcher executes ranked repository searches against a prepared index.
type IndexSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}
