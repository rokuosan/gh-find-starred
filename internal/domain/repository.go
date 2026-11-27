package domain

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

// FetchResult contains loaded repositories and metadata about the load source.
type FetchResult struct {
	Repositories []Repository
	FromCache    bool
	CacheWarning error
}

// SearchResult pairs a matched repository with its relevance score.
type SearchResult struct {
	Repository Repository
	Score      float64
}
