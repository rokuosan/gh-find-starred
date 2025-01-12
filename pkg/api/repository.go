package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/config"
	gql "github.com/cli/shurcooL-graphql"
)

type GitHubRepository struct {
	Name        string `json:"name"`
	Url         string `json:"url"`
	Description string `json:"description"`
	Readme      string `json:"readme"`
}

type GitHubRepositories []GitHubRepository

type RepositoryService interface {
	FindStarredRepositories(string) (FindStarredRepositoriesResult, error)
}

type FindStarredRepositoriesResult struct {
	Repositories []GitHubRepository
	PageInfo     PageInfo
}

type repositoryService struct{}

func NewRepositoryService() RepositoryService {
	return &repositoryService{}
}

// FindStarredRepositories は GitHub のスターをつけたリポジトリを指定されたカーソルから取得します
func (s *repositoryService) FindStarredRepositories(after string) (FindStarredRepositoriesResult, error) {
	c, err := api.DefaultGraphQLClient()
	if err != nil {
		return FindStarredRepositoriesResult{}, err
	}

	q := getStarredRepositoriesQuery{}
	if err := q.Execute(c, map[string]interface{}{
		"expression": gql.String("HEAD:README.md"),
		"after":      gql.String(after),
	}); err != nil {
		return FindStarredRepositoriesResult{}, err
	}

	return FindStarredRepositoriesResult{
		Repositories: q.Repositories(),
		PageInfo:     q.PageInfo(),
	}, nil
}

func GetStarredRepositoriesFromCache() (GitHubRepositories, error) {
	path := fmt.Sprintf("%s/starred_repositories.json", config.CacheDir())
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var cacheData struct {
		ExpiresAt string             `json:"expires_at"`
		CreatedAt string             `json:"created_at"`
		Data      GitHubRepositories `json:"data"`
	}

	err = decoder.Decode(&cacheData)
	if err != nil {
		return nil, err
	}
	expiresAt, err := time.Parse(time.RFC3339, cacheData.ExpiresAt)
	if err != nil {
		return nil, err
	}

	if time.Now().After(expiresAt) {
		return nil, errors.New("cache expired")
	}

	respositories := make(GitHubRepositories, len(cacheData.Data))
	for i, d := range cacheData.Data {
		respositories[i] = d
	}

	return respositories, nil
}
