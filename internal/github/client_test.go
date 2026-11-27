package github

import (
	"context"
	"errors"
	"testing"

	gql "github.com/cli/shurcooL-graphql"
	"github.com/rokuosan/gh-find-starred/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubQuerier struct {
	fn func(ctx context.Context, name string, q interface{}, variables map[string]interface{}) error
}

func (s *stubQuerier) QueryWithContext(ctx context.Context, name string, q interface{}, variables map[string]interface{}) error {
	return s.fn(ctx, name, q, variables)
}

func TestClient_FetchStarredRepositories_MultiPageAndProgress(t *testing.T) {
	var cursors []string
	querier := &stubQuerier{
		fn: func(ctx context.Context, name string, q interface{}, variables map[string]interface{}) error {
			assert.Equal(t, "GetStarredRepositoriesQuery", name)
			assert.Equal(t, gql.String("HEAD:README.md"), variables["expression"])

			after, ok := variables["after"].(gql.String)
			require.True(t, ok)
			cursors = append(cursors, string(after))

			dst := q.(*getStarredRepositoriesQuery)
			switch string(after) {
			case "":
				dst.Viewer.StarredRepositories.Nodes = []StarredRepository{
					createStarredRepository("one", "https://github.com/u/one", "d1", "r1"),
					createStarredRepository("two", "https://github.com/u/two", "d2", "r2"),
				}
				dst.Viewer.StarredRepositories.PageInfo = domain.PageInfo{
					HasNextPage: true,
					EndCursor:   "cursor-1",
				}
			case "cursor-1":
				dst.Viewer.StarredRepositories.Nodes = []StarredRepository{
					createStarredRepository("three", "https://github.com/u/three", "d3", "r3"),
				}
				dst.Viewer.StarredRepositories.PageInfo = domain.PageInfo{
					HasNextPage: false,
					EndCursor:   "cursor-2",
				}
			default:
				t.Fatalf("unexpected cursor %q", after)
			}
			return nil
		},
	}

	var progress []int
	client := NewClient(querier)
	repos, err := client.FetchStarredRepositories(context.Background(), func(p domain.FetchProgress) {
		progress = append(progress, p.FetchedCount)
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"", "cursor-1"}, cursors)
	assert.Equal(t, []int{2, 3}, progress)
	require.Len(t, repos, 3)
	assert.Equal(t, "one", repos[0].Name)
	assert.Equal(t, "three", repos[2].Name)
}

func TestClient_FetchStarredRepositories_Errors(t *testing.T) {
	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		called := false
		client := NewClient(&stubQuerier{
			fn: func(ctx context.Context, name string, q interface{}, variables map[string]interface{}) error {
				called = true
				return nil
			},
		})

		_, err := client.FetchStarredRepositories(ctx, nil)
		assert.ErrorIs(t, err, context.Canceled)
		assert.False(t, called)
	})

	t.Run("query error", func(t *testing.T) {
		wantErr := errors.New("api failed")
		client := NewClient(&stubQuerier{
			fn: func(ctx context.Context, name string, q interface{}, variables map[string]interface{}) error {
				return wantErr
			},
		})

		_, err := client.FetchStarredRepositories(context.Background(), nil)
		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("empty cursor with hasNextPage", func(t *testing.T) {
		client := NewClient(&stubQuerier{
			fn: func(ctx context.Context, name string, q interface{}, variables map[string]interface{}) error {
				dst := q.(*getStarredRepositoriesQuery)
				dst.Viewer.StarredRepositories.PageInfo = domain.PageInfo{
					HasNextPage: true,
					EndCursor:   "",
				}
				return nil
			},
		})

		_, err := client.FetchStarredRepositories(context.Background(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pagination error")
	})

	t.Run("unchanged cursor with hasNextPage", func(t *testing.T) {
		calls := 0
		client := NewClient(&stubQuerier{
			fn: func(ctx context.Context, name string, q interface{}, variables map[string]interface{}) error {
				calls++
				dst := q.(*getStarredRepositoriesQuery)
				dst.Viewer.StarredRepositories.PageInfo = domain.PageInfo{
					HasNextPage: true,
					EndCursor:   "same-cursor",
				}
				return nil
			},
		})

		_, err := client.FetchStarredRepositories(context.Background(), nil)
		require.Error(t, err)
		assert.Equal(t, 2, calls)
		assert.Contains(t, err.Error(), "pagination error")
	})
}
