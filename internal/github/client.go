package github

import (
	"context"
	"errors"
	"fmt"

	ghapi "github.com/cli/go-gh/v2/pkg/api"
	gql "github.com/cli/shurcooL-graphql"
	"github.com/rokuosan/gh-find-starred/internal/domain"
)

// GraphQLQuerier abstracts GraphQL query execution for testability.
type GraphQLQuerier interface {
	QueryWithContext(ctx context.Context, name string, q interface{}, variables map[string]interface{}) error
}

// Client fetches starred repositories from GitHub's GraphQL API.
type Client struct {
	querier GraphQLQuerier
}

// NewDefaultClient constructs a Client backed by go-gh's default GraphQL client.
func NewDefaultClient() (*Client, error) {
	c, err := ghapi.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("create default graphql client: %w", err)
	}
	return &Client{querier: c}, nil
}

// NewClient constructs a Client using the provided GraphQLQuerier.
func NewClient(querier GraphQLQuerier) *Client {
	return &Client{querier: querier}
}

// FetchStarredRepositories retrieves all starred repositories across pages,
// respecting ctx cancellation and reporting incremental progress.
func (c *Client) FetchStarredRepositories(ctx context.Context, onProgress func(domain.FetchProgress)) ([]domain.Repository, error) {
	if c == nil || c.querier == nil {
		return nil, errors.New("github client is not initialized")
	}

	var allRepos []domain.Repository
	cursor := ""

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var q getStarredRepositoriesQuery
		variables := map[string]interface{}{
			"expression": gql.String("HEAD:README.md"),
			"after":      gql.String(cursor),
		}
		if err := c.querier.QueryWithContext(ctx, "GetStarredRepositoriesQuery", &q, variables); err != nil {
			return nil, err
		}

		pageRepos := q.Repositories()
		allRepos = append(allRepos, pageRepos...)
		if onProgress != nil {
			onProgress(domain.FetchProgress{FetchedCount: len(allRepos)})
		}

		pageInfo := q.PageInfo()
		if !pageInfo.HasNextPage {
			break
		}
		if pageInfo.EndCursor == "" || pageInfo.EndCursor == cursor {
			return nil, errors.New("pagination error: hasNextPage is true without advancing endCursor")
		}
		cursor = pageInfo.EndCursor
	}

	return allRepos, nil
}
