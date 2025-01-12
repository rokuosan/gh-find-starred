package api

import "github.com/cli/go-gh/v2/pkg/api"

type RateLimit struct {
	Cost      int    `graphql:"cost" json:"cost"`
	Limit     int    `graphql:"limit" json:"limit"`
	Remaining int    `graphql:"remaining" json:"remaining"`
	ResetAt   string `graphql:"resetAt" json:"resetAt"`
}

type PageInfo struct {
	HasNextPage     bool   `graphql:"hasNextPage" json:"hasNextPage"`
	HasPreviousPage bool   `graphql:"hasPreviousPage" json:"hasPreviousPage"`
	StartCursor     string `graphql:"startCursor" json:"startCursor"`
	EndCursor       string `graphql:"endCursor" json:"endCursor"`
}

type TextObject struct {
	Blob struct {
		Text string
	} `graphql:"... on Blob"`
}

type StarredRepository struct {
	Name        string `json:"name"`
	Url         string `json:"url"`
	Description string `json:"description"`

	Object TextObject `graphql:"object(expression: $expression)"`
}

type GraphQLQuery interface {
	Execute(*api.GraphQLClient, map[string]interface{}) error
}

type getStarredRepositoriesQuery struct {
	Viewer struct {
		StarredRepositories struct {
			Nodes    []StarredRepository
			PageInfo PageInfo `graphql:"pageInfo"`
		} `graphql:"starredRepositories(after: $after, first: 100)"`
	} `graphql:"viewer"`
	RateLimit RateLimit `graphql:"rateLimit"`
}

func (q *getStarredRepositoriesQuery) Repositories() GitHubRepositories {
	repos := make([]GitHubRepository, len(q.Viewer.StarredRepositories.Nodes))
	for i, node := range q.Viewer.StarredRepositories.Nodes {
		repos[i] = GitHubRepository{
			Name:        node.Name,
			Url:         node.Url,
			Description: node.Description,
			Readme:      node.Object.Blob.Text,
		}
	}
	return repos
}

func (q *getStarredRepositoriesQuery) PageInfo() PageInfo {
	return q.Viewer.StarredRepositories.PageInfo
}

func (q *getStarredRepositoriesQuery) Execute(client *api.GraphQLClient, variables map[string]interface{}) error {
	return client.Query("GetStarredRepositoriesQuery", q, variables)
}
