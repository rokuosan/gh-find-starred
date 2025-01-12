package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func createStarredRepository(name, url, description, readme string) StarredRepository {
	return StarredRepository{
		Name:        name,
		Url:         url,
		Description: description,
		Object: TextObject{
			Blob: struct{ Text string }{
				Text: readme,
			},
		},
	}
}

func createStarredRepositoriesQuery(nodes []StarredRepository, pageInfo PageInfo) *getStarredRepositoriesQuery {
	return &getStarredRepositoriesQuery{
		Viewer: struct {
			StarredRepositories struct {
				Nodes    []StarredRepository
				PageInfo PageInfo `graphql:"pageInfo"`
			} `graphql:"starredRepositories(after: $after, first: 100)"`
		}{
			StarredRepositories: struct {
				Nodes    []StarredRepository
				PageInfo PageInfo `graphql:"pageInfo"`
			}{
				Nodes:    nodes,
				PageInfo: pageInfo,
			},
		},
	}
}

func Test_getStarredRepositoriesQuery_Repositories(t *testing.T) {
	tests := []struct {
		name  string
		query *getStarredRepositoriesQuery
		want  GitHubRepositories
	}{
		{
			name: "test",
			query: createStarredRepositoriesQuery(
				[]StarredRepository{
					createStarredRepository("1", "", "", ""),
					createStarredRepository("2", "", "", ""),
				},
				PageInfo{},
			),
			want: GitHubRepositories{
				{
					Name: "1",
				},
				{
					Name: "2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.query.Repositories(), tt.want)
		})
	}
}
