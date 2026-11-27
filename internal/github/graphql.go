package github

import "github.com/rokuosan/gh-find-starred/internal/domain"

// RateLimit holds GitHub GraphQL rate-limit metadata.
type RateLimit struct {
	Cost      int    `graphql:"cost" json:"cost"`
	Limit     int    `graphql:"limit" json:"limit"`
	Remaining int    `graphql:"remaining" json:"remaining"`
	ResetAt   string `graphql:"resetAt" json:"resetAt"`
}

// TextObject represents a GitHub GraphQL Blob fragment for README contents.
type TextObject struct {
	Blob struct {
		Text string
	} `graphql:"... on Blob"`
}

// StarredRepository represents a node returned by the starredRepositories GraphQL query.
type StarredRepository struct {
	Name        string `json:"name"`
	Url         string `json:"url"`
	Description string `json:"description"`

	Object TextObject `graphql:"object(expression: $expression)"`
}

type getStarredRepositoriesQuery struct {
	Viewer struct {
		StarredRepositories struct {
			Nodes    []StarredRepository
			PageInfo domain.PageInfo `graphql:"pageInfo"`
		} `graphql:"starredRepositories(after: $after, first: 100)"`
	} `graphql:"viewer"`
	RateLimit RateLimit `graphql:"rateLimit"`
}

func (q *getStarredRepositoriesQuery) Repositories() []domain.Repository {
	repos := make([]domain.Repository, len(q.Viewer.StarredRepositories.Nodes))
	for i, node := range q.Viewer.StarredRepositories.Nodes {
		repos[i] = domain.Repository{
			Name:        node.Name,
			URL:         node.Url,
			Description: node.Description,
			Readme:      node.Object.Blob.Text,
		}
	}
	return repos
}

func (q *getStarredRepositoriesQuery) PageInfo() domain.PageInfo {
	return q.Viewer.StarredRepositories.PageInfo
}
