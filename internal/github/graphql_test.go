package github

import (
	"testing"

	"github.com/rokuosan/gh-find-starred/internal/domain"
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

func createStarredRepositoriesQuery(nodes []StarredRepository, pageInfo domain.PageInfo) *getStarredRepositoriesQuery {
	var q getStarredRepositoriesQuery
	q.Viewer.StarredRepositories.Nodes = nodes
	q.Viewer.StarredRepositories.PageInfo = pageInfo
	return &q
}

func TestGetStarredRepositoriesQuery_Repositories(t *testing.T) {
	q := createStarredRepositoriesQuery(
		[]StarredRepository{
			createStarredRepository("repo-1", "https://github.com/a/repo-1", "desc 1", "readme 1"),
			createStarredRepository("repo-2", "https://github.com/b/repo-2", "desc 2", "readme 2"),
		},
		domain.PageInfo{},
	)

	want := []domain.Repository{
		{
			Name:        "repo-1",
			URL:         "https://github.com/a/repo-1",
			Description: "desc 1",
			Readme:      "readme 1",
		},
		{
			Name:        "repo-2",
			URL:         "https://github.com/b/repo-2",
			Description: "desc 2",
			Readme:      "readme 2",
		},
	}

	assert.Equal(t, want, q.Repositories())
}
